// Command wasmbench measures the engine the browser runs, natively, over one board (agni issue 852).
// It composes internal/wasmengine over the board's folder held in memory, which is the worker's code
// path less the wasm runtime, and times the questions a viewer asks first: the read, the full
// check catalog, and one query. The browser half is web/browser/bench.mjs, which asks the same
// questions of the real worker, and `make wasm-bench` prints the two side by side.
//
//	go run ./tools/wasmbench --dir tools/samples/boards/royalblue54L-feather --design RoyalBlue54L-Feather.kicad_sch
//
// It prints one JSON object, so the make target can join it with the browser's.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/wasmengine"
)

// Query is the question the bench times. It joins every component to every net, so it scales with
// the board, which is what a measurement wants.
const Query = "component.net(?r, ?n) => ?r, ?n"

// The board's size, asked as queries. Both halves of the bench count the same way.
const (
	CountComponents = "component.net(?r, ?n) => count(distinct ?r)"
	CountNets       = "component.net(?r, ?n) => count(distinct ?n)"
)

// Result is one run's measurements. The browser half emits the same fields.
type Result struct {
	Host string `json:"host"`
	// The questions asked, so the browser half asks exactly these.
	Query           string `json:"query"`
	CountComponents string `json:"count_components"`
	CountNets       string `json:"count_nets"`
	Files           int    `json:"files"`
	Bytes           int64  `json:"bytes"`
	Components      int32  `json:"components"`
	Nets            int32  `json:"nets"`
	ComposeMS       int64  `json:"compose_ms"`
	ReadMS          int64  `json:"read_ms"`
	CheckMS         int64  `json:"check_ms"`
	Rules           int32  `json:"rules"`
	Findings        int    `json:"findings"`
	QueryMS         int64  `json:"query_ms"`
	QueryRows       int    `json:"query_rows"`
	PeakMB          int64  `json:"peak_mb"`
}

func main() {
	dir := flag.String("dir", "", "the board's folder, mounted whole")
	design := flag.String("design", "", "the design to open, relative to --dir (a file or a folder)")
	flag.Parse()
	if *dir == "" || *design == "" {
		fmt.Fprintln(os.Stderr, "usage: wasmbench --dir <folder> --design <path within it>")
		os.Exit(2)
	}
	r, err := Run(*dir, *design)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wasmbench:", err)
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(r)
}

// Run measures one board. The folder is read into memory first, as the browser holds it, so a read
// time measures parsing rather than the disk.
func Run(dir, design string) (Result, error) {
	res := Result{Host: "native", Query: Query, CountComponents: CountComponents, CountNets: CountNets}
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = b
		res.Bytes += int64(len(b))
		return nil
	})
	if err != nil {
		return res, err
	}
	res.Files = len(files)

	t := time.Now()
	eng, err := wasmengine.New(fshost.Mount{Name: "bench", FS: fshost.MemFS(files)})
	if err != nil {
		return res, err
	}
	res.ComposeMS = time.Since(t).Milliseconds()

	srv := httptest.NewServer(eng.Handler)
	defer srv.Close()
	ctx := context.Background()
	uri := "mount://bench/" + strings.TrimPrefix(filepath.ToSlash(design), "/")
	ds := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	cs := webapiconnect.NewCheckServiceClient(http.DefaultClient, srv.URL)
	qs := webapiconnect.NewQueryServiceClient(http.DefaultClient, srv.URL)

	t = time.Now()
	d, err := ds.GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Uri: uri}))
	if err != nil {
		return res, fmt.Errorf("GetDesign: %w", err)
	}
	res.ReadMS = time.Since(t).Milliseconds()
	_ = d
	// Counted by query rather than read off GetDesign, which leaves the counts empty for a file that
	// is its own drawing (see DesignService.GetDesign). Untimed.
	for _, c := range []struct {
		q   string
		out *int32
	}{{CountComponents, &res.Components}, {CountNets, &res.Nets}} {
		r, err := qs.RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Uri: uri, Query: c.q}))
		if err != nil {
			return res, fmt.Errorf("%s: %w", c.q, err)
		}
		if rows := r.Msg.GetRows(); len(rows) == 1 && len(rows[0].GetCells()) == 1 {
			n, _ := strconv.Atoi(rows[0].GetCells()[0])
			*c.out = int32(n)
		}
	}

	t = time.Now()
	c, err := cs.GetCheckReport(ctx, connect.NewRequest(&webapi.GetCheckReportRequest{Uri: uri}))
	if err != nil {
		return res, fmt.Errorf("GetCheckReport: %w", err)
	}
	res.CheckMS = time.Since(t).Milliseconds()
	res.Rules = c.Msg.GetReport().GetRulesRun()
	for _, s := range c.Msg.GetReport().GetSections() {
		res.Findings += int(s.GetCount())
	}

	t = time.Now()
	q, err := qs.RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Uri: uri, Query: Query}))
	if err != nil {
		return res, fmt.Errorf("RunQuery: %w", err)
	}
	res.QueryMS = time.Since(t).Milliseconds()
	res.QueryRows = len(q.Msg.GetRows())

	res.PeakMB = peakMB()
	return res, nil
}

// peakMB is the process's peak resident set on Linux (VmHWM), and otherwise the memory the Go
// runtime has obtained from the OS, which only grows.
func peakMB() int64 {
	if b, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if f := strings.Fields(line); len(f) >= 2 && f[0] == "VmHWM:" {
				if kb, err := strconv.ParseInt(f[1], 10, 64); err == nil {
					return kb / 1024
				}
			}
		}
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Sys >> 20)
}

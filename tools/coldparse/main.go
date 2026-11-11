// coldparse times the first board read of fresh `agni serve` processes (agni issue 1531). Each run
// starts a server the way the browser suite's childserver.ts does, ingests a design with its board,
// reads build.board from the Agni-Timing breakdown, and stops the server.
//
//	go run ./tools/coldparse -n 10 -mount jetson=examples/jetson-project -uri mount://jetson/designs/jetson
//
// -wrap runs each server under another command, such as "taskpolicy -c utility", and -attached
// keeps it in this process group. Any environment, such as GOGC or GODEBUG, passes through.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	n := flag.Int("n", 10, "how many fresh servers")
	bin := flag.String("bin", "bin/agni", "the agni binary")
	mount := flag.String("mount", "jetson=examples/jetson-project", "the --mount to serve")
	uri := flag.String("uri", "mount://jetson/designs/jetson", "the design to ingest with its board")
	wrap := flag.String("wrap", "", "a command to run each server under, such as \"taskpolicy -c utility\"")
	attached := flag.Bool("attached", false, "keep each server in this process group rather than its own")
	wait := flag.Duration("wait", 0, "how long to wait after the server answers /healthz before ingesting")
	second := flag.String("second", "", "a second mount, name=path, holding another copy of the design, ingested after the first in the same server")
	flag.Parse()

	var boards, totals []time.Duration
	for i := 0; i < *n; i++ {
		board, total, err := once(*bin, *mount, *uri, *wrap, *attached, *wait, *second)
		if err != nil {
			fmt.Fprintf(os.Stderr, "run %d: %v\n", i, err)
			os.Exit(1)
		}
		boards = append(boards, board)
		totals = append(totals, total)
		fmt.Printf("run %2d  build.board %6d ms  ingest %6d ms\n", i, board.Milliseconds(), total.Milliseconds())
	}
	fast := 0
	for _, b := range boards {
		if b < 500*time.Millisecond {
			fast++
		}
	}
	sorted := append([]time.Duration(nil), boards...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a] < sorted[b] })
	ingests := append([]time.Duration(nil), totals...)
	sort.Slice(ingests, func(a, b int) bool { return ingests[a] < ingests[b] })
	fmt.Printf("build.board: %d of %d under 500 ms, median %d ms, min %d ms, max %d ms; ingest median %d ms\n", fast, len(boards), sorted[len(sorted)/2].Milliseconds(), sorted[0].Milliseconds(), sorted[len(sorted)-1].Milliseconds(), ingests[len(ingests)/2].Milliseconds())
}

func once(bin, mount, uri, wrap string, attached bool, wait time.Duration, second string) (time.Duration, time.Duration, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	args := []string{bin, "serve", "--addr", fmt.Sprintf("127.0.0.1:%d", port), "--mount", mount, "--web-dir", "web"}
	if second != "" {
		args = append(args, "--mount", second)
	}
	if wrap != "" {
		args = append(strings.Fields(wrap), args...)
	}
	cmd := exec.Command(args[0], args[1:]...) // #nosec G204 -- runs the agni binary, under a wrapper, that the person running this tool names in its flags
	cmd.Env = os.Environ()
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if !attached {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if err := cmd.Start(); err != nil {
		return 0, 0, err
	}
	defer func() {
		if attached {
			_ = cmd.Process.Kill()
		} else {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		_ = cmd.Wait()
	}()
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	for deadline := time.Now().Add(30 * time.Second); ; {
		if r, err := http.Get(base + "/healthz"); err == nil {
			r.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			return 0, 0, fmt.Errorf("the server did not come up")
		}
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(wait)
	board, total, err := ingest(base, uri)
	if err != nil || second == "" {
		return board, total, err
	}
	name, _, _ := strings.Cut(second, "=")
	again, _, err := ingest(base, "mount://"+name+strings.TrimPrefix(uri, "mount://"+strings.SplitN(strings.TrimPrefix(uri, "mount://"), "/", 2)[0]))
	if err != nil {
		return 0, 0, err
	}
	fmt.Printf("        second copy in the same server: build.board %d ms\n", again.Milliseconds())
	return board, total, nil
}

// ingest reads uri with its board on the server at base, and returns build.board and the request's
// total from its Agni-Timing breakdown.
func ingest(base, uri string) (time.Duration, time.Duration, error) {
	req := connect.NewRequest(&webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: uri}, ReadBoard: true})
	req.Header().Set("Agni-Timing", "1")
	resp, err := webapiconnect.NewSnapshotServiceClient(http.DefaultClient, base).Ingest(context.Background(), req)
	if err != nil {
		return 0, 0, err
	}
	var rt webapi.RequestTiming
	if err := protojson.Unmarshal([]byte(resp.Header().Get("Agni-Timing")), &rt); err != nil {
		return 0, 0, err
	}
	var board time.Duration
	for _, s := range rt.GetStages() {
		if s.GetName() == "build.board" {
			board = time.Duration(s.GetMicros()) * time.Microsecond
		}
	}
	return board, time.Duration(rt.GetTotalMicros()) * time.Microsecond, nil
}

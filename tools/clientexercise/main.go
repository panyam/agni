// Command clientexercise is mission #1625's exercise, `make exercise-clients` (agni issue 1628). It
// starts one `agni serve` with a state directory, as a shared server would run, then runs each
// language's client example against it. Each example prints one line of JSON summing up its answers,
// and each line must match the CLI's answers on the same design. It prints a step table, one row per
// language, and writes it to -out. A language whose example does not exist yet waits on its ticket.
//
// Each example is clients/examples/<language>/run, taking -server and -design.
// docsite/content/build/client-contract.md says what the line holds.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// language is one client the mission builds, and the ticket that builds it.
type language struct {
	name, ticket string
}

var languages = []language{
	{"go", "#1628"},
	{"ts", "#1630"},
	{"rust", "#1631"},
	{"java", "#1632"},
	{"cpp", "#1638"},
}

// summary is the line each example prints.
type summary struct {
	Version    string   `json:"version"`
	API        string   `json:"api"`
	Features   []string `json:"features"`
	Components int      `json:"components"`
	Nets       int      `json:"nets"`
	Sheets     int      `json:"sheets"`
	Findings   int      `json:"findings"`
	QueryRows  int      `json:"query_rows"`
	// The board's layers, placed parts and routed track segments, from GetBoardGeometry's pages
	// (agni issue 1634).
	BoardLayers int `json:"board_layers"`
	BoardParts  int `json:"board_parts"`
	BoardTracks int `json:"board_tracks"`
}

const resistorsOf = `component.class(?c, "resistor") => ?c`

func main() {
	agni := flag.String("agni", "bin/agni", "the agni binary to serve and to ask for the CLI's answers")
	mount := flag.String("mount", "probe=examples/cross-probe-project", "the mount the server and the CLI read, name=path")
	design := flag.String("design", "mount://probe/designs/jetson-cut", "the design every example ingests")
	out := flag.String("out", "", "a directory to write clients.md into")
	flag.Parse()
	rows, err := exercise(*agni, *mount, *design)
	if err != nil {
		fmt.Fprintln(os.Stderr, "clientexercise:", err)
		os.Exit(1)
	}
	report := "## Clients exercise, " + *design + "\n\n" + table(rows) + "\n"
	fmt.Print(report)
	if *out != "" {
		if err := os.MkdirAll(*out, 0o750); err == nil {
			_ = os.WriteFile(filepath.Join(*out, "clients.md"), []byte(report), 0o600)
		}
	}
	for _, r := range rows {
		if r.status == "fail" {
			os.Exit(1)
		}
	}
}

type row struct{ step, status, detail string }

func table(rows []row) string {
	var b strings.Builder
	b.WriteString("| Step | Status | Detail |\n| --- | --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.step, r.status, strings.ReplaceAll(r.detail, "|", "\\|"))
	}
	return b.String()
}

func exercise(agni, mount, design string) ([]row, error) {
	name, dir, ok := strings.Cut(mount, "=")
	if !ok {
		return nil, fmt.Errorf("-mount %q is not name=path", mount)
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	mount = name + "=" + root
	if agni, err = filepath.Abs(agni); err != nil {
		return nil, err
	}
	want, err := cliAnswers(agni, mount, design)
	if err != nil {
		return nil, err
	}
	server, stop, err := serve(agni, mount)
	if err != nil {
		return nil, err
	}
	defer stop()
	rows := []row{{"serve", "ok", fmt.Sprintf("%s with --store; the CLI reads %d components, %d nets, %d sheets, %d findings, %d query rows, and a board of %d layers, %d parts and %d tracks", server, want.Components, want.Nets, want.Sheets, want.Findings, want.QueryRows, want.BoardLayers, want.BoardParts, want.BoardTracks)}}
	for _, l := range languages {
		run := filepath.Join("clients", "examples", l.name, "run")
		if _, err := os.Stat(run); err != nil {
			rows = append(rows, row{l.name, "not available", "waits on " + l.ticket + ": no " + run})
			continue
		}
		got, err := runExample(run, server, design)
		if err != nil {
			rows = append(rows, row{l.name, "fail", err.Error()})
			continue
		}
		if diff := compare(got, want); diff != "" {
			rows = append(rows, row{l.name, "fail", diff})
			continue
		}
		rows = append(rows, row{l.name, "ok", fmt.Sprintf("%s, features %s, and the CLI's answers", got.Version, strings.Join(got.Features, " "))})
	}
	return rows, nil
}

// compare says how an example's line differs from what it should hold, or "".
func compare(got, want *summary) string {
	var d []string
	if got.API != "agni.v1" {
		d = append(d, fmt.Sprintf("api %q, want agni.v1", got.API))
	}
	if got.Version != want.Version {
		d = append(d, fmt.Sprintf("version %q, want the server's %q", got.Version, want.Version))
	}
	for _, f := range []string{"grpc", "snapshot_recovery", "snapshot_store"} {
		if !slices.Contains(got.Features, f) {
			d = append(d, "no feature "+f)
		}
	}
	for _, c := range []struct {
		what      string
		got, want int
	}{{"components", got.Components, want.Components}, {"nets", got.Nets, want.Nets}, {"sheets", got.Sheets, want.Sheets}, {"findings", got.Findings, want.Findings}, {"query rows", got.QueryRows, want.QueryRows},
		{"board layers", got.BoardLayers, want.BoardLayers}, {"board parts", got.BoardParts, want.BoardParts}, {"board tracks", got.BoardTracks, want.BoardTracks}} {
		if c.got != c.want {
			d = append(d, fmt.Sprintf("%d %s, the CLI reads %d", c.got, c.what, c.want))
		}
	}
	return strings.Join(d, "; ")
}

// cliAnswers is what the CLI answers on the design, the reference every example is held to.
func cliAnswers(agni, mount, design string) (*summary, error) {
	json1 := func(args ...string) (map[string]any, error) {
		cmd := exec.Command(agni, append(args, "--mount", mount, "--format", "json")...) // #nosec G204 -- the agni binary this exercise was pointed at
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("agni %s: %w", strings.Join(args, " "), err)
		}
		var m map[string]any
		if err := json.Unmarshal(out.Bytes(), &m); err != nil {
			return nil, fmt.Errorf("agni %s: %w", strings.Join(args, " "), err)
		}
		return m, nil
	}
	count := func(m map[string]any, key string) int {
		l, _ := m[key].([]any)
		return len(l)
	}
	number := func(m map[string]any, key string) int {
		n, _ := m[key].(float64)
		return int(n)
	}
	stats, err := json1("stats", design)
	if err != nil {
		return nil, err
	}
	check, err := json1("check", design)
	if err != nil {
		return nil, err
	}
	query, err := json1("query", design, resistorsOf)
	if err != nil {
		return nil, err
	}
	board, err := json1("board", design)
	if err != nil {
		return nil, err
	}
	tracks := 0
	for _, n := range board["nets"].([]any) {
		segs, _ := n.(map[string]any)["segments"].([]any)
		tracks += len(segs)
	}
	v, err := exec.Command(agni, "version").Output() // #nosec G204 -- as above
	if err != nil {
		return nil, err
	}
	version := strings.TrimPrefix(strings.SplitN(string(v), "\n", 2)[0], "agni ")
	return &summary{Version: version, Components: number(stats, "componentCount"), Nets: number(stats, "netCount"), Sheets: count(stats, "sheets"), Findings: count(check, "findings"), QueryRows: count(query, "rows"),
		BoardLayers: count(board, "layers"), BoardParts: count(board, "placements"), BoardTracks: tracks}, nil
}

// serve starts agni serve on a free port with a fresh state directory, from an empty working
// directory so it serves the API alone, and returns its URL and a function that stops it.
func serve(agni, mount string) (string, func(), error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	addr := l.Addr().String()
	l.Close()
	work, err := os.MkdirTemp("", "agni-clients-")
	if err != nil {
		return "", nil, err
	}
	cmd := exec.Command(agni, "serve", "--addr", addr, "--mount", mount, "--store", "file://"+filepath.ToSlash(filepath.Join(work, "store"))) // #nosec G204 -- as above
	cmd.Dir = work
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}
	stop := func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = os.RemoveAll(work)
	}
	url := "http://" + addr
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for {
		if r, err := http.Get(url + "/healthz"); err == nil { // #nosec G107 -- the server this exercise just started
			r.Body.Close()
			return url, stop, nil
		}
		select {
		case <-ctx.Done():
			stop()
			return "", nil, fmt.Errorf("agni serve did not answer at %s: %s", url, log.String())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// runExample runs one language's example and reads the last line it printed.
func runExample(run, server, design string) (*summary, error) {
	cmd := exec.Command("./"+run, "-server", server, "-design", design) // #nosec G204 -- an example committed in this repo
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %v: %s", run, err, strings.TrimSpace(errOut.String()))
	}
	var last string
	sc := bufio.NewScanner(&out)
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			last = t
		}
	}
	s := &summary{}
	if err := json.Unmarshal([]byte(last), s); err != nil {
		return nil, fmt.Errorf("%s printed %q, not a summary line: %v", run, last, err)
	}
	return s, nil
}

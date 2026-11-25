package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/viewerrooms"
	"github.com/panyam/mcpkit/client"
	"github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/experimental/ext/rooms"
	"github.com/panyam/mcpkit/experimental/ext/rooms/massrelay"
)

const design = "kicad/hier_numeric_root.kicad_sch"

// serve starts `agni serve --mcp --viewer-rooms` over agni's KiCad test boards and returns its URL.
// AGNI_BIN names the agni to run, else ../../bin/agni, which `make agni` writes.
func serve(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("AGNI_BIN")
	if bin == "" {
		bin = filepath.Join("..", "..", "bin", "agni")
	}
	bin, _ = filepath.Abs(bin)
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("no agni at %s; run make agni at the repo root, or set AGNI_BIN", bin)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	cmd := exec.Command(bin, "serve", "--addr", addr, "--mount", "kicad=readers/kicad/testdata", "--web-dir", stubWeb(t, root), "--mcp", "--viewer-rooms")
	cmd.Dir = root
	var log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	base := "http://" + addr
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		if res, err := http.Get(base + "/healthz"); err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return base
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("agni serve did not come up: %s", log.String())
		}
	}
}

// stubWeb is a web directory that serve accepts without a built bundle. The test drives no browser,
// so the bundle's files can be empty.
func stubWeb(t *testing.T, root string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "web", "templates"), filepath.Join(dir, "templates")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "static"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"app.js", "load.js", "app.css", "browse.js", "agni.wasm", "wasm_exec.js", "agni-worker.js"} {
		if err := os.WriteFile(filepath.Join(dir, "static", f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// tab is a viewer tab as the bridge sees it: it joins the design's room, lets agents in, offers the
// page tools and declares the page events. It records the calls it gets, answering each with an id
// the way the viewer does.
type tab struct {
	page  *rooms.Page
	mu    sync.Mutex
	calls []string
}

func (tb *tab) record(s string) int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.calls = append(tb.calls, s)
	return len(tb.calls)
}

func (tb *tab) seen() []string {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return append([]string(nil), tb.calls...)
}

func openTab(t *testing.T, srv string) *tab {
	t.Helper()
	joined, err := webapiconnect.NewViewerRoomServiceClient(http.DefaultClient, srv).
		JoinViewerRoom(t.Context(), connect.NewRequest(&webapi.JoinViewerRoomRequest{Design: design}))
	if err != nil {
		t.Fatalf("joining the viewer room: %v", err)
	}
	// The relay checks the Origin a browser sends on its own.
	upstream, _ := url.Parse(srv)
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	browser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Origin", srv)
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(browser.Close)
	page, err := rooms.JoinPage(t.Context(), massrelay.Dial(browser.URL+strings.TrimSuffix(joined.Msg.GetRelayUrl(), "/"), joined.Msg.GetToken()), joined.Msg.GetRoom())
	if err != nil {
		t.Fatalf("the tab joins its room: %v", err)
	}
	t.Cleanup(page.Leave)
	tb := &tab{page: page}
	for _, d := range viewerrooms.ToolDecls() {
		page.RegisterTool(rooms.PageTool{Name: d.Name, Description: d.Description, InputSchema: d.InputSchema, ReadOnly: d.ReadOnly,
			Execute: func(_ context.Context, c rooms.PageCall) (core.ToolResult, error) {
				var args map[string]any
				_ = json.Unmarshal(c.Arguments, &args)
				delete(args, "design")
				b, _ := json.Marshal(args)
				n := tb.record(d.Name + " " + string(b))
				switch d.Name {
				case "viewer_show_answer":
					return core.StructuredResult("shown", map[string]any{"answer": fmt.Sprintf("answer-%d", n)}), nil
				case "viewer_propose":
					return core.StructuredResult("proposed", map[string]any{"proposal": fmt.Sprintf("proposal-%d", n)}), nil
				}
				return core.StructuredResult("done", map[string]any{}), nil
			}})
	}
	for _, d := range viewerrooms.EventDecls() {
		page.DeclareEvent(d)
	}
	page.SetAgentsAllowed(true)
	return tb
}

// TestTheAgentAnswersInTheTabAndActsOnTheReadersChoice runs one round of the demo: the reader asks,
// the agent shows the design's parts and proposes highlighting R1, the reader accepts, and the agent
// highlights R1.
func TestTheAgentAnswersInTheTabAndActsOnTheReadersChoice(t *testing.T) {
	srv := serve(t)
	tb := openTab(t, srv)
	c := client.NewClient(srv+"/mcp", core.Implementation{Name: "viewer-agent-test", Version: "0"})
	if err := c.Connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	var out syncBuffer
	a := NewAgent(c, design, &out)
	if err := a.Listen(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx, true) }()

	if err := tb.page.Ask("which parts are on this sheet?"); err != nil {
		t.Fatal(err)
	}
	var proposal string
	for deadline := time.Now().Add(20 * time.Second); proposal == ""; time.Sleep(50 * time.Millisecond) {
		for _, call := range tb.seen() {
			if strings.HasPrefix(call, "viewer_propose ") {
				proposal = fmt.Sprintf("proposal-%d", len(tb.seen()))
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the agent proposed nothing; the tab saw %q and the agent said %q", tb.seen(), out.String())
		}
	}
	if err := tb.page.Emit(viewerrooms.EventProposalAnswered, map[string]any{"proposal": proposal, "choice": "accepted", "action": "Highlight R1 on the canvas"}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("the agent: %v; it said %q", err, out.String())
	}

	want := []string{
		`viewer_show_answer {"entities":[{"kind":"component","subject":"R1"},{"kind":"component","subject":"R2"},{"kind":"component","subject":"R3"},{"kind":"component","subject":"R4"}],"text":"You asked: which parts are on this sheet?\nkicad/hier_numeric_root.kicad_sch has 4 parts."}`,
		`viewer_propose {"action":"Highlight R1 on the canvas","entities":[{"kind":"component","subject":"R1"}]}`,
		`viewer_locate_entity {"kind":"component","subject":"R1"}`,
	}
	got := tb.seen()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the tab saw\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !strings.Contains(out.String(), proposal+" accepted: highlighted R1") {
		t.Errorf("the agent said %q", out.String())
	}
}

// syncBuffer is a bytes.Buffer the agent writes from its goroutine while the test reads it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

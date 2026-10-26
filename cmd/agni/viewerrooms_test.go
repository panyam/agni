package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/mcpserver"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
	"github.com/panyam/mcpkit/client"
	"github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/experimental/ext/rooms"
	"github.com/panyam/mcpkit/experimental/ext/rooms/massrelay"
	"github.com/panyam/mcpkit/experimental/integrations/roomsevents/channel"
)

// viewerRoomsServer serves the tutorial project as `serve --mcp --viewer-rooms` does: the Connect
// API, the relay, ViewerRoomService and /mcp with the bridge's tools and room events on it.
func viewerRoomsServer(t *testing.T) *httptest.Server {
	t.Helper()
	freshWorkspace(t)
	root, err := filepath.Abs(parityMounts[0].dir)
	if err != nil {
		t.Fatal(err)
	}
	ms := []mounts.Mount{{Name: parityMounts[0].name, Root: root}}
	api := servedAPIFor(t, ms, ms)
	resolver := &service.ProjectResolver{Store: projects.NewFSStore(projectTrees(ms)...), Config: &osProjectConfig{mounts: ms}}
	mux := http.NewServeMux()
	api.Register(mux)
	// A tab is a page, so its relay connection carries the page's Origin; a Go dialer sends none,
	// which the relay refuses.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == "" && strings.HasPrefix(r.URL.Path, relayPath) {
			r.Header.Set("Origin", "http://"+r.Host)
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	m, err := mcpserver.New(api, mcpserver.Options{URLBase: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	vr, err := serveViewerRooms(mux, m, resolver, []string{srv.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vr.Close)
	mux.Handle("/mcp", mcpserver.HTTPHandler(m, "/mcp"))
	return srv
}

// TestViewerRoomsServesTheViewerToolsBesideAgnis composes what `serve --mcp --viewer-rooms` adds over
// the tutorial project (agni issue 1154): the bridge's tools join agni's own on /mcp without
// displacing any, and a tab opened on a design's folder is told the room of its declared entry, which
// is the room an agent naming the design reaches.
func TestViewerRoomsServesTheViewerToolsBesideAgnis(t *testing.T) {
	srv := viewerRoomsServer(t)

	for _, wire := range []string{"session", "stateless"} {
		t.Run(wire, func(t *testing.T) {
			var opts []client.ClientOption
			if wire == "stateless" {
				opts = append(opts, client.WithClientMode(client.ClientModeStateless))
			}
			c := client.NewClient(srv.URL+"/mcp", core.Implementation{Name: "agni-test", Version: "0"}, opts...)
			if err := c.Connect(context.Background()); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = c.Close() })
			got := toolNames(t, c)
			want := slices.Concat(mcpTools, []string{"attach", "detach", "list_pages", "next_request", "pair",
				"viewer_apply_location", "viewer_get_location", "viewer_locate_entity", "viewer_locate_verdict", "viewer_select_finding", "viewer_show_sheet"})
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("tools %v, want %v", got, want)
			}
		})
	}

	rooms := webapiconnect.NewViewerRoomServiceClient(http.DefaultClient, srv.URL)
	for _, design := range []string{mcpGateway, mcpGateway + "/gateway.edn"} {
		resp, err := rooms.JoinViewerRoom(context.Background(), connect.NewRequest(&webapi.JoinViewerRoomRequest{Design: design}))
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Msg.GetRoom(); got != "design:"+mcpGateway+"/gateway.edn" {
			t.Errorf("%s joins room %q", design, got)
		}
		if resp.Msg.GetToken() == "" || !strings.HasPrefix(resp.Msg.GetRelayUrl(), "/relay") {
			t.Errorf("%s: %v", design, resp.Msg)
		}
	}
}

func TestViewerRoomsNeedsMCP(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"serve", "--viewer-rooms", "--addr", "127.0.0.1:0"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--mcp") {
		t.Fatalf("serve --viewer-rooms without --mcp: %v", err)
	}
}

// TestARoomsChannelHearsATabsQuestion runs mcpkit's rooms-channel front end against agni's /mcp as
// Claude Code starts it, over a pipe (agni issue 1173): it attaches to the room a tab joined, the
// tab's reader asks, and the pipe carries one notifications/claude/channel holding the question.
//
// It attaches with resource rather than design, because rooms-channel watches the room named in
// attach's arguments and a design names none (panyam/mcpkit#1640).
func TestARoomsChannelHearsATabsQuestion(t *testing.T) {
	srv := viewerRoomsServer(t)
	joined, err := webapiconnect.NewViewerRoomServiceClient(http.DefaultClient, srv.URL).
		JoinViewerRoom(t.Context(), connect.NewRequest(&webapi.JoinViewerRoomRequest{Design: mcpGateway}))
	if err != nil {
		t.Fatal(err)
	}
	room := joined.Msg.GetRoom()

	f, err := channel.New(t.Context(), channel.Config{BridgeURL: srv.URL + "/mcp", Logf: t.Logf})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.Close)
	h := startChannelHost(t, f)
	h.request(1, "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "claude-code", "version": "2"}})
	h.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	if res := h.request(2, "tools/call", map[string]any{"name": "attach", "arguments": map[string]any{"resource": room}}); string(res["isError"]) == "true" {
		t.Fatalf("attach to %s: %s", room, res["content"])
	}

	tab, err := rooms.JoinPage(t.Context(), massrelay.Dial(srv.URL+strings.TrimSuffix(joined.Msg.GetRelayUrl(), "/"), joined.Msg.GetToken()), room)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tab.Leave)
	tab.SetAgentsAllowed(true)
	deadline := time.Now().Add(5 * time.Second)
	for id := 3; ; id++ {
		res := h.request(id, "tools/call", map[string]any{"name": "list_pages", "arguments": map[string]any{"resource": room}})
		if strings.Contains(string(res["content"]), `\"allowed\":true`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the tab never agreed to the channel's session: %s", res["content"])
		}
		time.Sleep(50 * time.Millisecond)
	}
	// The event streams open after attach answers; give them a moment to be live.
	time.Sleep(300 * time.Millisecond)

	if err := tab.Ask("why is C1 on GND?"); err != nil {
		t.Fatal(err)
	}
	msgs := h.waitChannel(1, 5*time.Second)
	var content string
	var meta map[string]string
	_ = json.Unmarshal(msgs[0]["content"], &content)
	_ = json.Unmarshal(msgs[0]["meta"], &meta)
	if !strings.Contains(content, "why is C1 on GND?") || meta["event"] != "room.ask" || meta["resource"] != room || meta["page"] != string(tab.ID()) {
		t.Fatalf("the channel message was %q %v", content, meta)
	}
	time.Sleep(300 * time.Millisecond)
	if n := len(h.channelMessages()); n != 1 {
		t.Fatalf("one ask arrived as %d channel messages", n)
	}
}

// channelHost is Claude Code's side of a rooms-channel pipe: it sends requests and keeps the
// responses and notifications the front end writes.
type channelHost struct {
	t     *testing.T
	w     io.Writer
	mu    sync.Mutex
	resps map[int][]byte
	notes []map[string]json.RawMessage
}

func startChannelHost(t *testing.T, f *channel.FrontEnd) *channelHost {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = f.Serve(ctx, inR, outW) }()
	h := &channelHost{t: t, w: inW, resps: map[int][]byte{}}
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var m map[string]json.RawMessage
			if json.Unmarshal(sc.Bytes(), &m) != nil {
				continue
			}
			h.mu.Lock()
			if raw, ok := m["id"]; ok {
				var id int
				_ = json.Unmarshal(raw, &id)
				h.resps[id] = slices.Clone(sc.Bytes())
			} else {
				h.notes = append(h.notes, m)
			}
			h.mu.Unlock()
		}
	}()
	return h
}

func (h *channelHost) send(msg any) {
	h.t.Helper()
	b, _ := json.Marshal(msg)
	if _, err := h.w.Write(append(b, '\n')); err != nil {
		h.t.Fatal(err)
	}
}

// request sends method and returns its result, failing on an error response or none within 5s.
func (h *channelHost) request(id int, method string, params any) map[string]json.RawMessage {
	h.t.Helper()
	h.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		h.mu.Lock()
		raw := h.resps[id]
		h.mu.Unlock()
		if raw == nil {
			continue
		}
		var resp struct {
			Result map[string]json.RawMessage `json:"result"`
			Error  json.RawMessage            `json:"error"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil || resp.Error != nil {
			h.t.Fatalf("%s: %s", method, raw)
		}
		return resp.Result
	}
	h.t.Fatalf("no response to %s", method)
	return nil
}

func (h *channelHost) channelMessages() []map[string]json.RawMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []map[string]json.RawMessage
	for _, n := range h.notes {
		var method string
		_ = json.Unmarshal(n["method"], &method)
		if method == channel.NotificationMethod {
			var params map[string]json.RawMessage
			_ = json.Unmarshal(n["params"], &params)
			out = append(out, params)
		}
	}
	return out
}

func (h *channelHost) waitChannel(n int, within time.Duration) []map[string]json.RawMessage {
	h.t.Helper()
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if msgs := h.channelMessages(); len(msgs) >= n {
			return msgs
		}
	}
	h.t.Fatalf("%d channel messages arrived, want %d", len(h.channelMessages()), n)
	return nil
}

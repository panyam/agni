package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/authgate"
	"github.com/panyam/agni/internal/viewerrooms"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/mcpkit/client"
	"github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/experimental/ext/rooms"
	"github.com/panyam/mcpkit/experimental/ext/rooms/massrelay"
)

// peopleServer serves the tutorial project as `serve --mcp --viewer-rooms --auth tokens
// --local-accounts` does, with ada and bob as local accounts. It returns the server's URL.
func peopleServer(t *testing.T) string {
	t.Helper()
	old := authgate.CacheTTL
	authgate.CacheTTL = time.Millisecond
	t.Cleanup(func() { authgate.CacheTTL = old })
	t.Setenv("AGNI_CONFIG_DIR", t.TempDir())
	t.Setenv(envToken, "")
	freshWorkspace(t)
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	root, err := filepath.Abs(parityMounts[0].dir)
	if err != nil {
		t.Fatal(err)
	}
	web := fakeWebDir(t)
	if err := os.WriteFile(filepath.Join(web, "static", "browse.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := viewerOpts{
		store: testStore(t, store), uploadsDir: filepath.Join(dir, "uploads"), auth: authgate.Tokens, localAccounts: true,
		webDir: web, mcp: mcpHTTP, viewerRooms: true,
		extraMounts: []mounts.Mount{mountset.Folder(parityMounts[0].name, root)},
	}
	srv := serveViewer(t, o)
	for _, who := range []string{"ada", "bob"} {
		if out, err := runUsers(t, who+"'s long password\n", "add", who, "--store", testStoreURL(t, store), "--password-stdin"); err != nil {
			t.Fatalf("agni users add %s: %v: %s", who, err, out)
		}
	}
	return srv
}

// person is one signed-in reader: the room their tab joined, the tab on the relay, and their agent
// on /mcp.
type person struct {
	room  string
	tab   *rooms.Page
	agent *client.Client
}

// signInPerson signs who in, opens their tab on design, as the viewer does, and connects their agent.
// The tab reaches the relay through a proxy that adds what a browser sends on its own: the page's
// Origin and the session cookie.
func signInPerson(t *testing.T, srv, who, design string) person {
	t.Helper()
	tok, err := authgate.PasswordLogin(t.Context(), srv, who, who+"'s long password")
	if err != nil {
		t.Fatalf("%s signs in: %v", who, err)
	}
	upstream, _ := url.Parse(srv)
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	browser := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Origin", srv)
		r.AddCookie(&http.Cookie{Name: authgate.SessionCookie, Value: tok.Token})
		proxy.ServeHTTP(w, r)
	}))
	t.Cleanup(browser.Close)

	rc := webapiconnect.NewViewerRoomServiceClient(http.DefaultClient, srv)
	req := connect.NewRequest(&webapi.JoinViewerRoomRequest{Design: design})
	req.Header().Set("Authorization", "Bearer "+tok.Token)
	joined, err := rc.JoinViewerRoom(t.Context(), req)
	if err != nil {
		t.Fatalf("%s joins the viewer room: %v", who, err)
	}
	tab, err := rooms.JoinPage(t.Context(), massrelay.Dial(browser.URL+strings.TrimSuffix(joined.Msg.GetRelayUrl(), "/"), joined.Msg.GetToken()), joined.Msg.GetRoom())
	if err != nil {
		t.Fatalf("%s's tab joins its room on the relay: %v", who, err)
	}
	t.Cleanup(tab.Leave)
	tab.SetAgentsAllowed(true)
	offer(tab, "viewer_get_location", who+"'s tab")

	agent := client.NewClient(srv+"/mcp", core.Implementation{Name: who + "-agent", Version: "0"}, client.WithClientBearerToken(tok.Token))
	if err := agent.Connect(context.Background()); err != nil {
		t.Fatalf("%s's agent connects: %v", who, err)
	}
	t.Cleanup(func() { _ = agent.Close() })
	return person{room: joined.Msg.GetRoom(), tab: tab, agent: agent}
}

// offer has tab offer the declared page tool name, answering with answer, so a call on it reaches the
// bridge's agreement check. A tab that offers nothing gets "no page offering" whoever calls.
func offer(tab *rooms.Page, name, answer string) {
	for _, d := range viewerrooms.ToolDecls() {
		if d.Name == name {
			tab.RegisterTool(rooms.PageTool{Name: d.Name, Description: d.Description, InputSchema: d.InputSchema, ReadOnly: d.ReadOnly,
				Execute: func(context.Context, rooms.PageCall) (core.ToolResult, error) { return core.TextResult(answer), nil }})
		}
	}
}

// agreed lists the pages in room that agreed to agent, asking until want of them have or a few
// seconds pass, since a tab's switch reaches the bridge through relay events.
func agreed(t *testing.T, agent *client.Client, room string, want int) []string {
	t.Helper()
	var ids []string
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		out, err := agent.ToolCall(t.Context(), "list_pages", map[string]any{"resource": room})
		if err != nil {
			t.Fatalf("list_pages: %v", err)
		}
		var pages []struct {
			Page    string `json:"page"`
			Allowed bool   `json:"allowed"`
		}
		if err := json.Unmarshal([]byte(out), &pages); err != nil {
			t.Fatalf("list_pages answered %q: %v", out, err)
		}
		ids = ids[:0]
		for _, p := range pages {
			if p.Allowed {
				ids = append(ids, p.Page)
			}
		}
		if len(ids) >= want || time.Now().After(deadline) {
			return ids
		}
	}
}

// Two people sign in to one server, each opens a tab on the gateway with the switch on, and each
// has an agent (agni issues 1901 and 1902). A signed-in tab joins its room on the relay, and each
// agent is agreed to by its own person's tab alone. The browser exercise drives a real tab, so it
// checks the refusal on the other person's tab (make exercise-viewer-rooms).
func TestEachSignedInPersonsAgentDrivesTheirOwnTab(t *testing.T) {
	srv := peopleServer(t)
	ada := signInPerson(t, srv, "ada", mcpGateway)
	bob := signInPerson(t, srv, "bob", mcpGateway)
	room := ada.room

	for _, c := range []struct {
		name  string
		agent *client.Client
		own   *rooms.Page
	}{{"ada", ada.agent, ada.tab}, {"bob", bob.agent, bob.tab}} {
		got := agreed(t, c.agent, room, 1)
		if len(got) != 1 || got[0] != string(c.own.ID()) {
			t.Errorf("%s's agent is agreed to by %v; want %s's tab %s alone", c.name, got, c.name, c.own.ID())
		}
	}
}

// ada's agent calls a tool on bob's tab, whose switch is on. The bridge refuses, and the refusal says
// the tab is another person's, so the agent knows to ask bob to pair rather than to turn on a switch
// (agni issue 1905). The same call on ada's own tab runs.
func TestARefusalOnAnotherPersonsTabSaysSo(t *testing.T) {
	srv := peopleServer(t)
	ada := signInPerson(t, srv, "ada", mcpGateway)
	bob := signInPerson(t, srv, "bob", mcpGateway)
	agreed(t, ada.agent, ada.room, 1)

	call := func(tab *rooms.Page) (bool, string) {
		res, err := ada.agent.ToolCallFull(t.Context(), "viewer_get_location", map[string]any{"design": mcpGateway, "page": string(tab.ID())})
		if err != nil {
			t.Fatalf("viewer_get_location: %v", err)
		}
		var b strings.Builder
		for _, c := range res.Content {
			b.WriteString(c.Text)
		}
		return !res.IsError, b.String()
	}
	if ok, out := call(ada.tab); !ok || out != "ada's tab" {
		t.Fatalf("ada's agent on ada's tab: ok=%v %q; want ada's tab to answer", ok, out)
	}
	ok, out := call(bob.tab)
	if ok {
		t.Fatalf("ada's agent ran a tool on bob's tab: %q", out)
	}
	if !strings.Contains(out, "another person's tab") {
		t.Errorf("the refusal on bob's tab does not say it is another person's: %q", out)
	}
}

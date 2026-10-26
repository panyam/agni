package viewerrooms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panyam/mcpkit/client"
	"github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/experimental/ext/events"
	eventsclient "github.com/panyam/mcpkit/experimental/ext/events/clients/go"
	"github.com/panyam/mcpkit/experimental/ext/rooms"
	"github.com/panyam/mcpkit/experimental/ext/rooms/massrelay"
	"github.com/panyam/mcpkit/experimental/integrations/roomsevents"
	mcpkit "github.com/panyam/mcpkit/server"
)

// These run agni's composition rather than the bridge's logic, which mcpkit tests: the relay under
// /relay/ admitting only tokens this process minted, each for one room, the declared viewer_* tools,
// and an agent naming a design on each call, on both MCP wires.

// roomOf stands in for service.ViewerRoomService.Room: a design's folder and its entry name one room.
func roomOf(_ context.Context, design string) (string, error) {
	return "design:" + strings.TrimSuffix(design, ".sch") + ".sch", nil
}

type stack struct {
	rooms *Rooms
	url   string
}

func newStack(t *testing.T) *stack {
	t.Helper()
	r, err := New(Options{RoomOf: roomOf, CallTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	srv := mcpkit.NewServer(core.ServerInfo{Name: "agni-test", Version: "0"})
	r.Register(srv)
	mux := http.NewServeMux()
	mux.Handle("/relay/", http.StripPrefix("/relay", r.Relay()))
	mux.Handle("/mcp", srv.Handler(mcpkit.WithStreamableHTTP(true), mcpkit.WithPrefix("/mcp")))
	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		r.Close()
		ts.Close()
	})
	return &stack{rooms: r, url: ts.URL}
}

// calls records which tab ran which tool, and with what arguments.
type calls struct {
	mu  sync.Mutex
	got []string
}

func (c *calls) add(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, s)
}

func (c *calls) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.got)
}

// tab joins design's room with a token minted for it, as a viewer page does, and offers every
// declared tool, each recording the tab's name and the sheet it was asked for.
func (st *stack) tab(t *testing.T, name, design string, rec *calls, opts ...rooms.PageOption) *rooms.Page {
	t.Helper()
	room, err := roomOf(t.Context(), design)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := st.rooms.Mint(room)
	if err != nil {
		t.Fatal(err)
	}
	p, err := rooms.JoinPage(t.Context(), massrelay.Dial(st.url+"/relay", tok), room, opts...)
	if err != nil {
		t.Fatalf("tab %s joining %s: %v", name, room, err)
	}
	t.Cleanup(p.Leave)
	for _, d := range ToolDecls() {
		p.RegisterTool(rooms.PageTool{Name: d.Name, InputSchema: d.InputSchema, ReadOnly: d.ReadOnly,
			Execute: func(_ context.Context, call rooms.PageCall) (core.ToolResult, error) {
				var args struct{ Design, Sheet string }
				_ = json.Unmarshal(call.Arguments, &args)
				rec.add(name + ":" + args.Design + ":" + args.Sheet)
				return core.TextResult("ok"), nil
			}})
	}
	return p
}

func (st *stack) agent(t *testing.T, wire string) *client.Client {
	t.Helper()
	var opts []client.ClientOption
	if wire == "stateless" {
		opts = append(opts, client.WithClientMode(client.ClientModeStateless))
	}
	c := client.NewClient(st.url+"/mcp", core.ClientInfo{Name: "agent", Version: "0"}, opts...)
	if err := c.Connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	if c.UsingStatelessWire() != (wire == "stateless") {
		t.Fatalf("the agent is not on the %s wire", wire)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func call(t *testing.T, c *client.Client, tool string, args map[string]any) (bool, string) {
	t.Helper()
	res, err := c.ToolCallFull(t.Context(), tool, args)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, ct := range res.Content {
		b.WriteString(ct.Text)
	}
	return !res.IsError, b.String()
}

func eachWire(t *testing.T, fn func(t *testing.T, wire string)) {
	for _, w := range []string{"session", "stateless"} {
		t.Run(w, func(t *testing.T) { fn(t, w) })
	}
}

// TestAnAgentDrivesTheTabOfTheDesignItNames is the case #1154 is for: six tabs over three designs,
// and an agent switching between designs call by call, never attaching.
func TestAnAgentDrivesTheTabOfTheDesignItNames(t *testing.T) {
	eachWire(t, func(t *testing.T, wire string) {
		st := newStack(t)
		rec := &calls{}
		tabs := map[string]*rooms.Page{}
		for _, tb := range []struct{ name, design string }{
			{"A1", "m/a"}, {"A2", "m/a.sch"}, {"B1", "m/b"}, {"B2", "m/b.sch"}, {"C1", "m/c"}, {"C2", "m/c"},
		} {
			p := st.tab(t, tb.name, tb.design, rec)
			if tb.name[0] != 'A' {
				p.SetAgentsAllowed(true)
			}
			tabs[tb.name] = p
		}
		tabs["C1"].Focus()
		a := st.agent(t, wire)

		ok, out := call(t, a, "viewer_show_sheet", map[string]any{"design": "m/b", "page": string(tabs["B2"].ID()), "sheet": "power"})
		if !ok {
			t.Fatalf("calling B2: %s", out)
		}
		ok, out = call(t, a, "viewer_show_sheet", map[string]any{"design": "m/c.sch", "sheet": "io"})
		if !ok {
			t.Fatalf("calling design C: %s", out)
		}
		if got, want := rec.all(), []string{"B2:m/b:power", "C1:m/c.sch:io"}; !slices.Equal(got, want) {
			t.Fatalf("tabs ran %v, want %v", got, want)
		}

		// No tab on design A turned its switch on, so the call reaches none of them and says how a
		// reader lets an agent in.
		ok, out = call(t, a, "viewer_show_sheet", map[string]any{"design": "m/a", "sheet": "x"})
		if ok || len(rec.all()) != 2 {
			t.Fatalf("a call to design A ran (%v): %s", rec.all(), out)
		}
		if !strings.Contains(out, "agent access") {
			t.Errorf("the refusal does not say how to agree: %s", out)
		}
	})
}

func TestToolsListIsTheDeclaredViewerTools(t *testing.T) {
	eachWire(t, func(t *testing.T, wire string) {
		st := newStack(t)
		tools, err := st.agent(t, wire).ListTools(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		var viewer []string
		for _, tl := range tools {
			if strings.HasPrefix(tl.Name, toolPrefix) {
				viewer = append(viewer, tl.Name)
			}
		}
		slices.Sort(viewer)
		var want []string
		for _, d := range ToolDecls() {
			want = append(want, d.Name)
		}
		slices.Sort(want)
		if !slices.Equal(viewer, want) {
			t.Fatalf("tools/list has %v, want %v", viewer, want)
		}
	})
}

func TestACallWithNoDesignSaysToNameOne(t *testing.T) {
	st := newStack(t)
	ok, out := call(t, st.agent(t, "stateless"), "viewer_get_location", map[string]any{})
	if ok || !strings.Contains(out, "design") {
		t.Fatalf("a call naming no design answered %v: %s", ok, out)
	}
}

// TestATokenAdmitsItsOwnRoomAlone: a tab's token is for its design, so a page cannot join another
// design's room with it, and a token another process signed is refused.
func TestATokenAdmitsItsOwnRoomAlone(t *testing.T) {
	st := newStack(t)
	tok, _, err := st.rooms.Mint("design:m/a.sch")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rooms.JoinPage(t.Context(), massrelay.Dial(st.url+"/relay", tok), "design:m/b.sch"); err == nil {
		t.Fatal("a token for design A joined design B's room")
	}
	other, err := New(Options{RoomOf: roomOf})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	forged, _, _ := other.Mint("design:m/a.sch")
	if _, err := rooms.JoinPage(t.Context(), massrelay.Dial(st.url+"/relay", forged), "design:m/a.sch"); err == nil {
		t.Fatal("a token signed by another process joined")
	}
	if _, err := rooms.JoinPage(t.Context(), massrelay.Dial(st.url+"/relay", ""), "design:m/a.sch"); err == nil {
		t.Fatal("a page with no token joined")
	}
	p, err := rooms.JoinPage(t.Context(), massrelay.Dial(st.url+"/relay", tok), "design:m/a.sch")
	if err != nil {
		t.Fatalf("the positive control: a token joining its own room: %v", err)
	}
	p.Leave()
}

// TestATabTrustingTheBridgesAnswersNoOtherPeer (agni issue 1173): a tab that answers only the
// subjects Bridges names still takes the agent's calls through the bridge, and a peer holding a
// genuine token for the same room gets no answer when it calls the tab directly.
func TestATabTrustingTheBridgesAnswersNoOtherPeer(t *testing.T) {
	st := newStack(t)
	rec := &calls{}
	tab := st.tab(t, "A1", "m/a", rec, rooms.TrustBridges(st.rooms.Bridges()...))
	tab.SetAgentsAllowed(true)

	if ok, out := call(t, st.agent(t, "stateless"), "viewer_show_sheet", map[string]any{"design": "m/a", "sheet": "power"}); !ok {
		t.Fatalf("the bridge's call to a tab trusting Bridges() failed: %s", out)
	}
	if got := rec.all(); !slices.Equal(got, []string{"A1:m/a:power"}) {
		t.Fatalf("tab ran %v", got)
	}

	tok, _, err := st.rooms.Mint("design:m/a.sch")
	if err != nil {
		t.Fatal(err)
	}
	intruder, err := massrelay.Dial(st.url+"/relay", tok).Join(t.Context(), "design:m/a.sch")
	if err != nil {
		t.Fatal(err)
	}
	defer intruder.Leave()
	msg := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"viewer_show_sheet","arguments":{"design":"m/a","sheet":"forged"}}}`
	if err := intruder.Send(t.Context(), tab.ID(), rooms.MessageKind, json.RawMessage(msg)); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-intruder.Events():
			var reply struct {
				ID *int `json:"id"`
			}
			if ev.Type == rooms.MessageReceived && json.Unmarshal(ev.Payload, &reply) == nil && reply.ID != nil && *reply.ID == 7 {
				t.Fatalf("the tab answered a peer that is not a bridge: %s", ev.Payload)
			}
		case <-deadline:
			if got := rec.all(); len(got) != 1 {
				t.Fatalf("the intruder's call ran: %v", got)
			}
			return
		}
	}
}

// TestAnAgentHearsAReadersQuestionByNamingTheDesign is the Ask box's receive side (agni issue 1173):
// an agent calls next_request naming the design as every other tool does, and hears a question from
// a tab that agreed to it, never from one whose switch is off.
func TestAnAgentHearsAReadersQuestionByNamingTheDesign(t *testing.T) {
	eachWire(t, func(t *testing.T, wire string) {
		st := newStack(t)
		rec := &calls{}
		off := st.tab(t, "A1", "m/a", rec)
		on := st.tab(t, "A2", "m/a.sch", rec)
		on.SetAgentsAllowed(true)
		a := st.agent(t, wire)

		// list_pages takes the design too, and lists both tabs with only A2 agreed.
		ok, out := call(t, a, "list_pages", map[string]any{"design": "m/a"})
		if !ok {
			t.Fatalf("list_pages by design: %s", out)
		}
		var pages []struct {
			Page    string `json:"page"`
			Allowed bool   `json:"allowed"`
		}
		if err := json.Unmarshal([]byte(out), &pages); err != nil || len(pages) != 2 {
			t.Fatalf("list_pages answered %s (%v)", out, err)
		}
		for _, p := range pages {
			if p.Allowed != (p.Page == string(on.ID())) {
				t.Errorf("page %s allowed=%v", p.Page, p.Allowed)
			}
		}

		heard := func(ask func()) string {
			t.Helper()
			got := make(chan string, 1)
			go func() {
				res, err := a.ToolCallFull(t.Context(), "next_request", map[string]any{"design": "m/a.sch", "wait_seconds": 2})
				if err != nil {
					got <- "error: " + err.Error()
					return
				}
				b, _ := json.Marshal(res.StructuredContent)
				got <- string(b)
			}()
			time.Sleep(300 * time.Millisecond)
			ask()
			out := <-got
			if strings.HasPrefix(out, "error: ") {
				t.Fatalf("next_request by design: %s", out)
			}
			return out
		}
		if out := heard(func() { _ = off.Ask("from the tab that kept agents out") }); strings.Contains(out, "kept agents out") || !strings.Contains(out, `"event":null`) {
			t.Fatalf("an ask from a tab with its switch off reached the agent: %s", out)
		}
		out = heard(func() { _ = on.Ask("why is SDA flagged?") })
		if !strings.Contains(out, "why is SDA flagged?") || !strings.Contains(out, `"resource":"design:m/a.sch"`) || !strings.Contains(out, string(on.ID())) {
			t.Fatalf("the agent did not hear A2's question: %s", out)
		}
	})
}

// TestAnAgentHearsAReadersQuestionAsAnEvent is the Ask box as a turn (agni issue 1173): an agent on
// the session wire attaches by naming the design, subscribes to room.ask for the room attach
// answers, and is pushed a question from a tab that agreed to it, never one from a tab whose switch
// is off. This is what rooms-channel turns into a message in a Claude Code session.
func TestAnAgentHearsAReadersQuestionAsAnEvent(t *testing.T) {
	st := newStack(t)
	rec := &calls{}
	off := st.tab(t, "A1", "m/a", rec)
	on := st.tab(t, "A2", "m/a.sch", rec)
	on.SetAgentsAllowed(true)
	a := st.agent(t, "session")

	res, err := a.ToolCallFull(t.Context(), "attach", map[string]any{"design": "m/a"})
	if err != nil || res.IsError {
		t.Fatalf("attach by design: %v %v", err, res)
	}
	var attached struct {
		Resource string `json:"resource"`
	}
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &attached); err != nil || attached.Resource != "design:m/a.sch" {
		t.Fatalf("attach answered %s", b)
	}

	asks := make(chan events.Event, 4)
	stream, err := eventsclient.Stream(t.Context(), a, eventsclient.StreamOptions{
		EventName: roomsevents.EventAsk,
		Arguments: map[string]any{"resource": attached.Resource},
		OnEvent:   func(ev events.Event) { asks <- ev },
	})
	if err != nil {
		t.Fatalf("subscribing to %s in %s: %v", roomsevents.EventAsk, attached.Resource, err)
	}
	t.Cleanup(stream.Stop)

	_ = off.Ask("from the tab that kept agents out")
	_ = on.Ask("why is SDA flagged?")
	select {
	case ev := <-asks:
		var got roomsevents.AskPayload
		if err := json.Unmarshal(ev.Data, &got); err != nil {
			t.Fatalf("the event's data %s: %v", ev.Data, err)
		}
		if got.Text != "why is SDA flagged?" || got.Page != string(on.ID()) || got.User != LocalSubject || got.Resource != attached.Resource {
			t.Fatalf("the agent heard %+v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no room.ask event arrived for A2's question")
	}
	select {
	case ev := <-asks:
		t.Fatalf("a second event arrived, from the tab with its switch off: %s", ev.Data)
	case <-time.After(300 * time.Millisecond):
	}
}

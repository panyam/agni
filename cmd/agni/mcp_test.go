package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	viewerpb "github.com/panyam/agni/gen/go/agni/v1/viewer"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mcpserver"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/mcpkit/client"
	"github.com/panyam/mcpkit/core"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

const mcpGateway = "tut/designs/gateway"

// mcpServed is `agni serve --mcp` over the tutorial project: the Connect API and the MCP endpoint on
// one server, sharing its services, with a client for each.
type mcpServed struct {
	url string
	mcp *client.Client
	api parityClients
}

func serveMCP(t *testing.T, o mcpserver.Options) mcpServed {
	t.Helper()
	freshWorkspace(t)
	root, err := filepath.Abs(parityMounts[0].dir)
	if err != nil {
		t.Fatal(err)
	}
	ms := []mounts.Mount{{Name: parityMounts[0].name, Root: root}}
	api := servedAPIFor(t, ms, ms)
	mux := http.NewServeMux()
	api.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	url := srv.URL
	// The server links to itself, as `serve --mcp` with the viewer does, unless the test says why not.
	if o.NoLinks == "" {
		o.URLBase = url
	}
	m, err := mcpserver.New(api, o)
	if err != nil {
		t.Fatal(err)
	}
	mux.Handle("/mcp", mcpserver.HTTPHandler(m, "/mcp"))
	c := client.NewClient(url+"/mcp", core.Implementation{Name: "agni-test", Version: "0"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return mcpServed{url: url, mcp: c, api: newParityClients(t, mux)}
}

// mcpAnswer is a tool result's structured content, as every agni tool and query answer carries it.
type mcpAnswer struct {
	Link   string            `json:"link"`
	NoLink string            `json:"no_link"`
	Tables []json.RawMessage `json:"tables"`
	Answer json.RawMessage   `json:"answer"`
	Links  json.RawMessage   `json:"links"`
	Page   json.RawMessage   `json:"page"`
}

// pageCall is an answer's page action (agni issue 1155): the ViewerPageService method and the
// request a host embedding the viewer sends it instead of opening the link.
type pageCall struct {
	Method string          `json:"method"`
	Body   json.RawMessage `json:"body"`
}

// appliesLink fails unless raw is ApplyLocation carrying exactly link, so the action and the link an
// answer carries move a viewer to one place.
func appliesLink(t *testing.T, name string, raw json.RawMessage, link string) {
	t.Helper()
	var p pageCall
	req := &viewerpb.ApplyLocationRequest{}
	if err := json.Unmarshal(raw, &p); err != nil || p.Method != "ApplyLocation" || protojson.Unmarshal(p.Body, req) != nil {
		t.Errorf("%s: page action %s, want ApplyLocation", name, raw)
		return
	}
	if link == "" || req.GetUrl() != link || req.GetLocation() != nil {
		t.Errorf("%s: page action applies %q, want the answer's link %q alone", name, req.GetUrl(), link)
	}
}

func callMCP(t *testing.T, c *client.Client, tool string, args map[string]any) (*core.ToolResult, mcpAnswer) {
	t.Helper()
	res, err := c.ToolCallFull(context.Background(), tool, args)
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if res.IsError {
		t.Fatalf("%s %v: tool error: %s", tool, args, res.Content[0].Text)
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var a mcpAnswer
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("%s: structured content %s: %v", tool, data, err)
	}
	return res, a
}

// tablesOf reads the tables in structured content.
func tablesOf(t *testing.T, a mcpAnswer) []*webapi.Table {
	t.Helper()
	var out []*webapi.Table
	for _, raw := range a.Tables {
		tab := &webapi.Table{}
		if err := protojson.Unmarshal(raw, tab); err != nil {
			t.Fatal(err)
		}
		out = append(out, tab)
	}
	return out
}

func ingestConnect(t *testing.T, s mcpServed, uri string) string {
	t.Helper()
	snap, err := s.api.snap.Ingest(context.Background(), connect.NewRequest(&webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://" + uri}}))
	if err != nil {
		t.Fatal(err)
	}
	return snap.Msg.GetName()
}

func tabulateConnect(t *testing.T, s mcpServed, req *webapi.TabulateRequest) []*webapi.Table {
	t.Helper()
	resp, err := s.api.table.Tabulate(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg.GetTables()
}

// sameTables fails unless the MCP answer's tables are the Connect answer's, and holds a row count
// guard, since two empty answers agree about nothing.
func sameTables(t *testing.T, name string, got, want []*webapi.Table) {
	t.Helper()
	rows := 0
	for _, tab := range want {
		rows += len(tab.GetRows())
	}
	if rows == 0 {
		t.Fatalf("%s: the Connect answer has no rows, so agreeing with it proves nothing", name)
	}
	if len(got) != len(want) {
		t.Fatalf("%s: %d tables over MCP, %d over Connect", name, len(got), len(want))
	}
	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("%s: table %q differs\nMCP:     %v\nConnect: %v", name, want[i].GetName(), got[i], want[i])
		}
	}
}

// The tools answer as the Connect API does: each question asked both ways over one server gives the
// same tables, or the same wire message where the answer is not a table. This is #910's done-when.
func TestMCPAnswersAsConnectDoes(t *testing.T) {
	s := serveMCP(t, mcpserver.Options{})
	ctx := context.Background()
	snap := ingestConnect(t, s, mcpGateway)

	t.Run("query", func(t *testing.T) {
		const program = `net.rail(?net), not net.has_test_point(?net) => ?net`
		_, a := callMCP(t, s.mcp, "query", map[string]any{"design": mcpGateway, "program": program})
		resp, err := s.api.query.RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Snapshot: snap, Query: program}))
		if err != nil {
			t.Fatal(err)
		}
		sameTables(t, "query", tablesOf(t, a), tabulateConnect(t, s, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Query{Query: resp.Msg}}))
		if !strings.Contains(a.Link, "/designs/tut/designs/gateway/gateway.edn/view?") || !strings.Contains(a.Link, "query=") {
			t.Errorf("the link opens %q, want the design's entry with the query", a.Link)
		}
		appliesLink(t, "query", a.Page, a.Link)
	})

	// jaala's query tool records witnesses, and agni presents them as RunQuery does when asked for
	// them (agni issue 794), so an agent reads how each row was derived in the wire's own shape.
	t.Run("query witnesses", func(t *testing.T) {
		const program = `net.rail(?net), not net.has_test_point(?net) => ?net`
		_, a := callMCP(t, s.mcp, "query", map[string]any{"design": mcpGateway, "program": program})
		got := &webapi.RunQueryResponse{}
		if err := protojson.Unmarshal(a.Answer, got); err != nil {
			t.Fatal(err)
		}
		resp, err := s.api.query.RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Snapshot: snap, Query: program, Witnesses: true}))
		if err != nil {
			t.Fatal(err)
		}
		want := resp.Msg
		want.Work = 0 // what RunQuery's own evaluation cost, which the MCP answer did not run
		if len(want.GetRows()) == 0 || len(want.GetRows()[0].GetWitnesses()) != 2 {
			t.Fatalf("Connect answered %d rows without the two witnesses each, so the comparison proves nothing", len(want.GetRows()))
		}
		if !proto.Equal(got, want) {
			t.Errorf("the MCP answer differs from RunQuery with witnesses\nMCP:     %v\nConnect: %v", got, want)
		}
	})

	// The text an agent reads carries each row's derivation too, capped as jaala's own text caps it
	// (agni issue 1166), so a question over a library member answers why without the JSON.
	t.Run("query witnesses in the text", func(t *testing.T) {
		const program = `component.probed_one(?r, ?p, ?u) => ?r, ?u`
		res, _ := callMCP(t, s.mcp, "query", map[string]any{"design": mcpGateway, "program": program})
		text := res.Content[0].Text
		for _, want := range []string{
			"how each row holds:\nrow 1: C1, GND\n",
			"  component.probed_one(C1, PMIC_CORE_3V3, GND)  via component.probed_one(?r, ?probed, ?unprobed) :- ",
			"    component.two_terminal(C1, GND, PMIC_CORE_3V3)  via ",
			"      component.net_count(C1, 2)  [designs/gateway/gateway.edn]\n",
			"  … 1 more fact\n",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("the text lacks %q:\n%s", want, text)
			}
		}
		if strings.Contains(text, "not net.has_test_point(GND)") {
			t.Errorf("the text wrote row 1's ninth fact past the cap of eight:\n%s", text)
		}
	})

	t.Run("query with a binding", func(t *testing.T) {
		const program = `pin.net(?ref, ?pin, ?net) => ?ref, ?pin`
		_, a := callMCP(t, s.mcp, "query", map[string]any{"design": mcpGateway, "program": program, "bind": map[string]any{"net": "I2C_SDA"}})
		resp, err := s.api.query.RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Snapshot: snap, Query: program,
			Bindings: map[string]*webapi.QueryValue{"net": {Kind: &webapi.QueryValue_Text{Text: "I2C_SDA"}}}}))
		if err != nil {
			t.Fatal(err)
		}
		sameTables(t, "bound query", tablesOf(t, a), tabulateConnect(t, s, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Query{Query: resp.Msg}}))
		if !strings.Contains(a.Link, "bind=net%3DI2C_SDA") {
			t.Errorf("the link %q doesn't carry the binding", a.Link)
		}
	})

	t.Run("query with a list binding", func(t *testing.T) {
		const program = `pin.net(?ref, ?pin, ?net) => ?ref, ?pin, ?net`
		_, a := callMCP(t, s.mcp, "query", map[string]any{"design": mcpGateway, "program": program, "bind": map[string]any{"net": []any{"I2C_SDA", "I2C_SCL"}}})
		text := func(v string) *webapi.QueryValue { return &webapi.QueryValue{Kind: &webapi.QueryValue_Text{Text: v}} }
		resp, err := s.api.query.RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Snapshot: snap, Query: program,
			Bindings: map[string]*webapi.QueryValue{"net": {Kind: &webapi.QueryValue_List{List: &webapi.QueryValueList{Values: []*webapi.QueryValue{text("I2C_SDA"), text("I2C_SCL")}}}}}}))
		if err != nil {
			t.Fatal(err)
		}
		nets := map[string]bool{}
		for _, r := range resp.Msg.GetRows() {
			nets[r.GetCells()[2]] = true
		}
		if !nets["I2C_SDA"] || !nets["I2C_SCL"] || len(nets) != 2 {
			t.Fatalf("Connect answered nets %v, want both I2C nets and nothing else", nets)
		}
		sameTables(t, "list-bound query", tablesOf(t, a), tabulateConnect(t, s, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Query{Query: resp.Msg}}))
		if !strings.Contains(a.Link, "bind=net%3DI2C_SDA") || !strings.Contains(a.Link, "bind=net%3DI2C_SCL") {
			t.Errorf("the link %q doesn't carry both values", a.Link)
		}
	})

	t.Run("check", func(t *testing.T) {
		_, a := callMCP(t, s.mcp, "check", map[string]any{"design": mcpGateway})
		resp, err := s.api.check.CheckDesign(ctx, connect.NewRequest(&webapi.CheckDesignRequest{Snapshot: snap}))
		if err != nil {
			t.Fatal(err)
		}
		sameTables(t, "check", tablesOf(t, a), tabulateConnect(t, s, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Check{Check: resp.Msg}}))
		var links []struct {
			Verdict  string   `json:"verdict"`
			Rule     string   `json:"rule"`
			Subjects []string `json:"subjects"`
			Link     string   `json:"link"`
			Page     pageCall `json:"page"`
		}
		if err := json.Unmarshal(a.Links, &links); err != nil || len(links) == 0 || !strings.Contains(links[0].Link, "verdict=") || !strings.Contains(links[0].Link, "hash=") {
			t.Errorf("verdict links %s", a.Links)
		}
		// Each link carries the LocateVerdict a host embedding the viewer sends instead, naming the
		// verdict, rule and subjects the link itself carries (agni issue 1155).
		for _, l := range links {
			req := &viewerpb.LocateVerdictRequest{}
			if l.Page.Method != "LocateVerdict" || protojson.Unmarshal(l.Page.Body, req) != nil {
				t.Errorf("%s: page action %+v, want LocateVerdict", l.Verdict, l.Page)
				continue
			}
			if req.GetId() != l.Verdict || req.GetRule() != l.Rule || len(l.Subjects) == 0 || !slices.Equal(req.GetSubjects(), l.Subjects) {
				t.Errorf("%s: page action asks %v, want the link's verdict %q, rule %q and subjects %v", l.Verdict, req, l.Verdict, l.Rule, l.Subjects)
			}
			for _, sub := range l.Subjects {
				if !strings.Contains(l.Link, "subject="+url.QueryEscape(sub)) {
					t.Errorf("%s: subject %q is not in its link %s", l.Verdict, sub, l.Link)
				}
			}
		}
	})

	t.Run("trace", func(t *testing.T) {
		_, a := callMCP(t, s.mcp, "trace", map[string]any{"design": mcpGateway, "from": "J1.1", "to": "U1.1"})
		resp, err := s.api.design.TraceDesign(ctx, connect.NewRequest(&webapi.TraceDesignRequest{Snapshot: snap,
			From: &webapi.TraceEndpoint{RefDes: "J1", Pin: "1"}, To: &webapi.TraceEndpoint{RefDes: "U1", Pin: "1"}}))
		if err != nil {
			t.Fatal(err)
		}
		got := &webapi.TraceDesignResponse{}
		if err := protojson.Unmarshal(a.Answer, got); err != nil {
			t.Fatal(err)
		}
		if len(resp.Msg.GetTrace().GetNets()) == 0 || !proto.Equal(got, resp.Msg) {
			t.Errorf("trace over MCP %v\nover Connect %v", got, resp.Msg)
		}
		if !strings.Contains(a.Link, "trace=J1.1%2CU1.1") {
			t.Errorf("trace link %q", a.Link)
		}
		appliesLink(t, "trace", a.Page, a.Link)
	})

	t.Run("diff", func(t *testing.T) {
		const a1, b1 = mcpGateway + "/gateway.edn", mcpGateway + "/gateway-rev-b.edn"
		res, a := callMCP(t, s.mcp, "diff", map[string]any{"design": a1, "against": b1})
		resp, err := s.api.diff.DiffDesigns(ctx, connect.NewRequest(&webapi.DiffDesignsRequest{ASnapshot: ingestConnect(t, s, a1), BSnapshot: ingestConnect(t, s, b1)}))
		if err != nil {
			t.Fatal(err)
		}
		sameTables(t, "diff", tablesOf(t, a), tabulateConnect(t, s, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Diff{Diff: resp.Msg}}))
		// One link opens the earlier revision comparing against the later (agni issue 1126).
		want := s.url + "/designs/" + a1 + "/view?against=" + url.QueryEscape(b1)
		if a.Link != want || a.NoLink != "" || !strings.Contains(res.Content[0].Text, "open in the viewer: "+want+"\n") {
			t.Errorf("diff link %q (no_link %q), want %q; text:\n%s", a.Link, a.NoLink, want, res.Content[0].Text)
		}
		appliesLink(t, "diff", a.Page, a.Link)
		// Asked to pair near-renames, the link asks the viewer to pair them too.
		if _, ra := callMCP(t, s.mcp, "diff", map[string]any{"design": a1, "against": b1, "rename_approx": true}); ra.Link != want+"&rename_approx=1" {
			t.Errorf("diff link with rename_approx %q, want %q", ra.Link, want+"&rename_approx=1")
		}
	})

	t.Run("review", func(t *testing.T) {
		_, a := callMCP(t, s.mcp, "review", map[string]any{"design": mcpGateway})
		list, err := s.api.review.ListChecklists(ctx, connect.NewRequest(&webapi.ListChecklistsRequest{Snapshot: snap}))
		if err != nil || len(list.Msg.GetChecklists()) == 0 {
			t.Fatalf("checklists %v %v", list, err)
		}
		resp, err := s.api.review.CreateReview(ctx, connect.NewRequest(&webapi.CreateReviewRequest{Snapshot: snap, Manifest: list.Msg.GetChecklists()[0].GetManifest(), ValidateOnly: true}))
		if err != nil {
			t.Fatal(err)
		}
		sameTables(t, "review", tablesOf(t, a), tabulateConnect(t, s, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Review{Review: resp.Msg}}))
	})

	t.Run("design", func(t *testing.T) {
		_, a := callMCP(t, s.mcp, "design", map[string]any{"design": mcpGateway, "read_mask": []string{"design.nets"}, "nets": []string{"I2C_SDA"}})
		resp, err := s.api.design.GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Snapshot: snap, ReadMask: &fieldmaskpb.FieldMask{Paths: []string{"design.nets"}}, Nets: []string{"I2C_SDA"}}))
		if err != nil {
			t.Fatal(err)
		}
		got := &webapi.GetDesignResponse{}
		if err := protojson.Unmarshal(a.Answer, got); err != nil {
			t.Fatal(err)
		}
		if len(resp.Msg.GetDesign().GetNets()) != 1 || !proto.Equal(got, resp.Msg) {
			t.Errorf("design over MCP %v\nover Connect %v", got, resp.Msg)
		}
	})
}

// mcpTools are the tools an agent sees: jaala's query language tools, each taking a design, and
// agni's own.
var mcpTools = []string{"check", "checklists", "design", "designs", "diff", "explain", "look", "query", "query_set", "review", "schema", "trace", "values"}

func toolNames(t *testing.T, c *client.Client) []string {
	t.Helper()
	defs, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, d := range defs {
		names = append(names, d.Name)
	}
	slices.Sort(names)
	return names
}

// Every tool is offered, jaala's take a design beside their own arguments, and an argument no tool
// names is refused rather than ignored.
func TestMCPOffersEveryToolAndRefusesAStrayArgument(t *testing.T) {
	s := serveMCP(t, mcpserver.Options{})
	if got := toolNames(t, s.mcp); !slices.Equal(got, mcpTools) {
		t.Errorf("tools %v, want %v", got, mcpTools)
	}
	defs, _ := s.mcp.ListTools(context.Background())
	for _, d := range defs {
		schema, _ := json.Marshal(d.InputSchema)
		if !strings.Contains(string(schema), `"design"`) && d.Name != "designs" {
			t.Errorf("%s takes no design: %s", d.Name, schema)
		}
	}
	res, err := s.mcp.ToolCallFull(context.Background(), "check", map[string]any{"design": mcpGateway, "rule": "x"})
	if err == nil && !res.IsError {
		t.Errorf("check with a misspelled argument answered: %v", res.Content)
	}
	res, err = s.mcp.ToolCallFull(context.Background(), "query", map[string]any{"design": mcpGateway, "program": `net.rail(?n) => ?n`, "desing": "x"})
	if err == nil && !res.IsError {
		t.Errorf("query with a misspelled argument answered: %v", res.Content)
	}
}

// A snapshot the host dropped is ingested again on the next question, which then answers.
func TestMCPIngestsADroppedSnapshotAgain(t *testing.T) {
	s := serveMCP(t, mcpserver.Options{})
	ctx := context.Background()
	args := map[string]any{"design": mcpGateway, "program": `net.rail(?net) => ?net`}
	_, first := callMCP(t, s.mcp, "query", args)
	list, err := s.api.snap.ListSnapshots(ctx, connect.NewRequest(&webapi.ListSnapshotsRequest{}))
	if err != nil || len(list.Msg.GetSnapshots()) == 0 {
		t.Fatalf("the query held no snapshot: %v %v", list, err)
	}
	for _, sn := range list.Msg.GetSnapshots() {
		if _, err := s.api.snap.DeleteSnapshot(ctx, connect.NewRequest(&webapi.DeleteSnapshotRequest{Name: sn.GetName()})); err != nil {
			t.Fatal(err)
		}
	}
	_, again := callMCP(t, s.mcp, "query", args)
	if len(tablesOf(t, again)) == 0 || !proto.Equal(tablesOf(t, again)[0], tablesOf(t, first)[0]) {
		t.Errorf("after the snapshot was dropped, the answer changed or failed")
	}
}

// A server with no viewer answers without a link and says why, in the text and the structured content.
func TestMCPSaysWhyAnAnswerHasNoLink(t *testing.T) {
	s := serveMCP(t, mcpserver.Options{NoLinks: "this server serves the API without the viewer"})
	res, a := callMCP(t, s.mcp, "query", map[string]any{"design": mcpGateway, "program": `net.rail(?net) => ?net`})
	if a.Link != "" || a.NoLink == "" || !strings.Contains(res.Content[0].Text, "no viewer link: this server serves the API without the viewer") {
		t.Errorf("link %q, no_link %q, text:\n%s", a.Link, a.NoLink, res.Content[0].Text)
	}
}

// Asking about a pin the design lacks is the question's mistake, so it is a tool error, while two
// real pins with no route between them answer.
func TestMCPTraceToAPinTheDesignLacksIsAToolError(t *testing.T) {
	s := serveMCP(t, mcpserver.Options{})
	res, err := s.mcp.ToolCallFull(context.Background(), "trace", map[string]any{"design": mcpGateway, "from": "J1.1", "to": "U999.1"})
	if err != nil || !res.IsError || !strings.Contains(res.Content[0].Text, "cannot trace") {
		t.Errorf("a trace to U999.1: %v %v", res, err)
	}
	callMCP(t, s.mcp, "trace", map[string]any{"design": mcpGateway, "from": "J1.1", "to": "U1.1"})
}

// `agni mcp` serves the same tools over stdio as `serve --mcp` does over HTTP, and answers a query.
func TestAgniMCPServesTheSameToolsOverStdio(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	root, err := filepath.Abs(parityMounts[0].dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	freshWorkspace(t)
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		cmd := rootCmd()
		cmd.SetArgs([]string{"mcp", "--mount", "tut=" + root})
		cmd.SetIn(inR)
		cmd.SetOut(outW)
		cmd.SetErr(io.Discard)
		done <- cmd.ExecuteContext(ctx)
		_ = outW.Close()
	}()
	t.Cleanup(func() {
		cancel()
		_ = inW.Close()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("agni mcp did not stop when its context ended")
		}
	})
	c := client.NewClient("", core.Implementation{Name: "agni-test", Version: "0"}, client.WithIOTransport(outR, inW))
	if err := c.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if got := toolNames(t, c); !slices.Equal(got, mcpTools) {
		t.Errorf("tools over stdio %v, want %v", got, mcpTools)
	}
	res, a := callMCP(t, c, "query", map[string]any{"design": mcpGateway, "program": `net.rail(?net) => ?net`})
	if len(tablesOf(t, a)) == 0 || len(tablesOf(t, a)[0].GetRows()) == 0 {
		t.Errorf("no rows over stdio:\n%s", res.Content[0].Text)
	}
	if a.Link != "" || !strings.Contains(a.NoLink, "--server") {
		t.Errorf("without --server, link %q and no_link %q", a.Link, a.NoLink)
	}
}

// A check naming a rule that doesn't exist is the question's mistake, so the agent hears it as one with
// the rule it probably meant, rather than reading an empty run as a clean design (agni issue 1121).
func TestMCPCheckOfAnUnknownRuleIsAToolError(t *testing.T) {
	s := serveMCP(t, mcpserver.Options{})
	res, err := s.mcp.ToolCallFull(context.Background(), "check", map[string]any{"design": mcpGateway, "rules": []any{"i2c-missing-pullup"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || len(res.Content) == 0 || !strings.Contains(res.Content[0].Text, "did you mean i2c-pull-up") {
		t.Errorf("check of an unknown rule answered %+v, want a tool error offering i2c-pull-up", res)
	}
}

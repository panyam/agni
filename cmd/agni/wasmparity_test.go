package main

import (
	"context"
	"fmt"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/fshost"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/internal/wasmengine"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

// The browser engine serves designs from an in-memory copy through fshost, and `agni serve` reads the
// same files through osLoader. Two adapters behind one port can disagree with nothing erroring, which
// is how one design read two ways for most of 2026 (C32), so this asks both the same questions and
// requires identical answers. Two fixtures cover the two ways a design finds its drawing: the
// tutorial project names its companions in a descriptor and carries profiles, params and a library,
// and the review testdata holds an EDIF netlist whose schematic is an undeclared sibling .eds.

// parityMounts is the table both sides serve, relative to this package.
var parityMounts = []struct{ name, dir string }{
	{"tut", filepath.Join("..", "..", "examples", "tutorial-project")},
	{"rev", filepath.Join("testdata", "review")},
}

// servedParityMux composes the served side as `agni serve --mount ...` does. The mounts go in through
// the --mount flag's variable as well as the serve table, because the server's loader names
// provenance from that flag's table (newLoader), and without it every locator here records a
// basename that a real `serve --mount` does not.
func servedParityMux(t *testing.T, ms []mounts.Mount) http.Handler {
	t.Helper()
	return servedMuxFor(t, ms, ms)
}

// servedMuxFor is servedParityMux with this process's mounts (cli) apart from the server's own
// (ms), for a client whose files the server does not share. The server takes uploaded files, as
// `agni serve` does by default (agni issue 1061).
func servedMuxFor(t *testing.T, cli, ms []mounts.Mount) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	servedAPIFor(t, cli, ms).Register(mux)
	return mux
}

// servedAPIFor is the services servedMuxFor registers, for a test that serves them over another
// transport beside Connect, as `agni serve --mcp` does.
func servedAPIFor(t *testing.T, cli, ms []mounts.Mount) server.API {
	t.Helper()
	prevSpecs, prevVal, prevErr := cliMountSpecs, cliWSVal, cliWSErr
	cliMountSpecs = nil
	for _, m := range cli {
		cliMountSpecs = append(cliMountSpecs, m.Name+"="+m.Root)
	}
	cliWSOnce, cliWSVal, cliWSErr = sync.Once{}, nil, nil
	t.Cleanup(func() {
		cliMountSpecs, cliWSVal, cliWSErr = prevSpecs, prevVal, prevErr
		cliWSOnce = sync.Once{}
	})
	loader := &osLoader{mounts: ms, loader: newLoader()}
	resolver := &service.ProjectResolver{Store: projects.NewFSStore(projectTrees(ms)...), Config: &osProjectConfig{mounts: ms}, Uploads: uploadedHost}
	check, review, err := serveRuleServices(loader, nil, nil, "", nil, resolver, nil)
	if err != nil {
		t.Fatal(err)
	}
	return server.API{
		Workspace: service.NewWorkspaceService(osWorkspace(ms)).WithDesignFiles(resolver, osFiles(ms)),
		Project:   service.NewProjectService(resolver.Store),
		Design:    service.NewDesignService(loader, nil, render.DefaultStyle, resolver),
		Check:     check,
		Diff:      service.NewDiffService(loader, resolver),
		Query:     service.NewQueryService(loader, nil, resolver),
		Review:    review,
		Snapshot:  check.Snapshots(),
	}
}

// memCopy reads root into memory, as the browser holds a dropped folder.
func memCopy(t *testing.T, root string) fs.FS {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fshost.MemFS(files)
}

type parityClients struct {
	snap   webapiconnect.SnapshotServiceClient
	design webapiconnect.DesignServiceClient
	check  webapiconnect.CheckServiceClient
	query  webapiconnect.QueryServiceClient
	ws     webapiconnect.WorkspaceServiceClient
	table  webapiconnect.TableServiceClient
	review webapiconnect.ReviewServiceClient
	diff   webapiconnect.DiffServiceClient
	lex    webapiconnect.LexiconServiceClient
}

func newParityClients(t *testing.T, h http.Handler) parityClients {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := srv.Client()
	return parityClients{
		snap:   webapiconnect.NewSnapshotServiceClient(c, srv.URL),
		design: webapiconnect.NewDesignServiceClient(c, srv.URL),
		review: webapiconnect.NewReviewServiceClient(c, srv.URL),
		check:  webapiconnect.NewCheckServiceClient(c, srv.URL),
		query:  webapiconnect.NewQueryServiceClient(c, srv.URL),
		ws:     webapiconnect.NewWorkspaceServiceClient(c, srv.URL),
		table:  webapiconnect.NewTableServiceClient(c, srv.URL),
		diff:   webapiconnect.NewDiffServiceClient(c, srv.URL),
		lex:    webapiconnect.NewLexiconServiceClient(c, srv.URL),
	}
}

func TestWasmEngineAnswersAsTheServerDoes(t *testing.T) {
	var hostMounts []mounts.Mount
	var memMounts []fshost.Mount
	for _, m := range parityMounts {
		root, err := filepath.Abs(m.dir)
		if err != nil {
			t.Fatal(err)
		}
		hostMounts = append(hostMounts, mounts.Mount{Name: m.name, Root: root})
		memMounts = append(memMounts, fshost.Mount{Name: m.name, FS: memCopy(t, root)})
	}
	eng, err := wasmengine.New(memMounts...)
	if err != nil {
		t.Fatal(err)
	}
	served := newParityClients(t, servedParityMux(t, hostMounts))
	wasm := newParityClients(t, eng.Handler)

	cases := []struct {
		name, uri, query string
		// ruleFrom names a rule source that must appear in the report, so the project's own config
		// is on the path being compared rather than only the built-ins. Empty for a fixture whose
		// report is clean, where the drawing is what is being compared.
		ruleFrom string
	}{
		{"descriptor companions and project config", "mount://tut/designs/gateway", "net.has_test_point(?n) => ?n", "gateway-profiles/"},
		{"undeclared sibling .eds", "mount://rev/companion-demo.edn", "component.net(?r, ?n) => ?r, ?n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			askParity(t, served, wasm, c.uri, c.query, c.ruleFrom)
		})
	}
}

// askParity asks both hosts the questions the viewer asks on opening a design, and requires the same
// answers.
func askParity(t *testing.T, served, wasm parityClients, design, query, ruleFrom string) {
	t.Helper()
	ctx := context.Background()
	// Each host ingests the design into a snapshot of its own, whose name is random, so a question
	// names the snapshot of the host it is asked of.
	snaps := map[webapiconnect.SnapshotServiceClient]string{}
	for _, c := range []parityClients{served, wasm} {
		s, err := c.snap.Ingest(ctx, connect.NewRequest(&webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: design}}))
		if err != nil {
			t.Fatalf("Ingest %s: %v", design, err)
		}
		snaps[c.snap] = s.Msg.GetName()
	}
	snapOn := func(c parityClients) string { return snaps[c.snap] }
	ask := func(name string, call func(parityClients) (proto.Message, error)) proto.Message {
		t.Helper()
		a, errA := call(served)
		b, errB := call(wasm)
		// Every question here has an answer, and two hosts failing alike is not two hosts agreeing.
		if errA != nil || errB != nil {
			t.Fatalf("%s: served err %v, wasm err %v", name, errA, errB)
		}
		if !proto.Equal(a, b) {
			t.Errorf("%s: the wasm engine answers differently from the server:\n%s", name, firstDifference(a, b))
		}
		return a
	}

	got := ask("GetDesign", func(c parityClients) (proto.Message, error) {
		r, err := c.design.GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Snapshot: snapOn(c)}))
		return msgOf(r, err)
	})
	sheets := got.(*webapi.GetDesignResponse).GetSheets()
	if len(sheets) == 0 {
		t.Fatal("GetDesign: no sheets, so the sheet comparison below would compare nothing")
	}
	for _, s := range sheets {
		if s.GetId() == "graph" {
			t.Fatal("GetDesign: drew the auto-layout, so the design's own schematic was not found and both hosts agreeing on that proves nothing")
		}
	}
	for _, s := range sheets {
		ask("GetSheet "+s.GetId(), func(c parityClients) (proto.Message, error) {
			r, err := c.design.GetSheet(ctx, connect.NewRequest(&webapi.GetSheetRequest{Snapshot: snapOn(c), Sheet: s.GetId()}))
			return msgOf(r, err)
		})
	}
	// The page ingests with read_board after the first sheet draws (agni issue 1061), on either host.
	// The name is the one field that is the host's own, since each host picks it at random.
	ask("Ingest read_board", func(c parityClients) (proto.Message, error) {
		r, err := c.snap.Ingest(ctx, connect.NewRequest(&webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: design}, ReadBoard: true}))
		if err != nil {
			return nil, err
		}
		if r.Msg.GetName() == "" {
			return nil, fmt.Errorf("the snapshot has no name")
		}
		r.Msg.Name = ""
		return r.Msg, nil
	})
	report := ask("GetCheckReport", func(c parityClients) (proto.Message, error) {
		r, err := c.check.GetCheckReport(ctx, connect.NewRequest(&webapi.GetCheckReportRequest{Snapshot: snapOn(c)}))
		return msgOf(r, err)
	})
	sections := report.(*webapi.GetCheckReportResponse).GetReport().GetSections()
	// A check asked about one subject answers on both hosts alike (agni issue 897), so the viewer's
	// question about a selected net does not depend on which engine the page runs.
	whole, err := served.check.CheckDesign(ctx, connect.NewRequest(&webapi.CheckDesignRequest{Snapshot: snapOn(served)}))
	if err != nil || len(whole.Msg.GetVerdicts()) == 0 {
		t.Fatalf("CheckDesign: err %v with %d verdicts, so there is no subject to ask about", err, len(whole.Msg.GetVerdicts()))
	}
	subject := whole.Msg.GetVerdicts()[0].GetSubjects()[0]
	scoped := ask("CheckDesign scoped", func(c parityClients) (proto.Message, error) {
		r, err := c.check.CheckDesign(ctx, connect.NewRequest(&webapi.CheckDesignRequest{Snapshot: snapOn(c), Subjects: []*checkspb.Subject{subject}}))
		return msgOf(r, err)
	})
	if len(scoped.(*webapi.CheckDesignResponse).GetVerdicts()) == 0 {
		t.Fatalf("CheckDesign about %s:%s: no verdicts, so two empty answers would compare equal", subject.GetKind(), subject.GetRef())
	}
	// How the design's names were read is the same question on either host (agni issue 1266).
	lx := ask("GetLexicon", func(c parityClients) (proto.Message, error) {
		r, err := c.lex.GetLexicon(ctx, connect.NewRequest(&webapi.GetLexiconRequest{Snapshot: snapOn(c)}))
		return msgOf(r, err)
	})
	if len(lx.(*webapi.GetLexiconResponse).GetEntries()) == 0 {
		t.Fatal("GetLexicon: no entries, so two empty lexicons would compare equal")
	}
	for _, v := range whole.Msg.GetVerdicts() {
		if s := v.GetSubjects()[0]; s.GetKind() == "net" {
			ask("ExplainNet "+s.GetRef(), func(c parityClients) (proto.Message, error) {
				r, err := c.lex.ExplainNet(ctx, connect.NewRequest(&webapi.ExplainNetRequest{Snapshot: snapOn(c), Net: s.GetRef()}))
				return msgOf(r, err)
			})
			break
		}
	}
	ask("CompareLexicons", func(c parityClients) (proto.Message, error) {
		r, err := c.lex.CompareLexicons(ctx, connect.NewRequest(&webapi.CompareLexiconsRequest{Snapshot: snapOn(c),
			Conventions: &configpb.NamingConvention{Name: "parity", Lexicon: &configpb.NamingLexicon{Net: &configpb.NetNameVocab{Control: &configpb.VocabPatterns{Replace: true}}}}}))
		return msgOf(r, err)
	})
	if ruleFrom != "" {
		if len(sections) == 0 {
			t.Fatal("GetCheckReport: no findings, so two empty reports would compare equal")
		}
		found := false
		for _, sec := range sections {
			for _, g := range sec.GetRules() {
				found = found || strings.HasPrefix(g.GetRule(), ruleFrom)
			}
		}
		if !found {
			t.Errorf("GetCheckReport: no rule from %q, so the project's own config is not on the compared path", ruleFrom)
		}
	}
	// A saved report is rendered from the run the page holds (agni issue 127), so each host renders its
	// own run, and the files must be the same bytes apart from when they were written.
	for _, format := range []webapi.CheckReportFormat{webapi.CheckReportFormat_CHECK_REPORT_FORMAT_RESULTS_JSON, webapi.CheckReportFormat_CHECK_REPORT_FORMAT_HTML} {
		render := func(c parityClients) (string, error) {
			run, err := c.check.CheckDesign(ctx, connect.NewRequest(&webapi.CheckDesignRequest{Snapshot: snapOn(c)}))
			if err != nil {
				return "", err
			}
			r, err := c.check.RenderCheckReport(ctx, connect.NewRequest(&webapi.RenderCheckReportRequest{Snapshot: snapOn(c), Run: run.Msg, Format: format}))
			if err != nil {
				return "", err
			}
			return writtenAt.ReplaceAllString(string(r.Msg.GetContent()), "<time>"), nil
		}
		a, errA := render(served)
		b, errB := render(wasm)
		if errA != nil || errB != nil {
			t.Fatalf("RenderCheckReport %s: served err %v, wasm err %v", format, errA, errB)
		}
		if len(a) < 200 {
			t.Fatalf("RenderCheckReport %s: %d bytes, so two near-empty files would compare equal", format, len(a))
		}
		if a != b {
			t.Errorf("RenderCheckReport %s: the wasm engine writes a different file from the server", format)
		}
	}
	// A checklist run saved from the Review panel is rendered by whichever host the page runs on (agni
	// issue 1030), so both write one run's page identically. The run is a value, so neither reads a
	// store, which the browser engine does not have.
	rv := &webapi.Review{Results: &checkspb.CheckResults{
		Design:   &checkspb.DesignRef{Source: design, ContentHash: "abc123"},
		Manifest: "Parity",
		Areas: []*checkspb.ReviewArea{{Name: "Interfaces", Items: []*checkspb.ReviewItem{{
			Id: "I1", Title: "the CAN interface is terminated", Outcome: "fail",
			Findings: []*checkspb.Finding{{Rule: "can-term", Subject: &checkspb.Subject{Kind: "net", Ref: "CANH"}, Message: "not terminated"}},
		}}}},
	}}
	renderReview := func(c parityClients) (string, error) {
		r, err := c.review.RenderReviewReport(ctx, connect.NewRequest(&webapi.RenderReviewReportRequest{Review: rv, Links: &webapi.ReportLinks{UrlBase: "http://h", DesignUri: design}}))
		if err != nil {
			return "", err
		}
		return writtenAt.ReplaceAllString(string(r.Msg.GetContent()), "<time>"), nil
	}
	pa, errA := renderReview(served)
	pb, errB := renderReview(wasm)
	if errA != nil || errB != nil {
		t.Fatalf("RenderReviewReport: served err %v, wasm err %v", errA, errB)
	}
	if !strings.Contains(pa, "the CAN interface is terminated") {
		t.Fatal("RenderReviewReport: the page lacks its item, so two broken pages would compare equal")
	}
	if pa != pb {
		t.Error("RenderReviewReport: the wasm engine writes a different page from the server")
	}
	// A checklist is scored on the browser engine with validate_only, since it keeps no review store
	// (agni issue 931), so both hosts score one checklist alike and neither stores it.
	lists, err := served.review.ListChecklists(ctx, connect.NewRequest(&webapi.ListChecklistsRequest{Snapshot: snapOn(served)}))
	if err != nil {
		t.Fatalf("ListChecklists: %v", err)
	}
	// Only a design in a project has a checklist to score; the project case must have one, or the
	// comparison below is skipped for the case it exists for.
	if len(lists.Msg.GetChecklists()) == 0 && ruleFrom != "" {
		t.Fatal("ListChecklists: the project design offers no checklist")
	}
	if len(lists.Msg.GetChecklists()) > 0 {
		score := func(c parityClients) (*webapi.Review, error) {
			r, err := c.review.CreateReview(ctx, connect.NewRequest(&webapi.CreateReviewRequest{
				Snapshot: snapOn(c), Manifest: lists.Msg.GetChecklists()[0].GetManifest(), ValidateOnly: true,
			}))
			if err != nil {
				return nil, err
			}
			rv := r.Msg
			rv.GetResults().GetMeta().CreatedAt = ""
			return rv, nil
		}
		ra, errA := score(served)
		rb, errB := score(wasm)
		if errA != nil || errB != nil {
			t.Fatalf("CreateReview validate_only: served err %v, wasm err %v", errA, errB)
		}
		if n := len(ra.GetResults().GetAreas()); n == 0 {
			t.Fatal("CreateReview: the run scored no area, so two empty runs would compare equal")
		}
		if !proto.Equal(ra, rb) {
			t.Errorf("CreateReview validate_only: the wasm engine scores the checklist differently:\n%s", firstDifference(ra, rb))
		}
	}
	// A run saved as csv is tabulated and encoded by whichever host the page runs on (agni issue 956),
	// so both lay out and write one run identically. The run is the server's, sent to each.
	run, err := served.check.CheckDesign(ctx, connect.NewRequest(&webapi.CheckDesignRequest{Snapshot: snapOn(served)}))
	if err != nil {
		t.Fatalf("CheckDesign: %v", err)
	}
	tabulated := ask("Tabulate", func(c parityClients) (proto.Message, error) {
		r, err := c.table.Tabulate(ctx, connect.NewRequest(&webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Check{Check: run.Msg}}))
		return msgOf(r, err)
	}).(*webapi.TabulateResponse)
	if len(tabulated.GetTables()) == 0 || (ruleFrom != "" && len(tabulated.GetTables()[0].GetRows()) == 0) {
		t.Fatal("Tabulate: no rows, so two empty tables would compare equal")
	}
	ask("Encode", func(c parityClients) (proto.Message, error) {
		r, err := c.table.Encode(ctx, connect.NewRequest(&webapi.EncodeTablesRequest{Tables: tabulated.GetTables(), Encoding: webapi.TableEncoding_TABLE_ENCODING_CSV}))
		return msgOf(r, err)
	})
	ask("RunQuery", func(c parityClients) (proto.Message, error) {
		r, err := c.query.RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Snapshot: snapOn(c), Query: query}))
		return msgOf(r, err)
	})
	// The viewer asks for witnesses when a row is expanded (agni issue 794), so both hosts derive and
	// locate them alike.
	witnessed := ask("RunQuery witnesses", func(c parityClients) (proto.Message, error) {
		r, err := c.query.RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Snapshot: snapOn(c), Query: query, Witnesses: true}))
		return msgOf(r, err)
	}).(*webapi.RunQueryResponse)
	if rows := witnessed.GetRows(); len(rows) == 0 || len(rows[0].GetWitnesses()) == 0 {
		t.Fatal("RunQuery witnesses: no witnessed row, so two answers without witnesses would compare equal")
	}
	// A project's query sets are read off its queries/ directory by whichever host serves the page
	// (agni issue 745). A fixture carrying project config keeps sets, so both listing none is a failure.
	sets := ask("ListQuerySets", func(c parityClients) (proto.Message, error) {
		r, err := c.query.ListQuerySets(ctx, connect.NewRequest(&webapi.ListQuerySetsRequest{Snapshot: snapOn(c)}))
		return msgOf(r, err)
	}).(*webapi.ListQuerySetsResponse)
	if ruleFrom != "" && len(sets.GetQuerySets()) == 0 {
		t.Fatal("ListQuerySets: no sets, so two empty lists would compare equal")
	}
	ask("ListDesignFiles", func(c parityClients) (proto.Message, error) {
		r, err := c.ws.ListDesignFiles(ctx, connect.NewRequest(&webapi.ListDesignFilesRequest{Uri: design}))
		return msgOf(r, err)
	})
	dir := design[:strings.LastIndex(design, "/")]
	ask("ListDir", func(c parityClients) (proto.Message, error) {
		r, err := c.ws.ListDir(ctx, connect.NewRequest(&webapi.ListDirRequest{Uri: dir}))
		return msgOf(r, err)
	})
}

// writtenAt matches the moment a saved report records it was written, the one part two hosts writing
// the same run cannot share: a document's created_at and a verdict page's generated line.
var writtenAt = regexp.MustCompile(`"createdAt":\s*"[^"]*"|\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(Z| UTC)`)

func msgOf[T any](r *connect.Response[T], err error) (proto.Message, error) {
	if err != nil {
		return nil, err
	}
	return any(r.Msg).(proto.Message), nil
}

// firstDifference shows where two messages' text forms first part, with a little context either side.
func firstDifference(a, b proto.Message) string {
	opt := prototext.MarshalOptions{Multiline: true}
	la := strings.Split(opt.Format(a), "\n")
	lb := strings.Split(opt.Format(b), "\n")
	i := 0
	for i < len(la) && i < len(lb) && la[i] == lb[i] {
		i++
	}
	lo := max(i-3, 0)
	hiA, hiB := min(i+4, len(la)), min(i+4, len(lb))
	return fmt.Sprintf("served, from line %d:\n%s\nwasm, from line %d:\n%s", lo+1, strings.Join(la[lo:hiA], "\n"), lo+1, strings.Join(lb[lo:hiB], "\n"))
}

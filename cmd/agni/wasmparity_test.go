package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/fshost"
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
	prevSpecs, prevVal, prevErr := cliMountSpecs, cliWSVal, cliWSErr
	cliMountSpecs = nil
	for _, m := range ms {
		cliMountSpecs = append(cliMountSpecs, m.Name+"="+m.Root)
	}
	cliWSOnce, cliWSVal, cliWSErr = sync.Once{}, nil, nil
	t.Cleanup(func() {
		cliMountSpecs, cliWSVal, cliWSErr = prevSpecs, prevVal, prevErr
		cliWSOnce = sync.Once{}
	})
	loader := &osLoader{mounts: ms, loader: newLoader()}
	resolver := &service.ProjectResolver{Store: projects.NewFSStore(projectTrees(ms)...), Config: &osProjectConfig{mounts: ms}}
	check, review, err := serveRuleServices(loader, nil, nil, "", nil, resolver, nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	server.API{
		Workspace: service.NewWorkspaceService(osWorkspace(ms)).WithDesignFiles(resolver, osFiles(ms)),
		Project:   service.NewProjectService(resolver.Store),
		Design:    service.NewDesignService(loader, nil, render.DefaultStyle, resolver),
		Check:     check,
		Diff:      service.NewDiffService(loader, resolver),
		Query:     service.NewQueryService(loader, nil, resolver),
		Review:    review,
	}.Register(mux)
	return mux
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
	design webapiconnect.DesignServiceClient
	check  webapiconnect.CheckServiceClient
	query  webapiconnect.QueryServiceClient
	ws     webapiconnect.WorkspaceServiceClient
}

func newParityClients(t *testing.T, h http.Handler) parityClients {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := srv.Client()
	return parityClients{
		design: webapiconnect.NewDesignServiceClient(c, srv.URL),
		check:  webapiconnect.NewCheckServiceClient(c, srv.URL),
		query:  webapiconnect.NewQueryServiceClient(c, srv.URL),
		ws:     webapiconnect.NewWorkspaceServiceClient(c, srv.URL),
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
		r, err := c.design.GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Uri: design}))
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
		if s.GetId() == "board" {
			// GetSheet reads the board at the request URI rather than the resolved board tier, so both
			// hosts refuse the sheet GetDesign lists (agni 865). Drop this skip with that fix.
			continue
		}
		ask("GetSheet "+s.GetId(), func(c parityClients) (proto.Message, error) {
			r, err := c.design.GetSheet(ctx, connect.NewRequest(&webapi.GetSheetRequest{Uri: design, Sheet: s.GetId()}))
			return msgOf(r, err)
		})
	}
	report := ask("GetCheckReport", func(c parityClients) (proto.Message, error) {
		r, err := c.check.GetCheckReport(ctx, connect.NewRequest(&webapi.GetCheckReportRequest{Uri: design}))
		return msgOf(r, err)
	})
	sections := report.(*webapi.GetCheckReportResponse).GetReport().GetSections()
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
	ask("RunQuery", func(c parityClients) (proto.Message, error) {
		r, err := c.query.RunQuery(ctx, connect.NewRequest(&webapi.RunQueryRequest{Uri: design, Query: query}))
		return msgOf(r, err)
	})
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

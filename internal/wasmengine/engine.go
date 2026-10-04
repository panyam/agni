// Package wasmengine composes the services the browser engine serves (agni issue 178). It is
// separate from cmd/agni-wasm, which only bridges it to JavaScript, so the composition builds and is
// tested natively, against the server's, without a browser.
package wasmengine

import (
	"io/fs"
	"net/http"

	"github.com/panyam/agni"
	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/internal/version"
	"github.com/panyam/agni/service"

	_ "github.com/panyam/agni/stdlib/lib"           // registers the shipped derived relations (net.has_test_point, ...)
	_ "github.com/panyam/agni/stdlib/relations"     // registers the built-in EDB query relations (netlist/board/datasheet)
	_ "github.com/panyam/agni/stdlib/reviewquery"   // compiles a review manifest's inline query bindings as datalog
	_ "github.com/panyam/agni/stdlib/rules/builtin" // registers the built-in EE rule catalog (anonymous source)
	_ "github.com/panyam/agni/stdlib/rules/datalog" // registers the "dl" datalog-authored rule source
)

// Namespace is the global the wasm build's exports live under, globalThis.agni.
const Namespace = "agni"

// BrowserMount is the mount a page puts dropped files in (agni issue 854). Its files exist only in
// the visitor's browser, and a `.zip` in it reads as the folder it was made from.
const BrowserMount = "local"

// Build composes the engine over root, whose top-level directories are the mounts, which is how
// goapplib's wasmhost holds them. It is the rebuild function cmd/agni-wasm hands the host.
func Build(root fs.FS) (http.Handler, error) {
	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		return nil, err
	}
	var ms []fshost.Mount
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub, err := fs.Sub(root, e.Name())
		if err != nil {
			return nil, err
		}
		if e.Name() == BrowserMount {
			sub = fshost.ExpandZips(sub)
		}
		ms = append(ms, fshost.Mount{Name: e.Name(), FS: sub})
	}
	eng, err := New(ms...)
	if err != nil {
		return nil, err
	}
	return eng.Handler, nil
}

// Engine is one composition over one mount table. The browser rebuilds it when a mount changes,
// which is cheaper to reason about than services caching a project tree that moved under them.
type Engine struct {
	// Handler serves the Connect API, as `agni serve` does at the same paths.
	Handler http.Handler
	// Paths are the service paths Handler answers.
	Paths []string
	// Warnings are the legitimate absences agni.New reports, such as no datasheet corpus.
	Warnings []string
}

// New composes the engine over ms through agni.New, which refuses a build missing the rule catalog
// or the fact relations. That refusal matters most here: a browser bundle that silently shipped
// without the built-in rules would report every design clean.
//
// What the browser engine leaves out is deliberate. There is no native renderer (xschem and Lepton
// are processes), no review store (nothing a visitor drops is kept), no datasheet corpus yet (#852
// decides whether one ships), and no query budget, since the only person waiting on a query is the
// one who asked it.
// designCache is how many reads the browser engine keeps: fewer than a server, since a tab's memory
// is the visitor's.
const designCache = 8

func New(ms ...fshost.Mount) (*Engine, error) {
	// Every read goes through one cache, so a second request on a design costs its answer rather than
	// a re-parse (agni issue 895). The engine is rebuilt on every mount change, which drops the cache.
	files := fshost.New(ms...)
	host := service.NewCachingLoader(files, service.NewDesignCache(designCache))
	trees := make([]projects.Tree, 0, len(ms))
	for _, m := range ms {
		trees = append(trees, projects.Tree{Mount: m.Name, FS: m.FS})
	}
	store := projects.NewFSStore(trees...)
	resolver := &service.ProjectResolver{Store: store, Config: &fshost.ConfigResolver{Mounts: ms}}
	e, err := agni.New(agni.WithProjectResolver(resolver), agni.WithProducerVersion(version.Version()))
	if err != nil {
		return nil, err
	}
	check, review := e.RuleServices(agni.RuleServiceDeps{Loader: host})
	mux := http.NewServeMux()
	paths := server.API{
		Workspace: service.NewWorkspaceService(files.Workspace()).WithDesignFiles(resolver, files.Workspace()),
		Project:   service.NewProjectService(store),
		Design:    service.NewDesignService(host, nil, render.DefaultStyle, resolver),
		Check:     check,
		Diff:      service.NewDiffService(host, resolver),
		Query:     service.NewQueryService(host, nil, resolver),
		Review:    review,
	}.Register(mux)
	return &Engine{Handler: mux, Paths: paths, Warnings: e.Warnings()}, nil
}

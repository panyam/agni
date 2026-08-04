//go:build js && wasm

// agni-wasm compiles the engine's in-process services to WebAssembly so the docs
// site (and any static host) can run real Design/Check/Query RPCs entirely in the
// browser — no server. It builds the SAME Connect mux serve.go builds, over an
// in-memory-file loader, and bridges http.Handler.ServeHTTP to a JS function.
//
// The design bytes live in Go's js/wasm filesystem, populated from JS (a preloaded
// seed or a user upload) through a small in-memory `globalThis.fs` shim; the reader
// registry opens them with os as usual, so no engine code changes. The JS side wraps
// a Connect transport around `agniHTTP`, and the real web-app panels talk to it
// exactly as they talk to `agni serve`. See WS14-006.
package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"syscall/js"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/readers/formats"
)

// memLoader is the wasm service.Loader: the osLoader logic (mounts.Resolve +
// formats.Loader), unchanged. File reads resolve through Go's js/wasm os, backed
// by the JS in-memory fs shim, so a preloaded/uploaded design just works.
type memLoader struct {
	mounts []mounts.Mount
	loader *formats.Loader
}

func symbolsFor(faithful bool) string {
	if faithful {
		return formats.SymbolsFaithful
	}
	return formats.SymbolsGlyph
}

func (l *memLoader) Design(_ context.Context, m, path string) (*ir.Design, error) {
	abs, err := mounts.Resolve(l.mounts, m, path)
	if err != nil {
		return nil, err
	}
	return l.loader.ReadDesign(abs)
}

func (l *memLoader) Geometry(_ context.Context, m, path, layout string, faithful bool) (*geom.SchematicGeometry, error) {
	abs, err := mounts.Resolve(l.mounts, m, path)
	if err != nil {
		return nil, err
	}
	return l.loader.ResolveGeometry(abs, layout, nil, symbolsFor(faithful))
}

func (l *memLoader) Report(_ context.Context, m, path string, faithful bool) (*graph.ConversionReport, error) {
	abs, err := mounts.Resolve(l.mounts, m, path)
	if err != nil {
		return nil, err
	}
	return l.loader.ConversionReport(abs, symbolsFor(faithful), nil)
}

func (l *memLoader) Expectations(_ context.Context, m, path string) (*expect.Expectations, error) {
	// A docs demo carries no expectation sidecars; absence is normal (nil, nil).
	return nil, nil
}

func (l *memLoader) Board(_ context.Context, m, path string) (*geom.BoardGeometry, error) {
	abs, err := mounts.Resolve(l.mounts, m, path)
	if err != nil {
		return nil, err
	}
	return l.loader.BoardGeometry(abs)
}

// buildMux wires the Design/Check/Query Connect handlers — the demos' surface —
// exactly as serve.go does. Datasheet/native/review/pages are omitted (a static
// docs demo needs none).
func buildMux() *http.ServeMux {
	loader := &memLoader{
		mounts: []mounts.Mount{{Name: "designs", Root: "/designs"}},
		loader: &formats.Loader{},
	}
	mux := http.NewServeMux()
	dsPath, dsHandler := webapiconnect.NewDesignServiceHandler(server.NewDesign(service.NewDesignService(loader, nil, render.DefaultStyle)))
	mux.Handle(dsPath, dsHandler)
	ckPath, ckHandler := webapiconnect.NewCheckServiceHandler(server.NewCheck(service.NewCheckService(loader, check.DefaultCatalog(), nil)))
	mux.Handle(ckPath, ckHandler)
	qPath, qHandler := webapiconnect.NewQueryServiceHandler(server.NewQuery(service.NewQueryService(loader, nil)))
	mux.Handle(qPath, qHandler)
	return mux
}

// agniHTTP(method, url, bodyUint8Array) -> { status, body: Uint8Array } runs one
// request through the in-process mux. The JS Connect transport calls this instead
// of fetch, so the real panels' clients drive the wasm engine.
func agniHTTP(mux *http.ServeMux) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		method := args[0].String()
		url := args[1].String()
		body := make([]byte, args[2].Get("length").Int())
		js.CopyBytesToGo(body, args[2])

		req := httptest.NewRequest(method, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		out := rec.Body.Bytes()
		buf := js.Global().Get("Uint8Array").New(len(out))
		js.CopyBytesToJS(buf, out)
		res := js.Global().Get("Object").New()
		res.Set("status", rec.Code)
		res.Set("body", buf)
		res.Set("contentType", rec.Header().Get("Content-Type"))
		return res
	})
}

func main() {
	mux := buildMux()
	js.Global().Set("agniHTTP", agniHTTP(mux))
	js.Global().Get("console").Call("log", "agni-wasm ready")
	select {} // keep the runtime alive for the exported callback
}

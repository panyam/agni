//go:build js && wasm

// agni-wasm is the engine as WebAssembly, for a browser to run with no server (agni issue 178). It
// serves the same Connect API `agni serve` does, composed by internal/wasmengine over designs the page
// hands it, and is meant to run in a Web Worker so a check that takes seconds never blocks the page.
//
// It exports three functions on the global object:
//
//	agniMount(name, files)               adds files, a {path: Uint8Array} object, to mount `name`
//	agniUnmount(name)                    removes a mount
//	agniHTTP(method, url, headers, body) one HTTP request, answered as a Promise of {status, headers, body}
//
// The bytes are pushed in before a request rather than read during one. fs.FS is synchronous and
// every source of bytes in a browser is asynchronous, so reading mid-request would mean blocking Go
// on a JS Promise inside a callback, which deadlocks the event loop that would settle it.
package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"syscall/js"

	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/internal/wasmengine"
)

// state is the mount table and the engine composed over it. A mount change recomposes the engine, so
// no service holds a project tree that has since been replaced. A mount accumulates files: the page
// brings each design in as it is first named (agni issue 853), and two designs in one mount share it.
type state struct {
	mu     sync.Mutex
	mounts map[string]map[string][]byte
	engine *wasmengine.Engine
	err    error
}

func (s *state) recompose() {
	names := make([]string, 0, len(s.mounts))
	for n := range s.mounts {
		names = append(names, n)
	}
	sort.Strings(names)
	ms := make([]fshost.Mount, 0, len(names))
	for _, n := range names {
		ms = append(ms, fshost.Mount{Name: n, FS: fshost.MemFS(s.mounts[n])})
	}
	s.engine, s.err = wasmengine.New(ms...)
}

func (s *state) handler() (http.Handler, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	return s.engine.Handler, nil
}

func (s *state) mount(this js.Value, args []js.Value) any {
	if len(args) != 2 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeObject {
		return jsError("agniMount(name, {path: Uint8Array})")
	}
	name, obj := args[0].String(), args[1]
	keys := js.Global().Get("Object").Call("keys", obj)
	files := make(map[string][]byte, keys.Length())
	for i := 0; i < keys.Length(); i++ {
		p := keys.Index(i).String()
		v := obj.Get(p)
		b := make([]byte, v.Get("length").Int())
		js.CopyBytesToGo(b, v)
		files[p] = b
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.mounts[name]
	if m == nil {
		m = map[string][]byte{}
		s.mounts[name] = m
	}
	for p, b := range files {
		m[p] = b
	}
	s.recompose()
	if s.err != nil {
		return jsError(s.err.Error())
	}
	return nil
}

func (s *state) unmount(this js.Value, args []js.Value) any {
	if len(args) != 1 || args[0].Type() != js.TypeString {
		return jsError("agniUnmount(name)")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.mounts, args[0].String())
	s.recompose()
	return nil
}

// serve answers one request as a Promise. The handler runs on a goroutine of its own and the callback
// returns at once, because a js.FuncOf callback that blocks pauses the JS event loop until it returns.
func (s *state) serve(this js.Value, args []js.Value) any {
	if len(args) != 4 {
		return rejected("agniHTTP(method, url, headers, body)")
	}
	method, url := args[0].String(), args[1].String()
	headers := map[string]string{}
	if h := args[2]; h.Type() == js.TypeObject {
		keys := js.Global().Get("Object").Call("keys", h)
		for i := 0; i < keys.Length(); i++ {
			k := keys.Index(i).String()
			headers[k] = h.Get(k).String()
		}
	}
	var body []byte
	if b := args[3]; b.Type() == js.TypeObject {
		body = make([]byte, b.Get("length").Int())
		js.CopyBytesToGo(body, b)
	}
	var executor js.Func
	executor = js.FuncOf(func(_ js.Value, pa []js.Value) any {
		resolve, reject := pa[0], pa[1]
		go func() {
			defer executor.Release()
			h, err := s.handler()
			if err != nil {
				reject.Invoke(js.Global().Get("Error").New(err.Error()))
				return
			}
			req := httptest.NewRequest(method, url, bytes.NewReader(body))
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			out := js.Global().Get("Uint8Array").New(rec.Body.Len())
			js.CopyBytesToJS(out, rec.Body.Bytes())
			hdrs := js.Global().Get("Object").New()
			for k := range rec.Header() {
				hdrs.Set(k, rec.Header().Get(k))
			}
			res := js.Global().Get("Object").New()
			res.Set("status", rec.Code)
			res.Set("headers", hdrs)
			res.Set("body", out)
			resolve.Invoke(res)
		}()
		return nil
	})
	return js.Global().Get("Promise").New(executor)
}

func jsError(msg string) js.Value { return js.Global().Get("Error").New(msg) }

func rejected(msg string) js.Value {
	return js.Global().Get("Promise").Call("reject", jsError(msg))
}

func main() {
	s := &state{mounts: map[string]map[string][]byte{}}
	s.recompose()
	if s.err != nil {
		// A build missing a registration seam composes nothing, and saying so beats an engine that
		// answers every design with a clean report.
		js.Global().Get("console").Call("error", fmt.Sprintf("agni-wasm: %v", s.err))
	}
	js.Global().Set("agniMount", js.FuncOf(s.mount))
	js.Global().Set("agniUnmount", js.FuncOf(s.unmount))
	js.Global().Set("agniHTTP", js.FuncOf(s.serve))
	if ready := js.Global().Get("agniReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}

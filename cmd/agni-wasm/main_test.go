//go:build js && wasm

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall/js"
	"testing"

	"github.com/panyam/agni/internal/wasmengine"
	"github.com/panyam/goapplib/wasmhost"
)

// This runs under Node (`make wasm-test`), as the browser's worker does: the tutorial project goes in
// through globalThis.agni.add and a check report comes out of globalThis.agni.http, both Promises.
// What the report SAYS is held to the server's by TestWasmEngineAnswersAsTheServerDoes in cmd/agni,
// natively. This is the part that needs a JavaScript host: that the exports exist under agni's
// namespace, take bytes, and answer without deadlocking.

// await blocks the test until p settles. The test body is not a js.FuncOf callback, so blocking here
// lets the event loop run the Promise's work.
func await(t *testing.T, p js.Value) js.Value {
	t.Helper()
	type settled struct {
		v   js.Value
		err string
	}
	done := make(chan settled, 1)
	ok := js.FuncOf(func(_ js.Value, a []js.Value) any { done <- settled{v: a[0]}; return nil })
	fail := js.FuncOf(func(_ js.Value, a []js.Value) any { done <- settled{err: a[0].Call("toString").String()}; return nil })
	defer ok.Release()
	defer fail.Release()
	p.Call("then", ok, fail)
	s := <-done
	if s.err != "" {
		t.Fatal(s.err)
	}
	return s.v
}

func TestEngineAnswersThroughTheJSExports(t *testing.T) {
	h := wasmhost.New(wasmengine.Namespace)
	if err := h.Rebuild(wasmengine.Build); err != nil {
		t.Fatalf("compose: %v", err)
	}
	release := h.Export()
	defer release()
	agni := js.Global().Get(wasmengine.Namespace)

	root := filepath.Join("..", "..", "examples", "tutorial-project")
	files := js.Global().Get("Object").New()
	n := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		arr := js.Global().Get("Uint8Array").New(len(b))
		js.CopyBytesToJS(arr, b)
		files.Set(filepath.ToSlash(rel), arr)
		n++
		return nil
	})
	if err != nil || n == 0 {
		t.Fatalf("read the fixture: %d files, %v", n, err)
	}
	await(t, agni.Call("add", "tut", files))

	post := func(path, body string) (int, []byte) {
		arr := js.Global().Get("Uint8Array").New(len(body))
		js.CopyBytesToJS(arr, []byte(body))
		headers := js.Global().Get("Object").New()
		headers.Set("Content-Type", "application/json")
		res := await(t, agni.Call("http", "POST", path, headers, arr))
		out := make([]byte, res.Get("body").Get("length").Int())
		js.CopyBytesToGo(out, res.Get("body"))
		return res.Get("status").Int(), out
	}
	// A question names a snapshot, so the design is ingested first, under the id a page would mint
	// (agni issue 1061).
	if status, out := post("/agni.v1.webapi.SnapshotService/Ingest", `{"snapshot":{"uri":"mount://tut/designs/gateway"},"snapshotId":"js"}`); status != 200 {
		t.Fatalf("Ingest: status %d, body %.300s", status, out)
	}
	status, out := post("/agni.v1.webapi.CheckService/GetCheckReport", `{"snapshot":"snapshots/js"}`)
	if status != 200 {
		t.Fatalf("GetCheckReport: status %d, body %.300s", status, out)
	}
	if !strings.Contains(string(out), "gateway-profiles/") {
		t.Errorf("GetCheckReport carries no rule from the project's own profiles, so the project config did not reach the engine:\n%.500s", out)
	}
	if !strings.Contains(string(out), `"sourceFile":"designs/gateway/gateway.edn"`) {
		t.Errorf("GetCheckReport has no finding located in designs/gateway/gateway.edn:\n%.500s", out)
	}
}

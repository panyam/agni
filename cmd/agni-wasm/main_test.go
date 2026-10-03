//go:build js && wasm

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall/js"
	"testing"
)

// This runs under Node (`make wasm-test`), as the browser's worker will: the tutorial project goes in
// through agniMount and a check report comes out of agniHTTP's Promise. What the report SAYS is held
// to the server's by TestWasmEngineAnswersAsTheServerDoes in cmd/agni, natively. This is the part that
// needs a JavaScript host: that the exports exist, take bytes, and answer without deadlocking.

func TestEngineAnswersThroughTheJSExports(t *testing.T) {
	s := &state{mounts: map[string]map[string][]byte{}}
	s.recompose()
	if s.err != nil {
		t.Fatalf("compose: %v", s.err)
	}
	mount, serve := js.FuncOf(s.mount), js.FuncOf(s.serve)
	defer mount.Release()
	defer serve.Release()

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
	if res := mount.Invoke("tut", files); !res.IsNull() && !res.IsUndefined() {
		t.Fatalf("agniMount: %s", res.Call("toString").String())
	}

	body := []byte(`{"uri":"mount://tut/designs/gateway"}`)
	arr := js.Global().Get("Uint8Array").New(len(body))
	js.CopyBytesToJS(arr, body)
	headers := js.Global().Get("Object").New()
	headers.Set("Content-Type", "application/json")
	promise := serve.Invoke("POST", "/agni.v1.webapi.CheckService/GetCheckReport", headers, arr)

	type result struct {
		status int
		body   string
		err    string
	}
	done := make(chan result, 1)
	onOK := js.FuncOf(func(_ js.Value, a []js.Value) any {
		b := make([]byte, a[0].Get("body").Get("length").Int())
		js.CopyBytesToGo(b, a[0].Get("body"))
		done <- result{status: a[0].Get("status").Int(), body: string(b)}
		return nil
	})
	onErr := js.FuncOf(func(_ js.Value, a []js.Value) any {
		done <- result{err: a[0].Call("toString").String()}
		return nil
	})
	defer onOK.Release()
	defer onErr.Release()
	promise.Call("then", onOK, onErr)
	r := <-done

	if r.err != "" || r.status != 200 {
		t.Fatalf("GetCheckReport: status %d, err %q, body %.300s", r.status, r.err, r.body)
	}
	if !strings.Contains(r.body, "gateway-profiles/") {
		t.Errorf("GetCheckReport carries no rule from the project's own profiles, so the project config did not reach the engine:\n%.500s", r.body)
	}
	if c := strings.Count(r.body, `"sourceFile":"designs/gateway/gateway.edn"`); c == 0 {
		t.Errorf("GetCheckReport has no finding located in designs/gateway/gateway.edn:\n%.500s", r.body)
	}
}

//go:build js && wasm

// agni-wasm is the engine as WebAssembly, for a browser to run with no server (agni issue 178). It
// serves the same Connect API `agni serve` does, composed by internal/wasmengine, and runs in a Web
// Worker so a check that takes seconds never blocks the page.
//
// The host is goapplib's wasmhost (agni issue 863). It exports globalThis.agni.{http, mount, add,
// unmount} as Promises, keeps every mount under one root fs.FS, and rebuilds the engine after each
// mount change, so no service holds a project tree that has since been replaced. The page adds a
// design's files the first time a request names it (agni issue 853), which is why it uses `add`.
package main

import (
	"github.com/panyam/agni/internal/wasmengine"
	"github.com/panyam/goapplib/wasmhost"
)

func main() {
	wasmhost.ServeRebuild(wasmengine.Namespace, wasmengine.Build)
}

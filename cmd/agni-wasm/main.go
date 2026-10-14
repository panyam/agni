//go:build js && wasm

// agni-wasm is the engine as WebAssembly, for a browser to run with no server (agni issue 178). It
// serves the same Connect API `agni serve` does, composed by internal/wasmengine, and runs in a Web
// Worker so a check that takes seconds never blocks the page.
//
// The host is goapplib's wasmhost (agni issue 863). It exports globalThis.agni.{http, mount, add,
// unmount} as Promises, keeps every mount under one root fs.FS, and rebuilds the engine after each
// mount change, so no service holds a project tree that has since been replaced. The page adds a
// design's files the first time a request names it (agni issue 853), which is why it uses `add`.
//
// Every parsed design, drawing and board is also kept in the origin's private file system
// (wasmhost.BrowserCache), which every worker of the page shares and a reload keeps, so a second lane,
// a lane's replacement or the next visit restores a design rather than reading it again (agni issue
// 911).
package main

import (
	"github.com/panyam/agni/internal/wasmengine"
	"github.com/panyam/goapplib/wasmhost"
)

// The collector runs at Go's default GOGC of 100 on purpose (agni issue 988). Wasm memory never shrinks,
// so a lower GOGC looked like a way to hold a worker near its live heap, and in Node it is: a Jetson
// check run peaks at 1020 MB at 50 against 1364 MB at 100. In Chromium it saved 130 MB on a first
// visit's jobs worker, nothing on a reloaded page's (1609 and 1614 MB, a peak Node does not show), and
// cost 14% on Run checks. 35 and 25 peaked higher than 100 in wasm (1619 and
// 1506 MB), and so did a GOMEMLIMIT of 700 to 900 MiB (1325 to 1738 MB). `make wasm-layers` with GOGC
// or GOMEMLIMIT set and `make wasm-bench` measure it again.
func main() {
	wasmhost.ServeRebuild(wasmengine.Namespace, wasmengine.BuildWith(wasmengine.Options{Store: wasmhost.BrowserCache()}))
}

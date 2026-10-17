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
	"runtime/debug"

	"github.com/panyam/agni/internal/wasmengine"
	"github.com/panyam/goapplib/wasmhost"
)

// The collector's automatic cycles are OFF (agni issue 1010), and the engine collects between
// requests and after each layer it loads instead (wasmengine.Options.CollectBetweenRequests). Go's wasm
// port has one thread and no asynchronous preemption, so a concurrent cycle runs beside a request and
// slows it until the cycle ends: two of them cost 5 s of a 7.8 s Jetson check, and with them gone the
// check took 2.6 s, while a forced collection over the same heap takes 20 to 57 ms. In Chromium, Run
// checks on Jetson went from 2926 to 1173 ms on one worker and from 4780 to 2397 ms over two lanes. A
// request's garbage now stands until it ends, which raised the serve worker after a read from 355 to
// 621 MB and the two-lane peak from 873 to 1159 MB (`make wasm-bench`). Changing GOGC instead only
// moves where a cycle lands: 200 and 400 took 6.2 and 8.7 s, and GOGC=off under a GOMEMLIMIT of
// 700 MiB took 12.7 s.
//
// Before that the collector ran at Go's default GOGC of 100 on purpose (agni issue 988). Wasm memory never shrinks,
// so a lower GOGC looked like a way to hold a worker near its live heap, and in Node it is: a Jetson
// check run peaks at 1020 MB at 50 against 1364 MB at 100. In Chromium it saved 130 MB on a first
// visit's jobs worker, nothing on a reloaded page's (1609 and 1614 MB, a peak Node does not show), and
// cost 14% on Run checks. 35 and 25 peaked higher than 100 in wasm (1619 and
// 1506 MB), and so did a GOMEMLIMIT of 700 to 900 MiB (1325 to 1738 MB). `make wasm-layers` with GOGC
// or GOMEMLIMIT set and `make wasm-bench` measure it again.
//
// What does lower the peak is one collection at the right moment. The engine collects after each layer
// its DesignCache reads or restores (service.DesignCache.AfterLoad, agni issue 1000), which took the same
// check from 1343 to 805 MB on a first visit, since the rules then reuse what the board's parse left
// rather than growing past it.
func main() {
	debug.SetGCPercent(-1)
	wasmhost.ServeRebuild(wasmengine.Namespace, wasmengine.BuildWith(wasmengine.Options{Store: wasmhost.BrowserCache(), CollectBetweenRequests: true}))
}

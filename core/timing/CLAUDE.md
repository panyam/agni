# core/timing

This file covers how agni measures a slow request, a browser draw and a browser worker's memory.

## Timing a request

- **Measure a slow request before optimising it** (agni 914).
- `--timing` on `check`, `query`, `trace` and `review` prints each stage, the cache tier that answered each read, and the slowest rules.
- `query --explain` prints the plan.
- `agni serve --slow-request 2s` logs the same breakdown per request.
- An `Agni-Timing: 1` header returns the breakdown from the server or the browser engine. The viewer's `?timing` and the exercise's `--timing` send it.
- **A stage is recorded with `timing.Begin(ctx, "name")()`.** It is a no-op when nobody asked.
- The record travels in headers, so no answer changes from run to run.
- **`Agni-Timing: memory` shows which stage raised a peak** (agni 994). The page's `?timing=memory` and the CLI's `--timing-memory` send it. Each stage carries the memory as it ended, and each rule carries what it allocated.

## Timing a page

- **Under `?timing` the page logs each request's start and wall time** on its own clock (agni 968). The exercise's `--timing` turns them into a `checks breakdown` row. The row holds every request of a Run checks plus the page's own time.
- **`?timing` logs one `agni draw <view>` line per burst of pan and zoom** (agni 1331, `web/src/drawprofile.ts`). The line carries the GPU, the scene's counts, and each frame's draw, text-layer and draw-call figures.
- The draw line's type has no field for design text, so a team can paste it into a public issue. `drawprofile.test.ts` fails on any other string.
- **A walk reports loading a design apart from asking it questions** (agni 1060). `1. load` and `checks load` happen once per visit. `checks rules`, `checks again` and the query rows show a question's cost on a loaded design. A mission log leads with those.
- `wasm-bench` prints its rows in the same two groups.
- Compare a page's wall time with the engine's `totalMicros` before blaming the engine (agni 1007). A warm query can spend most of its time outside the engine.

## Browser worker memory

- **`make wasm-layers` shows what each layer of an open design holds**, native and wasm. It passes `GOGC` and `GOMEMLIMIT` through.
- **`make wasm-bench` shows each lane's wasm memory beside its live Go heap.** For a Jetson check it also prints each stage's memory and each rule's allocation.
- **Wasm memory never shrinks**, and collecting harder does not lower it reliably (agni 988). A lower GOGC or a memory limit peaked HIGHER than the default on Jetson.
- **One collection after each layer the cache reads does lower the peak** (agni 1000). The hook is `DesignCache.AfterLoad`, and only the browser engine sets it.
- **The browser engine's automatic collection is OFF** (agni 1010). Go's wasm port has one thread, so a concurrent cycle runs beside a rule and slows it.
- The engine collects after a layer loads and after a request that doubled the heap. It also collects between a read's files once the heap is 128 MB past what last survived (`wasmengine.Options.CollectBetweenRequests`).
- **A rule that costs seconds in the browser and milliseconds natively points at collection.** Read `make wasm-layers GODEBUG=gctrace=1` before reading the rule.

## Two bench rows that mislead

- **The one-worker `check_ms` is a check after an aborted one.** It reuses the first run's work or not, depending on where the abort landed, so it varies between identical runs.
- **`query_after_restart_ms` with `lane_restarts` 0** is the same query repeated on a warm worker.

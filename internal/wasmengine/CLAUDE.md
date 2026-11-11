# internal/wasmengine

This file covers the engine as it runs in the browser, and the traps in its workers.

## What it is

- **The engine also runs in the browser.** `cmd/agni-wasm` runs on goapplib's `wasmhost`, and `internal/wasmengine` composes it over `fshost`. `fshost` implements the service ports over `fs.FS` mounts.
- A design keeps one URL, `/designs/<mount>/<path>/view`. The ENGINE is a separate per-page choice, made by `agni serve --engine wasm`, a page's `?engine=`, or a static page.
- On wasm the page lists a design's files (`ListDesignFiles`) and fetches them from `/raw/<mount>/<path>`.
- The page sends a design over `--wasm-max-bytes` to the server instead. The limit is 128 MB, measured with `make wasm-bench`.
- The engine runs as a `serve` worker and a `jobs` worker (#911).
- Every worker keeps what it parsed in the origin's private file system, keyed on `wasmengine.StoreVersion`.
- **A worker checks what it restores by the page's listed hashes** (agni 1515). The page writes `.agni-sums.json` at each mount's root with every file's listed sha256 and size. So the serve worker restores a board it holds only as a placeholder, without the board's bytes. A mount without the manifest is checked by CRC-64 of its bytes, as the server is. A wasm built with plain `go build` from a dirty tree persists nothing. A wasm built by `make wasm` names its uncommitted diff.
- Dropped files go into the browser mount `local` and the browser's IndexedDB (`web/src/drops.ts`, agni 1072), so they survive a reload. Nothing ever lists `local` on a server. `ProposeDesigns` and `agni propose` group the dropped files into designs.
- `agni site` (`make demo-site`) writes the whole thing as static files under a prefix.
- `make wasm wasm-test` builds the engine and smoke-tests it under Node.
- `architecture/web-services.md`, "The same contract in the browser", has the architecture.

## Traps

- **Never block inside a `js.FuncOf` waiting on a JS promise.** Blocking pauses the event loop, and the promise needs that loop. So bytes are PUSHED into the worker before a request.
- **goutils' `memfs` refuses a file under a path that is a file.** A descriptor beside a design from a zip goes through `fshost`'s `.agni-overlay/`.
- **The server and the worker read through different adapters** (`osLoader`, `fshost`). `TestWasmEngineAnswersAsTheServerDoes` holds them to identical answers. A new served question belongs in it.
- **A browser worker replaced after every request keeps nothing in memory.** `DesignCache`'s memory entries die with the worker, and its replacement fetches the design's files again.
- So the serve lane replaces a worker past its limit only when `wouldShrink` in `web/src/wasm/client.ts` says a fresh worker would come back smaller. It judges on the live Go heap that goapplib reports (agni 990).
- The symptom of a bad limit is the console line `the serve worker held N MB, M MB of it live, and was replaced` after every query. Measure with `make wasm-bench` before tightening `SERVE_LANE_MAX_BYTES` or `SHRINK_RATIO`.

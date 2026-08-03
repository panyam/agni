# Stack & architecture decision

See [README](README.md). Settled 2026-07-02. Enforceable rules live in
[/CONSTRAINTS.md](../CONSTRAINTS.md). Builds on the ingestion/IR design in
[13-ingestion-ir-architecture](13-ingestion-ir-architecture.md).

## Decisions

- **Engine language: Go, maximally.** Parsing, IR, diff, validation, and simulation
  are backend-heavy and Go-native. Reuses the existing stack (`goutils`, `gocurrent`
  for later fan-out; `devloop` for the dev loop).
- **IR: Protobuf.** One `.proto` is the source of truth; codegen produces Go structs
  and TS types (and WASM bindings) via the `protokit` / `protoc-gen-go-wasmjs`
  pipeline. Go and TS consume the same schema and never drift. Proto's built-in
  unknown-field retention gives lossless / forward-compat for free (see [13](13-ingestion-ir-architecture.md)).
- **EDIF reader: hand-rolled in Go.** EDIF is S-expressions, trivial to parse. No
  need for Galore here.
- **Galore + tlex (TypeScript): reserved for the browser DSL IDE** (WS3). They are TS
  and built for web IDEs, so that is their lane: syntax highlighting, live parse,
  error squiggles for the rules DSL. Not used for ingestion.
- **UI: thin TS view; Model-View-Presenter.** The presenter pattern is mandatory; its
  runtime is per-surface (see [15-presenter-contract](15-presenter-contract.md)).
  - **View (TS):** pure function of the view-model. Captures input, forwards semantic
    intents, executes canvas/WebGL draw calls, owns GPU buffers, DOM/CSS, text.
  - **Presenter (per-surface runtime):** owns interaction logic, intents -> view-model.
    WASM (Go) for lower-frequency surfaces (viewer); TS for high-frequency ones (editor,
    live DSL) to avoid per-event boundary cost.
  - **Core domain logic (Go, always):** IR, diff, rules, sim. Runtime-agnostic,
    authoritative, runs headless in Go tests, CLI, backend; called by every presenter.
  - Unidirectional data flow (Elm/redux-shaped).
- **WASM is an optional code-reuse mechanism, not a speed hack and not mandatory.** Demo
  one needs no WASM (netlist diff is text/graph). WASM enters as the viewer's presenter
  runtime; high-frequency surfaces use a TS presenter instead.

## Keeping the WASM boundary cheap

Go/WASM has higher per-call boundary overhead than Rust/C WASM, so the boundary
carries meaning, not pixels:
- **Layer the render model:** static geometry uploaded once (GPU-resident); only a
  tiny dynamic overlay (highlight/selection/cursor) crosses per frame.
- **Batch input to rAF:** one intent batch per frame, not per raw event.
- **Camera is view-local:** pan/zoom affine transform applied in TS/GPU; the
  presenter is told the new viewport only when culling/hit-testing needs it. Dragging
  stays smooth even if a WASM frame lags.
- Bulk data via typed arrays / linear memory; O(a few) crossings per frame.

At 60fps (~16ms/frame; 30fps acceptable) this keeps interop well under 1ms. The real
perf ceiling is geometry volume and rendering (solve with canvas/WebGL + viewport
culling + spatial index), not the boundary.

## Caveats

- **Text metrics are browser-owned.** If the presenter lays out labels it needs
  measurements back from the view; design a small measurement callback.
- **Go/WASM binaries are large** (runtime + GC). Load-time concern; compress, or
  TinyGo if it bites.
- **Don't pre-optimize.** Build the viewer, profile, push work into WASM only if TS
  can't keep up.

## Repo layout (demo one)

Go module at repo root. `proto/` for the IR schema, `internal/` for reader/ir/diff,
`cmd/edifdiff/` for the CLI. TS/viewer added later under its own dir.

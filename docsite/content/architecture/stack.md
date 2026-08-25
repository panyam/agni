---
title: "Stack and platform"
description: "The Go engine, the protobuf IR, the optional WebAssembly path, the thin TypeScript view, and the boundaries between them."
---

The platform is a Go engine with a protobuf intermediate representation, a thin TypeScript view,
and WebAssembly as an optional way to reuse the Go logic in the browser. This page records why
each piece is where it is and where the boundaries fall.

## The engine is Go

Parsing, the IR, diff, rules, and later simulation are backend-heavy work that Go handles
directly. The engine reuses shared Go libraries for utilities and, later, for concurrency when
work fans out. All of it runs headless: in Go tests, in the CLI, and in the backend.

The engine packages stay pure. A reader takes an `io.Reader` or bytes, not a file path, and diff
and checks operate on the IR. Keeping the packages free of file paths, a database, and any
browser-only calls is what lets the same code run in every one of those places.

<details>
<summary>Why there is no parser-generator in the ingestion path</summary>

Not every part of a schematic file needs a hand-rolled parser, and none of them needs a generated
one. EDIF is S-expressions, which are trivial to parse directly in Go with no parser-generator
involved. An in-house parser-generator toolkit that targets the browser is reserved for a future
rules-DSL editor, where syntax highlighting, live parsing, and inline error markers are the job.
Ingestion has none of those requirements.

</details>

## Package layout

The top-level tree separates the engine from the content it evaluates, and that split
makes the open-core boundary real: the engine ships as a library and the rule content is one of
several sources that register into it.

{{ includeFile "figures/engine-layers.svg" }}

- `core/` is the pure engine, and it owns the evaluation machinery and no rules: the IR model, the
  net solver, `check` (the rule runtime and the spec interpreter), `query` (the Datalog evaluator),
  `diff`, `render`, and `svg`.
- `stdlib/` is the standard content that registers into the engine through public seams. Each rule
  and relation keeps its reference markdown beside its code, embedded and served as the runtime
  `Detail`.
- `readers/` holds the format readers plus `readers/formats`, the registry and loader that own all
  file I/O so the core never opens a file.
- `datasheet/` is the parameter and document stack: `datasheet/param`, `datasheet/doc`, and
  `datasheet/derive`.
- `cmd/` is the CLI, `protos/` and `gen/` are the schema and its generated code, `internal/`
  holds engine-private helpers, and `docsite/` is this documentation site, a separate module.

The `core` and `stdlib` boundary is load-bearing: no `core` package depends on `stdlib` in its
production build, so the engine has no built-in rules baked in, and a program composes the catalog
it wants by importing the sources it wants. An overlay adds its own rules the same way the standard
library does.

## The IR is protobuf

One `.proto` schema is the source of truth. Code generation produces Go structs, TypeScript
types, and WebAssembly bindings from it, so Go and TypeScript consume the same schema and cannot
drift. Protobuf's built-in retention of unknown fields also gives lossless round-tripping and
forward compatibility for free, which matters when a reader meets a construct a later schema will
name but the current one does not. The [ingestion and IR](../ingestion-and-ir/) page covers how
readers populate it.

## The view is thin

The UI follows a Model-View-Presenter split, with the view a pure function of a view-model. The
view captures input, forwards semantic intents, executes canvas and WebGL draw calls, owns the
GPU buffers and the DOM, and measures text. It holds no domain logic. Data flows one way, in the
shape of an Elm or Redux app. The [web app and presenter](../web-app/) page covers the contract
in full.

Text metrics are the one measurement that has to travel the other way, because only the browser
has them. A presenter laying out labels needs them back, so the contract includes a small
measurement callback.

## WebAssembly is optional

WebAssembly is a code-reuse mechanism, not a speed trick and not a requirement. The shipped
viewer runs entirely without it, keeping the engine server-side behind the Connect API and a thin
TypeScript presenter in the browser. WebAssembly enters only if a surface wants to reuse the Go
diff and IR logic in the browser directly, for an offline or zero-server viewer. A
high-frequency surface such as an editor would use a TypeScript presenter instead, to avoid a
per-event boundary crossing.

The order of work is build the viewer, profile it, and push work into WebAssembly only if
TypeScript cannot keep up. There is no reason to pre-optimize a boundary before a real surface
demands it.

<details>
<summary>How the boundary would be kept cheap, if a surface ever needs it</summary>

Go compiled to WebAssembly has higher per-call boundary overhead than a C or Rust equivalent, so
if the presenter runs in the browser the boundary is designed to carry meaning, not pixels.

- The render model is layered. Static geometry is uploaded to the GPU once and stays resident.
  Only a small dynamic overlay for highlight, selection, and cursor crosses the boundary per
  frame.
- Input is batched to the animation frame. One intent batch per frame, not one per raw event.
- The camera is view-local. Pan and zoom are an affine transform applied in TypeScript and the
  GPU, and the presenter is told the new viewport only when culling or hit-testing needs it, so
  dragging stays smooth even if a presenter frame lags.
- Bulk data crosses as typed arrays over linear memory, so there are only a handful of crossings
  per frame.

At 60 frames per second, roughly 16 ms per frame, this keeps interop well under a millisecond.
The real performance ceiling is geometry volume and rendering, solved with canvas or WebGL,
viewport culling, and a spatial index, not the boundary.

A Go WebAssembly binary is also large, because it carries the runtime and garbage collector. That
is a load-time concern to address with compression, or a smaller Go toolchain, only if it becomes
a problem.

</details>

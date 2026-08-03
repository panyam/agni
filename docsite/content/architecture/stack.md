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

Not every part of a schematic file needs a hand-rolled parser. EDIF, for example, is
S-expressions, which are trivial to parse directly in Go with no parser-generator involved. An
in-house parser-generator toolkit that targets the browser is reserved for a future rules-DSL
editor, where syntax highlighting, live parsing, and inline error markers are the job, not for
ingestion.

## WebAssembly is optional

WebAssembly is a code-reuse mechanism, not a speed trick and not a requirement. The shipped
viewer runs entirely without it, keeping the engine server-side behind the Connect API and a thin
TypeScript presenter in the browser. WebAssembly enters only if a surface wants to reuse the Go
diff and IR logic in the browser directly, for an offline or zero-server viewer. A
high-frequency surface such as an editor would use a TypeScript presenter instead, to avoid a
per-event boundary crossing.

### Keeping the WebAssembly boundary cheap

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

## Practical notes

- Text metrics belong to the browser. If a presenter lays out labels it needs measurements back
  from the view, so the contract includes a small measurement callback.
- A Go WebAssembly binary is large, because it carries the runtime and garbage collector. That is
  a load-time concern to address with compression, or a smaller Go toolchain, only if it becomes a
  problem.
- The order of work is build the viewer, profile it, and push work into WebAssembly only if
  TypeScript cannot keep up. There is no reason to pre-optimize the boundary before a real surface
  demands it.

---
title: "Web app and presenter"
description: "The browser viewer and visual diff over agni serve, the mount and service contract, and the presenter pattern behind the client."
---

The browser viewer and visual diff run over `agni serve`. This page covers how designs reach
the browser through the mount model, the wire contract of Connect services, the render and
highlight contracts both renderers share, the presenter pattern the client is built on, and how
that pattern is realized in the shipped viewer.

The web tier owns no engine logic. It chooses which reader, layout, packer, and rule set to
invoke per file and serves the result. The readers, the [diff](../semantic-diff/), the
[checks](../rules-and-checks/), and the [renderers](../geometry-and-rendering/) stay engine
packages that the CLI drives too.

## The presenter pattern

The engine, meaning parse, IR, diff, rules, and later simulation, is logic that must run in three
places: a backend that batch-parses and validates large files, the browser viewer, and the CLI
and tests. That is the shape the presenter pattern is designed for, so it is the architecture
here rather than an analogy borrowed from web apps.

The pattern separates three tiers.

- **The view is TypeScript, always.** It renders and captures raw input. It executes canvas and
  WebGL draw calls, owns GPU buffers and the DOM, and turns a cursor position into a semantic
  intent. It never contains domain logic.
- **The presenter owns interaction logic and turns intents into a view-model.** Its runtime is a
  per-surface choice. A WebAssembly build of the Go presenter suits lower-frequency, read-mostly
  surfaces because it reuses the Go logic directly. A TypeScript presenter suits high-frequency
  surfaces such as continuous dragging or live editing, where crossing a WebAssembly boundary on
  every event would add latency.
- **The core domain logic is Go, always.** IR, diff, rules, and simulation are runtime-agnostic
  and authoritative. They run headless in Go tests, in the CLI, and in the backend, and every
  presenter calls them.

The contract between view and presenter is duplex and semantic in both directions. It is
expressed as typed interfaces, either two proto services when the contract crosses a network or
WebAssembly boundary, or a plain typed TypeScript interface when the presenter is in-process. It
is not an event bus and not a shared store.

- Intents travel up: a component was selected, a net was hovered, the diff filter changed, the
  viewport changed.
- Commands travel down: highlight this net, set the selection, show the diff overlay, set the
  checks list.

Two rules keep the contract clean. Intents are semantic and free of DOM detail. Picking,
hit-testing, and the camera are view-local, so the view owns the spatial index of what it drew
and the presenter never sees pixels or DOM events. And a semantic command is the primitive, with
an HTML-over-the-wire renderer allowed on top of it. Keeping `SetDiffReport(data)` primitive and
`SetDiffReportContent(html)` a renderer on top preserves the ability to swap the frontend.

Canvas commands are semantic by necessity, since a canvas cannot take HTML, so a schematic or PCB
surface receives highlight and overlay commands, not markup. Panels are the deliberate fork,
where a data-heavy panel may take rendered HTML as one renderer over the same typed state.

## Architecture

The request path runs from the browser through two server layers into the engine.

In the browser, leaf UI components (panels, the file tree, controls) push intents to a
TypeScript presenter, which drives a WebGL canvas for the packed render form and an SVG host for
the SVG document and its overlay. The presenter talks to the server over Connect using proto
JSON.

On the server, Connect handlers receive the call and delegate to transport-neutral services. The
services take loaders in and return proto messages out, with no HTTP knowledge. They call the
`formats` loader and registry to read a file, and the render, diff, and check engine packages to
process it. Server-rendered pages carry declared holes that the browser's UI components mount
into, so there is no client-side HTML skeleton to drift from the server's.

The two server layers are deliberate. The transport-neutral services are the reusable surface,
and the Connect handlers are glue over them.

## The mount model

`agni serve [webdir] --mount name=path`, repeatable. The positional argument is the web asset
directory, defaulting to `web`, not a design folder. Designs enter only through mounts, each
exposing one folder to the file browser under a stable name. Every file-addressing request in the
API is a pair of a mount name and a mount-relative path. The server resolves it inside the mount
and rejects any path that escapes it. The browser never sees or sends an absolute filesystem
path.

File I/O stays at this edge. The loader opens files and hands readers an `io.Reader`, including
the multi-file cases such as a KiCad hierarchy or xschem and gEDA symbol resolution, through
openers the reader never constructs itself.

## The services

Everything is Connect, proto-first, in JSON or binary, with a generated TypeScript client. The
split follows resource lifetimes. Workspace navigation, one design's rendering, checks over a
design, the two-design diff, and ad-hoc datalog search are independent concerns with independent
cadences.

| Service | RPC | What it does |
|---|---|---|
| Workspace | ListMounts | the mount names the tree roots on |
| Workspace | ListDir | one directory level, entries plus per-file format label from the reader registry, formatless files hidden client-side |
| Design | GetDesign | load and summarize one design: sheet list, effective layout, available layouts, native availability |
| Design | GetSheet | one rendered sheet, where `format` picks PACKED (columnar bytes for WebGL), SVG (the verification backend), or NATIVE (the format's own tool) |
| Design | HighlightSheet | resolve highlight spec layers against one sheet: PACKED yields primitive-index groups, SVG a transparent same-frame overlay document |
| Design | GetLayoutReport | how an auto-layout drew each component (glyph, box, provided symbol, or unresolved) |
| Check | ListRules | the rule catalog with tags and per-design availability, static per build, fetched once |
| Check | CheckDesign | run a rule subset and return findings, where each subject joins the packed primitive keys for highlighting |
| Check | GetExpectations | the design's expectation sidecar as its own resource, reconciled against findings client-side |
| Check | GetCheckReport | the severity-organized report, the same shape as `agni check --format report` |
| Diff | DiffDesigns | semantic diff of two designs plus the highlight maps, the wire form shared with `agni diff --format json` |
| Query | RunQuery | evaluate an ad-hoc datalog query over the design's fact base, returning columns and provenance-linked rows, the same engine as `agni query` |
| Query | ListRelations | the relation catalog with arg labels, summary, and kind, driving the panel's click-to-insert picker |
| Review | GetReviewManifest | resolve a stored checklist into a manifest value, parsed and validated |
| Review | CreateReview | run a checklist against one design and store the result, returning the stored run |
| Review | GetReview | one stored run, by resource name |
| Review | ListReviews | stored runs newest first, paginated, filterable by design |
| Review | DeleteReview | remove a stored run |

A few contract details bite if missed.

- **Effective values echo back.** GetDesign returns the layout actually used, since a request for
  an unavailable layout resolves rather than erroring, and the client adopts the echo. The same
  holds for the sheet id. The client navigates by the ids GetDesign returned, never by index
  guesses, because sheet ids are non-numeric on purpose and a numeric selector means a positional
  index.
- **A highlight must mirror its sheet.** An overlay is only meaningful over the base render it was
  framed for, so the sheet, layout, and symbols in a HighlightSheet call must match the GetSheet
  it overlays.
- **The board is a sheet.** A `.kicad_pcb` renders as a synthetic "board" sheet, which is why
  navigation, deep links, and both highlight paths needed no board-specific client plumbing.
- **Diff is all-or-nothing.** If either side fails to load, the call fails. There is no partial
  diff.
- **Query evaluates on the server over netlist facts.** RunQuery loads the design, builds the
  model, and runs the same evaluator the CLI runs. Serve wires no parameter directory and
  datasheet data is deployment-bound, so a query over the datasheet parameter relation returns no
  rows here. The evaluator is dependency-free Go, so a later revision could evaluate it in the
  browser instead.
- **A design is named, a checklist is sent.** CreateReview carries the review manifest as a value
  while the design stays a mount-relative ref, and the split is deliberate. A design is megabytes,
  needs a reader chosen by extension, and is re-requested across many calls, so re-sending it every
  time would be absurd. A checklist is a small declaration the caller already holds, and a service
  that took a path for it would need a filesystem to do its job. GetReviewManifest is the bridge for
  a client that holds a ref and no filesystem: it reads and validates once, and the client sends the
  value it got back. The CLI skips it, because reading the file the user named is its own job.
- **Reviews are the one resource; everything else is a verb.** A review RUN outlives the call that
  made it, so it has a name and the four standard methods, with paging and filtering following AIP.
  Every other rpc here is a pure function of files on disk, so its arguments are its whole identity
  and a resource name would be ceremony. That split is CONSTRAINTS C23 and is deliberate rather
  than drift.
- **Stored runs need a volume.** `agni serve --review-store <dir>` names a WRITABLE directory,
  separate from the read-only design mounts, so persisting runs never turns a mount into a write
  surface. In a container it is a mounted volume. Without the flag the four review resource methods
  answer with a failed-precondition naming it, rather than running the checks and dropping the
  result. Runs stored there are visible to every client of the server; `agni serve` has no
  authentication yet.
- **A stored run embeds the checklist it scored.** The document carries a manifest SNAPSHOT, not
  just the manifest's name. A checklist is an editable file, so a name would resolve to whatever it
  says today, and last quarter's review would re-render against this quarter's questions with its
  outcomes intact underneath. That is the failure the snapshot exists to prevent.

## The render contract

There are three render surfaces per sheet, from one geometry source.

- **PACKED** is the columnar form: one static integer vertex buffer uploaded once, with primitive
  records keyed by reference designator, net, and pin so that selection and highlighting are index
  joins. Text draws in a DOM overlay, since GL draws no glyphs.
- **SVG** is the verification backend and the default mode, with full text fidelity everywhere.
- **NATIVE** is the format's own tool as a golden reference, per-format and opt-in.

The highlight layer is decoupled from the base render. A highlight spec names components, nets,
and pins, plus a style and a shape (outline, bounding rectangle, or bounding circle, per entity),
and it projects to whichever backend is showing. The WebGL client resolves specs locally against
the keys it already holds, with no round-trip, and the Go and TypeScript resolvers share a
twinned test fixture so their semantics provably match. The SVG path fetches the overlay document
and stacks it over the same frame.

## The client

- **The presenter is a humble object.** It owns the semantic loop, open a file, get its sheets,
  render, run checks, apply highlights, and pushes state snapshots into one typed sink of view
  interfaces. It calls views and clients, never the DOM or a UI framework directly, so it tests
  with plain mocks.
- **The UI is islands.** Leaf components render the pushed state and emit intents back up. Panels
  live in a docking shell, and a saved layout is stamped with the panel registry at save time, so
  a newly registered panel appears without a migration.
- **The review panel derives its own tally.** A run's document carries per-item outcomes, not a
  tally, so the browser counts them itself with the same rules `review.Report.Tally()` applies. The
  two implementations are checked against one committed fixture
  (`core/review/testdata/tally_twin.json`), read by a Go test and a TypeScript test, because the
  number they must agree on is `covered`, and a client that bucketed a verdict differently would
  report a checklist as answered when nobody had answered it.
- **URLs are deep-linkable.** The path is `/files/<mount>/<path>`, with query parameters for the
  sheet, the render mode, the layout, and symbol paths. A directory is the same path with a
  trailing slash. The presenter reports location changes and the composition root reflects them
  into the browser URL.
- **The diff view** is side-by-side synced panes, a changes panel with click-to-locate, and an
  overlay (union) mode gated by an alignment check. Netlist-only formats, whose auto-layout node
  positions shift between revisions, refuse the overlay by design, while faithful-geometry
  revisions pass.

## Where the presenter runs

The presenter pattern wants presenter logic behind a contract with the view. The shipped viewer
satisfies that with an in-process TypeScript presenter speaking Connect to the server, which is
the "per-surface runtime" choice for a lower-frequency surface. Browsing a schematic or board,
viewing a diff, and hovering or selecting are low-frequency semantic events that tolerate a
network hop. Per-frame interaction such as pan, zoom, and hit-testing never crosses the wire,
because it is view-local.

A WebAssembly-compiled Go presenter that reuses the diff and IR logic in the browser remains the
option for an offline or zero-server viewer. Nothing in the wire contract assumes the presenter's
location, which is the property that keeps that swap possible.

Keeping the WebAssembly boundary cheap is why the contract is shaped this way. Go compiled to
WebAssembly has higher per-call boundary overhead than a C or Rust equivalent, so when the
presenter does run in the browser the boundary carries meaning, not pixels. Static geometry is
uploaded to the GPU once, only a small dynamic overlay for highlight, selection, and cursor
crosses per frame, input is batched to one intent per animation frame, and the camera transform
is applied in TypeScript and the GPU so dragging stays smooth even if a presenter frame lags. At
60 frames per second this keeps interop well under a millisecond, and the real performance ceiling
is geometry volume and rendering, not the boundary.

## Serving pieces worth knowing

- The web bundle is a build artifact, not committed. The full test target builds it and asserts
  exactly one framework core is present. A stale bundle is the classic "my change does nothing"
  failure.
- `--theme` selects the render palette on the server for both renderers, since the packed payload
  carries the group colors, so WebGL and SVG stay color-identical.
- Board layer visibility is client-side in both renderers, through CSS strata and draw-loop skips,
  never a render parameter.

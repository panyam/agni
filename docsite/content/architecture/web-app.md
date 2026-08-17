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

## The pages

Four server-rendered shells, each with its own bundle, so a page downloads what it uses and no more.

| URL | Page | Bundle | What it is |
|---|---|---|---|
| `/` | `LandingPage` | `landing.js` | Where am I going: the destinations, this browser's recents, and the designs the server's projects declare. Also the catch-all, so an unmatched URL offers choices instead of 404ing |
| `/designs/<mount>/<dir>/` | `BrowsePage` | `browse.js` | The design tree plus a read-only first-sheet preview |
| `/designs/<mount>/<path>/view` | `ViewerPage` | `app.js` | The work page: WebGL canvas, checks, query, diff |
| `/datasheets/files/<mount>/<path>` | `DatasheetsPage` | `datasheets.js` | The extraction workbench, its own tree included |

The `/designs/` space holds two pages behind one pattern, split by the trailing `/view`, because a
`ServeMux` pattern's `{path...}` wildcard must be its last segment. `/files/` is the retired
pre-WS9-049 space and permanently redirects.

**SIX edits for a new page**, and three of them fail quietly. The template (`web/templates/X.html`,
which must define BOTH its `Body` block and a `{{ define "X" }}{{ template "BasePage" . }}{{ end }}`,
or the render errors with `"X" is undefined`), its entry point (`web/src/x.ts`), the bundle in
`web/build.mjs`, that bundle's line in `web/.gitignore` (or the build artifact rides into a commit),
the route in `cmd/agni/webpage.go`'s `registerPages`, and a boot test. The script tag goes in an
`AppScript` block, not loose in `Body`.

**Every page has a composition root, so every page gets a boot test** (C11's Verify names them). It
boots the real entry point against the real template under jsdom and fails on a hole nothing mounts
or a client nothing passes. That is not a viewer-specific concern: the browse page went untested for
months and the workbench shipped a deep link that recorded nothing, both in the window when
`composition.test.ts` was the only one.

Both trees are the same shape over one listing API: each declares what it opens (`web/src/treeprune.ts`),
hides rows of other kinds, and shows how many mounts the server pruned for it. A mount of PDFs is
invisible to the viewer and is the whole sidebar on the workbench.

Recents are per-user browser state (`web/src/recents.ts`, localStorage), written where a design or
datasheet is OPENED rather than previewed. They are not server state: there is no user identity to
key a list to, so one visitor's recents would be everyone's, and the project and design resources are
read-only on purpose ([C23](https://github.com/panyam/agni/blob/main/CONSTRAINTS.md)). The landing
page's other list is the opposite kind of thing, declared in `project.yaml` and `design.yaml` and
shared by everyone the server serves.

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
| Workspace | ListMounts | the mount names a tree roots on; `opens` drops the ones holding nothing that client can open and returns how many, so the sidebar can account for a mount an operator configured and cannot find |
| Workspace | ListDir | one directory level, each file labeled with its reader `format` and the `kind` of client that opens it (design, datasheet, or neither); `opens` declares what the caller can open, which drops folders with none of it anywhere beneath them (a bounded server-side walk, since one level of listing cannot see that far) and is what lets the two trees prune the same mounts to opposite answers |
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
| Check | GetNamingConvention | resolve a stored convention config into a value an OverlayConfig carries, parsed and validated |
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
  while the design stays an artifact URI, and the split is deliberate. A design is megabytes,
  needs a reader chosen by extension, and is re-requested across many calls, so re-sending it every
  time would be absurd. A checklist is a small declaration the caller already holds, and a service
  that took a path for it would need a filesystem to do its job. GetReviewManifest is the bridge for
  a client that holds a URI and no filesystem: it reads and validates once, and the client sends the
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
- **The viewer can ask under its own vocabulary.** A naming convention is picked from the mount,
  resolved server-side into a value, and carried on every rule-running request as an `OverlayConfig`.
  It REPLACES the server's `--conventions` default for that request rather than adding to it, so the
  top bar names which vocabulary produced the answers on screen. That indicator is load-bearing
  rather than decorative: replacement can stop a rule running, a rule that stops running produces no
  findings, and in a findings list that is indistinguishable from a design that got fixed.
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
- **The composition root is tested, because everything else being tested is what hid its bugs.**
  Every other web test constructs its subject with collaborators supplied. `main.ts` is the one
  thing nothing constructs: it resolves each island hole, builds each client, and assembles the
  presenter. Twice that gap shipped a feature that was green in CI and inert in the browser, once
  because a client was never passed and once because a view was never wired. `web/src/composition.test.ts`
  boots the real entry point under jsdom against the real `ViewerPage.html`, replays a design URL
  through the restore loop with stubbed transport, and asserts three things: every island hole the
  page declares gets mounted, every port `ViewSink` declares gets a view, and every client gets used.
  It asserts wiring only. Nothing rendered is checked, because jsdom does not render; the flows that
  need a real browser are tracked in agni issue 136.
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

**Every viewport navigates the same way, from one definition.** `web/src/panzoom.ts` holds the wheel
curve and the cursor-anchored zoom math, and the three viewports that have a camera all take it from
there: the WebGL schematic canvas, the SVG reference render, and the datasheet workbench. The wheel
zooms toward the cursor and a drag pans, so learning one viewport teaches all of them. Two things
follow from having it in one file. The exponential curve makes zoom scale-free, so an overshoot and
an equal correction land exactly where they started at any zoom level. And the choice itself is
revisitable: the datasheet workbench trades the PDF-reader convention (wheel scrolls, ctrl+wheel
zooms) for consistency with the schematic viewers, and reversing that is an edit to panzoom.ts's
callers rather than a hunt through three viewers.

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

## A library that cannot load is a component that cannot be tested

The datasheet workbench (`regionview.tsx`, `transcribe.tsx`) went untested for a long time, and the
reason was one import. `pdfrender.ts` sets pdf.js's worker options and pulls in its canvas module at
LOAD time, and that module reaches for `DOMMatrix`, which jsdom does not have. Any file importing it
therefore throws before a single test runs, whatever the test intended to assert. An 885-line
component with no component test looked like a testing gap; it was an import graph.

The fix is the split now in place, and it generalizes to any browser-only library:

- `pdfsource.ts` holds the PORT (`PdfSource`, `RenderedPage`) and imports nothing at runtime. Naming
  a pdf.js TYPE is free, because `import type` is erased.
- `pdfrender.ts` holds the implementation and exports `realPdfSource`.
- `regionview.tsx` names only the port, and takes it as a required parameter. **A default would
  reintroduce the problem**, since a default value is a runtime import of the implementation.
- `datasheets.ts`, the composition root, is the single place pdf.js enters the app.

So a test supplies a stub page and renders the real workbench: `regionview.test.tsx` covers the
non-passive wheel listener, the live-transform / settled-rasterize split, and fit-on-first-page.
`transcribe.test.tsx` needed no seam at all, only a stub handlers object, and found a swallowed
keystroke on its first run (a signal was set before the event value was read, so Solid wrote the
empty derived id back into the field mid-handler).

## Wiring a new panel

**FOUR edits for a new viewer panel**, and the last one is the one everybody forgets. The island
(`web/src/<panel>.tsx`), its hole in `web/templates/ViewerPage.html` (`data-component="..."`), its
field on `ViewSink` in `web/src/viewer.ts`, and its construction plus wiring in `web/src/main.ts`.

`main.ts` is the composition root and nothing else constructs it, so a missed fourth edit is invisible
to every other test: the presenter's view ports are OPTIONAL by design (an embedding host may leave a
panel out, see `build/overlay.md`), which means an unwired port is a silent no-op rather than a type
error. That has shipped a green-CI, broken-in-the-browser feature twice, once with a client never
passed and once with a view never wired.

`web/src/composition.test.ts` now boots the real `main.ts` under jsdom against the real page and fails
on any of those omissions, so let the test tell you what you forgot. Read its header comment before
changing the wiring; `docsite/content/architecture/web-app.md` has the rationale.

**A test fixture that is a PLAIN OBJECT LITERAL standing in for a proto message is invisible to
`pnpm run typecheck`.** Nesting `Project`'s config fields under `config` left
`projectpresenter.test.ts` structurally wrong and the typecheck green; only the runtime assertion
caught it. So after a proto reshape, `pnpm run typecheck` passing is NOT evidence the web side is
done — run `make testall`. Prefer `create(SomeSchema, {...})` in a new fixture, which does get
checked.

**A `oneof` on the wire is a FIELD NAME, not the `{case, value}` pair the client decodes it into.**
A boot test that stubs `fetch` writes the SERVER's JSON, so `GetSheet` returns `{"svg": "<svg…>"}`.
Writing `{content: {case: "svg", value: "<svg…>"}}` — the shape the TypeScript client hands you after
decoding — yields an empty document and a blank stage, and the test then fails for a reason unrelated
to the wiring it was written for. `composition.test.ts` carried that wrong shape harmlessly for
months, because it asserts only that a call was MADE; `browse.test.ts` asserts what came back, so it
noticed on its first run.

**A JSX expression that reads only PLAIN OBJECT properties never re-runs.** Solid wraps an attribute
or child expression in an effect over the signals it reads, so `class={p.pinRefs.includes(x) ? "on" :
""}` — where `p` came from a list and is not itself reactive — subscribes to NOTHING and renders once.
The data changes, the DOM does not. Read through the accessor instead (`props.spec().parameters.some(
...)`), which tracks. This shipped an inert set of buttons with every unit test green, because the
helpers were correct and only the subscription was missing; a component test is the only thing that
sees it, and `transcribe.tsx` still has none (OUT_OF_SCOPE).

**Never `window.confirm` / `alert` / `prompt` in a panel.** A native dialog blocks the page, which
blocks browser automation outright: the screenshot and drive-the-app flows stop responding with no
error. Use an inline two-step (the `deletePackage` confirm in `transcribe.tsx`), which is also better
UX, since it can name what is about to be lost rather than asking a generic "are you sure".

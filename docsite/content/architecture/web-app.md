---
title: "Web app and presenter"
description: "The browser viewer and visual diff over agni serve, the mount model, the render contract, and the presenter pattern behind the client."
---

The browser viewer and visual diff run over `agni serve`. This page covers the presenter pattern the
client is built on, how designs reach the browser through the mount model, the render and highlight
contracts both renderers share, and how that pattern is realized in the shipped viewer. Three pages
carry the rest of the web tier: the wire contract is on
[Web service contract](../web-services/), how a reader names and inspects an entity is on
[Picking and querying](../web-picking/), and the wiring and traps of the client itself are on
[Working in the web client](../web-client/).

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

{{ includeFile "figures/presenter-tiers.svg" }}

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
and the Connect handlers are glue over them. Which service owns which RPC is on
[Web service contract](../web-services/).

{{ includeFile "figures/web-request-path.svg" }}

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

`agni serve --mount name=path`, repeatable. Designs enter only through mounts, each
exposing one folder to the file browser under a stable name. Every file-addressing request in the
API is a pair of a mount name and a mount-relative path. The server resolves it inside the mount
and rejects any path that escapes it. The browser never sees or sends an absolute filesystem
path.

File I/O stays at this edge. The loader opens files and hands readers an `io.Reader`, including
the multi-file cases such as a KiCad hierarchy or xschem and gEDA symbol resolution, through
openers the reader never constructs itself.

## The render contract

There are three render surfaces per sheet, from one geometry source.

{{ includeFile "figures/render-surfaces.svg" }}

- **PACKED** is the columnar form: one static integer vertex buffer uploaded once, with primitive
  records keyed by {{ explainable "reference-designator" }}, net, and pin so that selection and highlighting are index
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
  overlay (union) mode gated by an alignment check. {{ explainable "netlist" "Netlist" }}-only formats, whose auto-layout node
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
location, so the swap stays possible.

Keeping that boundary cheap is why the contract is shaped this way: it carries meaning rather than
pixels, so static geometry is uploaded to the GPU once and only a small dynamic overlay crosses per
frame. The [stack](../stack/) page has the full argument.

## Serving pieces worth knowing

- The web bundle is a build artifact, not committed. The full test target builds it and asserts
  exactly one framework core is present. A stale bundle is the classic "my change does nothing"
  failure.
- `--theme` selects the render palette on the server for both renderers, since the packed payload
  carries the group colors, so WebGL and SVG stay color-identical.
- Board layer visibility is client-side in both renderers, through CSS strata and draw-loop skips,
  never a render parameter.

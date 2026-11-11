---
title: "Web app and presenter"
description: "The browser viewer and visual diff over agni serve, the mount model, the render contract, and the presenter pattern behind the client."
---

The browser viewer and visual diff run over `agni serve`. This page covers the presenter pattern
under the client, how designs reach the browser through the mount model, the render and highlight
contracts shared by both renderers, and how the shipped viewer realizes that pattern. Three other
pages cover the rest of the web tier. [Web service contract](../web-services/) has the wire
contract, [Picking and querying](../web-picking/) covers how a reader names and inspects an entity,
and [Working in the web client](../web-client/) has the wiring and traps of the client itself.

The web tier owns no engine logic. It chooses which reader, layout, packer, and rule set to
invoke per file and serves the result. The readers, the [diff](../semantic-diff/), the
[checks](../rules-and-checks/), and the [renderers](../geometry-and-rendering/) are engine
packages, and the CLI drives them too.

## The presenter pattern

The engine covers parse, IR, diff, rules, and later simulation. That logic must run in three
places: a backend that batch-parses and validates large files, the browser viewer, and the CLI
and tests. The presenter pattern was designed for that case, so it is the architecture
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
an HTML-over-the-wire renderer allowed on top of it. If `SetDiffReport(data)` is the primitive and
`SetDiffReportContent(html)` is a renderer on top of it, the frontend can still be swapped.

Canvas commands are semantic by necessity, since a canvas cannot take HTML, so a schematic or PCB
surface receives highlight and overlay commands, not markup. Panels deliberately differ. A
data-heavy panel may take rendered HTML, as one renderer over the same typed state.

## Architecture

The request path runs from the browser through two server layers into the engine.

In the browser, leaf UI components push intents to a TypeScript presenter. The leaf components
are the panels, the file tree and the controls. The presenter drives the canvas's sheet surface,
which draws the packed form in WebGL and the SVG document and its overlay in an SVG host. A sheet view
beside the canvas draws through a surface of its own (`web/src/sheetsurface.ts`). It talks to the server over
Connect using proto JSON.

On the server, Connect handlers receive the call and delegate to transport-neutral services. The
services take loaders in and return proto messages out, with no HTTP knowledge. They call the
`formats` loader and registry to read a file, and the render, diff, and check engine packages to
process it. Server-rendered pages declare holes, and the browser's UI components mount into them,
so there is no client-side HTML skeleton to drift from the server's.

The two server layers are deliberate. The transport-neutral services are the reusable surface,
and the Connect handlers are glue over them. [Web service contract](../web-services/) lists which
service owns which RPC.

{{ includeFile "figures/web-request-path.svg" }}

## The pages

There are five server-rendered shells, each with its own bundle, so a page downloads what it uses and no more.

| URL | Page | Bundle | What it is |
|---|---|---|---|
| `/` | `LandingPage` | `landing.js` | Where am I going: the destinations, this browser's recents, and the designs the server's projects declare. Also the catch-all, so an unmatched URL offers choices instead of 404ing |
| `/designs/<mount>/<dir>/` | `BrowsePage` | `browse.js` | The design tree plus a read-only first-sheet preview |
| `/designs/<mount>/<path>/view` | `ViewerPage` | `app.js` | The work page: the canvas in SVG or WebGL, sheet views, checks, query, diff |
| `/designs/<mount>/<path>/load`, `/designs/local/load/` | `LoadPage` | `load.js` | Reads a design into the engine's store before the viewer opens it, step by step, then opens the viewer at the same address (agni issue 1072). With no design, the browser mount's page takes dropped files and lists the kept drops |

`agnids` serves the extraction workbench, rather than `agni serve` (agni issue 744). The workbench
is `DatasheetsPage` at `/datasheets/files/<mount>/<path>`, with its own bundle `datasheets.js`. It
is built from its own package, `datasheet/web`, in the pnpm workspace beside this `web/`. The two
share only the plain
TS modules in `web-shared/` (URIs, recents, pan and zoom maths, the workbench's URL format, and the
pruned-mount note), and the landing page links the workbench only when `agni serve --datasheets-url`
names where it runs.

The `/designs/` space holds three pages behind one pattern, split by the trailing `/view` or `/load`,
because a `ServeMux` pattern's `{path...}` wildcard must be its last segment. The viewer opens a
design only if the Load page recorded reading it on this engine and build. It sends any other design
to its `/load` with the query whole (`web/src/admit.ts`), so every link into the viewer still reaches
its design. `/files/` is the retired pre-WS9-049 space and permanently redirects.

**A new page takes SIX edits**, and three of them fail quietly. The six are the template
(`web/templates/X.html`), its entry point (`web/src/x.ts`), the bundle in `web/build.mjs`, that
bundle's line in `web/.gitignore`, the route in `cmd/agni/webpage.go`'s `registerPages`, and a boot
test. The template must define BOTH its `Body` block and a `{{ define "X" }}{{ template "BasePage" . }}{{ end }}`,
or the render errors with `"X" is undefined`. Without the `.gitignore` line the build artifact goes
into a commit. The script tag goes in an `AppScript` block, not loose in `Body`.

**Every page has a composition root, so every page gets a boot test.** C11's Verify names three of
the four, and `LandingPage` has only a template-id check in `web/src/landingpanels.test.tsx`. A boot
test boots the real entry point against the real template under jsdom. It fails when nothing mounts
a hole or nothing passes a client. The concern is wider than the viewer. While `composition.test.ts`
was the only boot test, the browse page went untested for months and the workbench shipped a deep
link that recorded nothing.

Both trees use one listing API, and each declares what it opens (`web/src/treeprune.ts`),
hides rows of other kinds, and shows how many mounts the server pruned for it. A mount of PDFs is
invisible to the viewer and is the whole sidebar on the workbench.

Recents are per-user browser state (`web/src/recents.ts`, localStorage), written where a design or
datasheet is OPENED rather than previewed. They are not server state. The server has no user
identity to key a list to, so one visitor's recents would be everyone's, and the project and design
resources are read-only on purpose ([C23](https://github.com/panyam/agni/blob/main/CONSTRAINTS.md)).
The landing page's other list is the opposite kind of thing. `project.yaml` and `design.yaml`
declare it, and everyone the server serves shares it.

## The mount model

`agni serve --mount name=path` adds a mount, and the flag repeats. Designs enter only through mounts, each
exposing one folder to the file browser under a stable name. Every file-addressing request in the
API is a pair of a mount name and a mount-relative path. The server resolves it inside the mount
and rejects any path that escapes it. The browser never sees or sends an absolute filesystem
path.

File I/O happens only at this edge. The loader opens files and hands readers an `io.Reader`. That
includes the multi-file cases, such as a KiCad hierarchy or xschem and gEDA symbol resolution, where
the loader hands over openers and the reader never constructs one itself.

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
twinned test fixture so a divergence fails a test on both sides. The SVG path fetches the overlay document
and stacks it over the same frame.

A bounding shape draws a translucent fill under an opaque stroke in the same color, so the copper
under a part still shows and the part's edge still reads on a dense board (agni issue 1536). Its
least pad and its stroke are fractions of the drawing's smaller span, so a two-pad footprint frames
at about twice its own span and a large part keeps a 10% pad. The Highlight menu's width control
scales the stroke as it scales a net's path.

After a check run the drawing shows nothing until a finding is focused (agni issue 356). A reader
can opt in to every finding at once, the run-wide field, through Highlight › Show every finding,
which is kept in `localStorage`. When the field is on, a focus stacks over it as muted context. The
presenter reads the field through one helper, `field()`, so every path that paints it honours the
choice.

## The client

- **The presenter is a humble object.** It owns the semantic loop (open a file, get its sheets,
  render, run checks, apply highlights) and pushes state snapshots into one typed sink of view
  interfaces. It calls views and clients, never the DOM or a UI framework directly, so it tests
  with plain mocks.
- **The UI is islands.** Leaf components render the pushed state and emit intents back up. Panels
  live in a docking shell, and a saved layout is stamped with the panel registry at save time, so
  a newly registered panel appears without a migration.
- **The review panel derives its own tally.** A run's document carries per-item outcomes, not a
  tally, so the browser counts them itself, applying the same rules as `review.Report.Tally()`. A
  Go test and a TypeScript test both read one committed fixture,
  `core/review/testdata/tally_twin.json`, and check their implementation against it. The two must
  agree on `covered`. A client that bucketed a verdict differently would report a checklist as
  answered when nobody had answered it.
- **URLs are deep-linkable.** The viewer's path is `/designs/<mount>/<path>/view`, with query
  parameters for the sheet, the render mode, the layout, and symbol paths. A directory opens the
  browse page at `/designs/<mount>/<dir>/`. The presenter reports location changes and the composition root reflects them
  into the browser URL.
- **The composition root is tested, because testing everything else hid its bugs.** Every other
  web test constructs its subject with collaborators supplied. Nothing constructs `main.ts`. It
  resolves each island hole, builds each client, and assembles the presenter. Twice that gap shipped
  a feature that was green in CI and inert in the browser, once because a client was never passed
  and once because a view was never wired. `web/src/composition.test.ts` boots the real entry point
  under jsdom against the real `ViewerPage.html`, replays a design URL through the restore loop with
  stubbed transport, and asserts three things: every island hole declared in the page gets mounted,
  every port declared in `ViewSink` gets a view, and every client gets used. It asserts wiring only.
  Nothing rendered is checked, because jsdom does not render; the flows that need a real browser run
  under `make browser-test` (agni issue 323).
- **The diff view** is side-by-side synced panes, a changes panel with click-to-locate, and an
  overlay (union) mode gated by an alignment check. {{ explainable "netlist" "Netlist" }}-only formats
  refuse the overlay by design, because their auto-layout node positions shift between revisions.
  Faithful-geometry revisions pass.

**Every viewport navigates the same way, from one definition.** `web/src/panzoom.ts` holds the wheel
curve and the cursor-anchored zoom math, and the three viewports that have a camera all take it from
there: the WebGL schematic canvas, the SVG reference render, and the datasheet workbench. The wheel
zooms toward the cursor and a drag pans, so learning one viewport teaches all of them. Two things
follow from having it in one file. The exponential curve makes zoom scale-free, so an overshoot and
an equal correction return exactly to where they started at any zoom level. And the choice itself
can be revisited. The datasheet workbench gives up the PDF-reader convention (wheel scrolls,
ctrl+wheel zooms) to match the schematic viewers. Reversing that means editing panzoom.ts's callers
rather than hunting through three viewers.

## Where the presenter runs

The presenter pattern wants presenter logic behind a contract with the view. The shipped viewer
satisfies that with an in-process TypeScript presenter that speaks Connect to the server. That is
the "per-surface runtime" choice for a lower-frequency surface. Browsing a schematic or board,
viewing a diff, and hovering or selecting are low-frequency semantic events that tolerate a
network hop. Per-frame interaction such as pan, zoom, and hit-testing never crosses the wire,
because it is view-local.

An offline or zero-server viewer still has an option. A WebAssembly-compiled Go presenter could
reuse the diff and IR logic in the browser. Nothing in the wire contract assumes the presenter's
location, so the swap is still possible.

The contract carries meaning rather than pixels to keep that boundary cheap, so static geometry is
uploaded to the GPU once and only a small dynamic overlay crosses per frame. The [stack](../stack/)
page has the full argument.

## Serving pieces worth knowing

- The web bundle is a build artifact, not committed. The full test target builds it and asserts
  exactly one framework core is present. A stale bundle is the classic "my change does nothing"
  failure.
- `--theme` selects the render palette on the server for both renderers, since the packed payload
  carries the group colors, so WebGL and SVG draw identical colors.
- Board layer visibility is client-side in both renderers, never a render parameter. The board
  sheet's `GetSheet` answer lists its layers as `BoardLayerRow`s (agni 1351). Each row names the
  SVG elements that draw it (`svg_selector`) and the packed groups (`groups`). So the viewer hides a
  row with one CSS rule or a draw-loop skip, and knows nothing about how a board is drawn. The
  Sheets panel lists the rows under the Board entry, and the viewer keeps a design's hidden rows in
  browser storage. A sheet view beside the canvas lists the same rows over a hidden set of its own,
  with its rules scoped to its own drawing (agni issue 1401).

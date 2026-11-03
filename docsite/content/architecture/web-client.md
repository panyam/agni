---
title: "Working in the web client"
description: "The four edits a new viewer panel takes, and the traps that ship green in CI and broken in the browser."
---

This page collects what bites when you change the web client. It covers the edits a new panel
takes, the framework behaviours that produce an inert UI with every unit test green, and the
failures that only a real browser or a composition-root test can see. The architecture underneath is on
[Web app and presenter](../web-app/). The panels serve an interaction model, and
[Picking and querying](../web-picking/) describes it.

## Wiring a new panel

**A new viewer panel takes FOUR edits**, and everybody forgets the last one. They are the island
(`web/src/<panel>.tsx`), its hole in `web/templates/ViewerPage.html` (`data-component="..."`), its
field on `ViewSink` in `web/src/viewer.ts`, and its construction plus wiring in `web/src/main.ts`.

**A panel that DOCKS costs two more**, and they are not optional. One is an entry in `VIEWER_PANELS`
and the other is a line in `defaultLayout`, both in `web/src/dock.ts`. `dock.test.ts` asserts the
registry by exact list, so it catches a missing entry. Nothing catches a missing line, and the line has
an ordering rule. dockview resolves `referencePanel` against panels that ALREADY EXIST. Suppose a new
panel tabs with a panel that the layout places later. Then boot throws rather than deferring. The trace
panel first failed `composition.test.ts` this way. Add the panel after the one it tabs with.

`main.ts` is the composition root and nothing else constructs it. The presenter's view ports are
OPTIONAL by design, because an embedding host may leave a panel out (see `build/extending.md`). So an
unwired port is a silent no-op rather than a type error, and every other test misses a forgotten fourth
edit. That has shipped a green-CI, broken-in-the-browser feature twice, once with a client never passed
and once with a view never wired.

`web/src/composition.test.ts` now boots the real `main.ts` under jsdom against the real page and fails
on any of those omissions, so let the test tell you what you forgot. Read its header comment before
changing the wiring; `docsite/content/architecture/web-app.md` has the rationale.

## Traps that ship green

**A panel's derived rows must be MEMOIZED, or selecting a row rebuilds the whole panel.** Solid's
`<For>` keys by object REFERENCE, and a derivation like `collapseSorted(props.state().findings)` mints
fresh objects on every call. Written as a plain function it re-runs on ANY state push. A new selection
changes nothing about the rows, but pushing one produced all-new objects. `<For>` matched none of them,
and the entire table was torn down and rebuilt. Rebuilding the rows resets the scroll container,
so clicking a finding halfway down a long list threw the reader back to the top (agni issue 367).

The fix wraps the derivation in `createMemo` over the one input that should rebuild rows. A Solid memo
compares by reference, so a push that leaves that input identical stops at the memo boundary. The
selected row still restyles. The row component reads `selected` itself, and that fine-grained read
recreates nothing. A derivation feeding `<For>` is a memo, and the memo boundary is a correctness
concern here rather than a performance one.

**`LOCATE_REASON_UNSPECIFIED` is not a neutral default.** Its contract is "the entity IS drawn,
expected to highlight", so leaving it on a subject that cannot be located actively tells the viewer to
say nothing. `AnnotateSheets` set a reason for buses alone. So every undrawn NET shipped a value that
asserted the opposite of the truth. Clicking such a finding did nothing and explained nothing. The same
net, reached through a query result cell, explained itself perfectly (agni issue 366). If a zero value
asserts a positive claim, every producer has to set it, or it lies by omission. `NO_GEOMETRY` makes a
narrower claim of the same kind. It says the schematic was read and leaves the entity out. When the
schematic failed to read, every subject reads `SCHEMATIC_UNREADABLE` instead. A broken companion once
made a whole design look drawn on no sheet, and nothing said the file had failed (agni issue 1230).

**`pnpm run typecheck` cannot see a test fixture that is a PLAIN OBJECT LITERAL standing in for a
proto message.** Nesting `Project`'s config fields under `config` left
`projectpresenter.test.ts` structurally wrong and the typecheck green; only the runtime assertion
caught it. So after a proto reshape, `pnpm run typecheck` passing is NOT evidence the web side is
done. Run `make testall`. Prefer `create(SomeSchema, {...})` in a new fixture, which does get
checked.

**A `oneof` on the wire is a FIELD NAME, not the `{case, value}` pair the client decodes it into.**
A boot test that stubs `fetch` writes the SERVER's JSON, so `GetSheet` returns `{"svg": "<svg…>"}`.
After decoding, the TypeScript client hands you `{content: {case: "svg", value: "<svg…>"}}`. A stub
that writes that shape yields an empty document and a blank stage. The test then fails for a reason
unrelated to the wiring it was written for. `composition.test.ts` carried that wrong shape harmlessly
for months, because it asserts only that a call was MADE. `browse.test.ts` asserts what came back, so
it noticed on its first run.

**A JSX expression that reads only PLAIN OBJECT properties never re-runs.** Solid wraps an attribute
or child expression in an effect over the signals it reads. Suppose `p` came from a list and is not
itself reactive. Then `class={p.pinRefs.includes(x) ? "on" : ""}` subscribes to NOTHING and renders
once. The data changes, the DOM does not. Read through the accessor instead
(`props.spec().parameters.some(...)`), which tracks. This shipped an inert set of buttons with every
unit test green. The helpers were correct, and only the subscription was missing. Only a component test
sees it, and `transcribe.test.tsx` covers the pin and relation editors but not the binding chips.

**Never `window.confirm` / `alert` / `prompt` in a panel.** A native dialog blocks the page, which
blocks browser automation outright, so the screenshot and drive-the-app flows stop responding with no
error. Use an inline two-step (the `deletePackage` confirm in `transcribe.tsx`), which is also better
UX, since it can name what is about to be lost rather than asking a generic "are you sure".

## A library that cannot load is a component that cannot be tested

One import kept the datasheet workbench untested for a long time. The workbench is `regionview.tsx`
and `transcribe.tsx`, in `datasheet/web/src` since agni issue 744. `pdfrender.ts` sets pdf.js's worker
options and pulls in its canvas module at LOAD time. That module reaches for `DOMMatrix`, which jsdom
does not have. So any file importing it throws before a single test runs, whatever the test intended to
assert. An 885-line component with no component test looked like a testing gap, and the cause was the
import graph.

The split below fixes it, and the same split works for any browser-only library.

- `pdfsource.ts` holds the PORT (`PdfSource`, `RenderedPage`) and imports nothing at runtime. Naming
  a pdf.js TYPE is free, because `import type` is erased.
- `pdfrender.ts` holds the implementation and exports `realPdfSource`.
- `regionview.tsx` names only the port, and takes it as a required parameter. **A default would
  reintroduce the problem**, since a default value is a runtime import of the implementation.
- `datasheets.ts`, the composition root, is the single place pdf.js enters the app.

So a test supplies a stub page and renders the real workbench.

<details>
<summary>What the tests that became possible then caught</summary>

`regionview.test.tsx` covers the non-passive wheel listener, the live-transform / settled-rasterize
split, and fit-on-first-page. `transcribe.test.tsx` needed no port at all, only a stub handlers
object, and found a swallowed keystroke on its first run (a signal was set before the event value
was read, so Solid wrote the empty derived id back into the field mid-handler).

</details>

## A panel that works on click can still be broken on arrival

Two bugs shipped past a green unit suite and were found only by driving a real browser, both on the
arrive-by-link path rather than the click-a-row path.

A cold load has empty caches. So a URL naming a verdict opened on "Press Run checks" and resolved
nothing. The CLI-to-viewer hop failed at the one moment it is used. Separately, the presenter set the
focused id without pushing state. The canvas drew the proof, but the panel remained on the other table.
The sentence explaining the proof remained hidden behind a toggle, and the reader has no reason to know
about that toggle.

Neither is a rendering bug, so `make browser-test` is not for either one. **A panel test mounts state
directly, so it only ever exercises the state a working presenter would push.** A deep link is a
different entry point into the same panel, arriving with caches cold and view state unset. Test the
arrival, not just the interaction.

## Two caches for one answer will drift

The presenter caches check results per rule so toggling a rule that already ran costs no round-trip.
When the considered set arrived it got a second cache beside the first, filled from the same response,
and the two drifted apart. Findings were invalidated in two places and verdicts in one. So after a
change to the naming vocabulary, the screen still showed a considered set computed under the old one.

That matters more for verdicts than for findings. **A verdict is keyed by rule NAME, and rule names
change when the convention changes.** The server's naming rules disappear, and the request's appear
under a different namespace. So a surviving verdict can name a rule that no longer exists. It can
answer for a subject that nothing re-examined. That verdict claims coverage from a run that never
happened.

The first fix was to clear them together and assert it, rather than leaving the mirror between two
caches to hold by inspection. Agni issue 390 then removed the mirror. The server keeps each rule's
results on the design's cached model, so a rule it has already run costs a round trip and no
evaluation. The presenter holds one run. That run is the engine's `CheckDesignResponse`, with the
rules it answers. The findings, the considered set, the skipped rules, a saved report and the Tables
panel all read that run. A new design or a convention change drops it in one place.

The presenter still keeps that one answer rather than asking the server again on every Run. Run asks
only for the selected rules the run lacks and folds the reply in. Asking for the whole selection
each time was measured on Jetson at 0.3 to 0.8 s in the browser for an answer already on screen,
where keeping it costs nothing. The rule is one held answer per question, never a second copy of it
beside the first.

## The document half and the interaction half

The presenter is two objects with a one-way dependency (agni issue 390). `DocumentStore` lives in
`web/src/document.ts`. It holds what is true of the open design and of its check configuration,
whatever the reader is doing. That covers the design's identity and sheets, the layouts it offers, the
rule catalog and the active ruleset, the held check run, the expectation sidecar and the naming
convention. The store owns the two requests that fill those, `ListRules` and `CheckDesign`.
`ViewerPresenter` holds what the reader is doing (selection, focus, highlights, camera, panel tabs, the
busy state). It reads the store through `presenter.doc` and pushes views.

**The dependency runs one way.** Interaction reads the document, and the document never reads
interaction. Imagine the store asked to "recompute the verdicts for the selected subject". That would
make a cycle across what may become a WebAssembly boundary, the expensive kind. `document.test.ts`
reads `document.ts` and fails if it imports an interaction module or names interaction state. Its
positive control checks that the same patterns match `viewer.ts`. A new field belongs in the store only
if it holds with no reader present. A field that changes with a click belongs in the presenter.

## The live page contract

A viewer page is reached three ways: a link (`ViewerLocation` in the URL, applied once by `restore`),
a click in a panel, and a call from a host through `ViewerPageAPI` (`web/src/pageapi.ts`, agni issue
1150). All three reach the same place, because they share the code that picks where the page goes.
`sheetToOpen` in `viewer.ts` picks the sheet for a finding, a review finding and a verdict. A passing
verdict has no finding to borrow sheets from, so a verdict carries its own sheets on the wire
(`Verdict.sheets`, filled by `AnnotateRunSheets` in `service/sheets.go`). `restore` and
`applyLocation` both end in `arrive`, which asks what a location carries beyond the design.
`web/src/navparity.test.ts` runs each path to the same finding and verdict, and names any path that
ends elsewhere. A new way to move the page goes through these, or it gets a row in that test.

`ViewerPageAPI` is the TypeScript face of `ViewerPageService` (`protos/agni/v1/viewer/page.proto`),
and the page answers that service itself. `pageservice.ts` dispatches a method name and a protojson
body over it. A transport only carries that pair. Today the transport is postMessage from a parent
frame (`embed.ts`, with the origin allowlist from `agni serve --embed-origin`). Later it may be a
server-held session or an MCP app host. So a transport adds no behaviour, and a call behaves the same
over any of them. The page takes one request from any origin, `ApplyLocation` carrying only a `url`
(`isLinkOnly`). A saved report sends it, and its reply carries no location.
[Questions, links and the live page](../viewer-control/) shows how the three layers meet. The
contract differs from a panel click in one way on purpose. A click on the focused finding clears it,
and a call asks for a state, so selecting the focused finding keeps it. How a parent page uses it is
in [Driving an embedded viewer](../../build/embedding-the-viewer/).

## App events and the review tour

Once the viewer has done something for the reader, it sends a named event to the page's `EventBus`
(`web/src/viewerevents.ts`). Examples are a checklist or the checks having run, a finding or a query
cell located, a set opened, a question asked, an area selected or a report saved. The presenter emits
through its `events` port and the query panel and `main.ts` through `viewerEmitter`. `ViewerEvents`
types every payload, and `viewerEventNames` must list every event or the file fails to compile.

The review tour (`web/src/reviewtour.ts`, `?tour=review`) is tsappkit's `Tour` over those events
(panyam/goapplib#128). Each step dims the page, leaves the controls it needs reachable, and advances
on the viewer's event for the action, never on a click it infers. An action the viewer reports no
event for cannot be a step, so a new step starts with a new event here. `reviewtour.test.ts` fails
on a step whose event nothing emits. `?tourpace=host` holds each step until a driver releases it.
The driver uses a handle the page leaves, `__agniTour`. The browser-review walk takes the tour this
way. In a headed run the reader does each step while the walk checks it. Otherwise the walk acts
through the holes, so a hole that misses its control fails the walk in the gate.

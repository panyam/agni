---
title: "Working in the web client"
description: "The four edits a new viewer panel takes, and the traps that ship green in CI and broken in the browser."
---

This page is what bites when you change the web client. It covers the edits a new panel takes, the
framework behaviours that produce an inert UI with every unit test green, and the failures that only
a real browser or a composition-root test can see. The architecture underneath is on
[Web app and presenter](../web-app/), and the interaction model the panels serve is on
[Picking and querying](../web-picking/).

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

**A panel's derived rows must be MEMOIZED, or selecting a row rebuilds the whole panel.** Solid's
`<For>` keys by object REFERENCE, and a derivation like `collapseSorted(props.state().findings)` mints
fresh objects on every call. Written as a plain function it re-runs on ANY state push, so pushing a new
selection (which changes nothing about the rows) produced all-new objects, `<For>` matched none of
them, and the entire table was torn down and rebuilt. Rebuilding the rows resets the scroll container,
so clicking a finding halfway down a long list threw the reader back to the top (agni issue 367).

`createMemo` over the one input that should rebuild rows fixes it, because a Solid memo compares by
reference and a push that leaves that input identical stops at the memo boundary. The selected row
still restyles, since the row component reads `selected` itself and that is a fine-grained read rather
than a reason to recreate anything. A derivation feeding `<For>` is a memo, and the memo boundary is a
correctness concern here rather than a performance one.

**`LOCATE_REASON_UNSPECIFIED` is not a neutral default.** Its contract is "the entity IS drawn,
expected to highlight", so leaving it on a subject that cannot be located actively tells the viewer to
say nothing. `AnnotateSheets` set a reason for buses alone, so every undrawn NET shipped a value
asserting the opposite of the truth and clicking such a finding did nothing and explained nothing,
while the same net reached through a query result cell explained itself perfectly (agni issue 366). A
zero value that asserts a positive claim needs every producer to set it, or it lies by omission.

**A test fixture that is a PLAIN OBJECT LITERAL standing in for a proto message is invisible to
`pnpm run typecheck`.** Nesting `Project`'s config fields under `config` left
`projectpresenter.test.ts` structurally wrong and the typecheck green; only the runtime assertion
caught it. So after a proto reshape, `pnpm run typecheck` passing is NOT evidence the web side is
done. Run `make testall`. Prefer `create(SomeSchema, {...})` in a new fixture, which does get
checked.

**A `oneof` on the wire is a FIELD NAME, not the `{case, value}` pair the client decodes it into.**
A boot test that stubs `fetch` writes the SERVER's JSON, so `GetSheet` returns `{"svg": "<svg…>"}`.
Writing `{content: {case: "svg", value: "<svg…>"}}`, the shape the TypeScript client hands you after
decoding, yields an empty document and a blank stage, and the test then fails for a reason unrelated
to the wiring it was written for. `composition.test.ts` carried that wrong shape harmlessly for
months, because it asserts only that a call was MADE; `browse.test.ts` asserts what came back, so it
noticed on its first run.

**A JSX expression that reads only PLAIN OBJECT properties never re-runs.** Solid wraps an attribute
or child expression in an effect over the signals it reads, so `class={p.pinRefs.includes(x) ? "on" :
""}`, where `p` came from a list and is not itself reactive, subscribes to NOTHING and renders once.
The data changes, the DOM does not. Read through the accessor instead (`props.spec().parameters.some(
...)`), which tracks. This shipped an inert set of buttons with every unit test green, because the
helpers were correct and only the subscription was missing; a component test is the only thing that
sees it, and `transcribe.tsx` still has none (OUT_OF_SCOPE).

**Never `window.confirm` / `alert` / `prompt` in a panel.** A native dialog blocks the page, which
blocks browser automation outright: the screenshot and drive-the-app flows stop responding with no
error. Use an inline two-step (the `deletePackage` confirm in `transcribe.tsx`), which is also better
UX, since it can name what is about to be lost rather than asking a generic "are you sure".

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

So a test supplies a stub page and renders the real workbench.

<details>
<summary>What the tests that became possible then caught</summary>

`regionview.test.tsx` covers the non-passive wheel listener, the live-transform / settled-rasterize
split, and fit-on-first-page. `transcribe.test.tsx` needed no seam at all, only a stub handlers
object, and found a swallowed keystroke on its first run (a signal was set before the event value
was read, so Solid wrote the empty derived id back into the field mid-handler).

</details>

## A panel that works on click can still be broken on arrival

Two bugs shipped past a green unit suite and were found only by driving a real browser, both on the
arrive-by-link path rather than the click-a-row path.

A cold load has empty caches, so a URL naming a verdict landed on "Press Run checks" and resolved
nothing, which is the CLI-to-viewer hop failing at the one moment it is used. And the presenter set
the focused id without pushing state, so the canvas drew the proof while the panel stayed on the other
table and the sentence explaining it stayed hidden behind a toggle the reader has no reason to know
about.

Neither is a rendering bug, so neither is what `make browser-test` is for. **The general shape is that
a panel test mounts state directly and therefore only ever exercises the state the presenter would
push if it were working.** A deep link is a different entry point into the same panel: it arrives with
caches cold and view state unset. Test the arrival, not just the interaction.

## Two caches for one answer will drift

The presenter caches check results per rule so toggling a rule that already ran costs no round-trip.
When the considered set arrived it got a second cache beside the first, filled from the same response,
and the two did not stay in step: findings were invalidated in two places and verdicts in one, so
changing the naming vocabulary left a considered set computed under the old one on screen.

The sharp part is why that matters more for verdicts than for findings. **A verdict is keyed by rule
NAME, and a convention change is exactly when rule names change**: the server's naming rules disappear
and the request's appear under a different namespace. A surviving verdict can therefore name a rule
that no longer exists and answer for a subject nothing re-examined, which is a coverage claim about a
run that never happened.

The fix was to clear them together and assert it, rather than leaving the mirror between two caches to
hold by inspection. The better fix is for the frontend not to hold answers at all, which needs the
server to cache first or toggling a rule re-runs every selected rule. That is agni issue 390, along
with the presenter split it enables.

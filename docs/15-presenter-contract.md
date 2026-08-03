# Presenter-contract architecture (our adoption)

See [README](README.md). Enforceable rules in [/CONSTRAINTS.md](../CONSTRAINTS.md).
Companion to the stack decision in [14-stack-and-architecture](14-stack-and-architecture.md).

**Source:** the app-agnostic presenter-contract design thesis (PRESENTER_CONTRACT_THESIS,
from the goapplib project; the shared `@panyam/tsappkit` + `@panyam/tsappkit-solid`
packages are its reference implementation). This doc records how we adopt it for the EDA
tooling platform.

## Why it fits us

Our engine (parse, IR, diff, rules, later simulation) is logic that must run in three
places: the backend (batch parse/validate/simulate large files), the browser (the
interactive viewer), and the CLI/tests. That is Principle 1 verbatim, so this is our
architecture, not an analogy. The shipped viewer keeps the engine server-side behind the
Connect web API and runs a thin TS presenter in the browser; compiling the same Go core
to WASM for an in-browser engine stays the option, not the requirement.

## Principle mapping

1. **One logic tier, three runtimes.** Engine packages (`edif`, `diff`, `check`, `ir`,
   later `sim`) are runtime-agnostic: no `syscall/js`, no DB, take `io.Reader`/bytes,
   not paths. Drivers: `cmd/*` (CLI), `web/server` (server), `cmd/wasm` (browser).
   Demo one's CLI is the "test/CLI driver."
   - **Our refinement (heavy ops):** choose per operation where to run. Heavy ops
     (parse a 62MB schematic, simulation, WCCA) run server-side; the CLI/tests run them
     in-process. For the browser split, see "the presenter is mandatory; its runtime is
     per-surface" below.

2. **Duplex contract, both directions semantic**, expressed as typed view interfaces:
   two proto services when the contract crosses a WASM/network boundary (like lilbattle
   `GameViewPresenter` + `GameViewerPage`), or a plain typed TS interface when the
   presenter is in-process. Not an event bus, not a shared store (C3):
   - Intents up: `ComponentSelected(refDes)`, `NetHovered(netId)`,
     `DiffFilterChanged(...)`, `ViewportChanged(bounds)`, `CheckSelected(id)`.
   - Commands down: `HighlightNet(netId)`, `SetSelection(...)`, `ShowDiffOverlay(data)`,
     `SetDiffReport(data)` / `SetDiffReportContent(html)`, `SetChecks(...)`.
   - **Rule:** intents are semantic and DOM-free. Picking/hit-testing is view-local
     (the view owns the spatial index of what it drew and turns a cursor position into
     `NetHovered`), and the camera is view-local. The presenter never sees pixels or
     DOM events.

3. **Command altitude.** Canvas (schematic/PCB) commands are semantic (a canvas can't
   take HTML): `HighlightNet`, `ShowDiffOverlay`. Panels (diff report, checks list,
   BOM, netlist tree) are the deliberate fork.

4. **Semantic command is the primitive; HTML-over-the-wire is a renderer on top.**
   Keep `SetDiffReport(data)` primitive; `SetDiffReportContent(html)` is one renderer.
   Preserves swap-the-frontend.

5. **Local-first / reconciliation.** Not central yet: the viewer/diff is read-mostly
   analysis. Becomes real only if we build collaborative editing/review (the
   AllSpice-adjacent layer). Defer, but keep the contract amenable; when adopted, write
   the reconciliation model down first (Principle 5).

6. **Routing on the backend.** Server-rendered pages (GoAppLib/templar), each page
   booting its presenter (WASM or TS per surface). Interactive UI mounts as islands into
   declared holes via `@panyam/tsappkit-solid` (`SolidIsland` + `signalView`); framework
   deps (`solid-js`) stay in the leaf island adapter, never in core/presenter. No SPA
   router (C11).

## The presenter is mandatory; its runtime is per-surface

The presenter pattern (the duplex semantic contract) is invariant. WHERE the presenter
runs is a per-surface choice driven by interaction frequency. Three tiers:

1. **View (TS, always):** rendering + raw input capture.
2. **Presenter (per-surface runtime):** interaction logic, intents -> view-model.
   - **WASM (Go)** for lower-frequency, read-mostly surfaces: reuses the Go logic.
   - **TS** for high-frequency surfaces (continuous dragging, live routing/editing)
     where a per-event WASM boundary crossing would add latency/jank. The fast loop
     stays in JS with no boundary crossing.
3. **Core domain logic (Go, always):** IR, diff, rules, simulation. Runtime-agnostic
   and authoritative. Every presenter calls it for domain ops; a TS presenter calls it
   via WASM or the server at coarse checkpoints (commit, validate, heavy compute).

Principle 1's "one logic tier" applies rigorously to the **core** (tier 3). A TS
presenter (tier 2) is thin orchestration, so what lives in TS is minimal and mechanical,
not the valuable domain logic. Keep the presenter thin so choosing TS for a surface
costs little.

**lilbattle does exactly this:** a WASM presenter for the Viewer page, and a TS
presenter for the Editor page where mouse movements need high-frequency handling.

**Our mapping:**
- **Design viewer** (browse schematic/PCB, view diff, hover/select; pan/zoom is
  view-local, hover throttled to rAF): lower-frequency. As built (WS9), it runs a thin
  **TS presenter** (`web/src/viewer.ts`) that calls the Go engine over Connect, which
  keeps heavy ops server-side (C7) with one runtime shipped. A **WASM presenter**
  reusing the Go diff/IR logic in-browser remains the swap-in if an offline or
  zero-server viewer is needed; the duplex contract does not change.
- **Editor / rules-DSL IDE** (interactive placement/routing, or live DSL editing with
  instant feedback): high-frequency, so **TS presenter**. This is also where Galore/tlex
  (TS) live: the DSL editor uses them for live parse/highlight, calling the Go core for
  validation.

## The two decisions, as made (WS9)

- **Panel rendering school:** the shipped shell is server-rendered pages with framework
  islands (`@panyam/tsappkit-solid`) mounted into declared holes; commands stay semantic
  (the panels receive typed state, not HTML). HTML-over-the-wire remains an available
  renderer on top of the semantic primitive if a data-heavy panel wants it.
- **Presenter runtime:** TS for the viewer, calling the engine over Connect; heavy ops
  server-side regardless (C7). WASM stays the per-surface swap-in described above.

## Effect on demo one

Minimal. Keep engine packages pure (`edif.Read(io.Reader)`; `diff`/`check` operate on
the IR). The CLI is the driver. The proto services materialized with the web viewer
(the `webapi` Connect services, C2), wrapping the same functions the CLI calls; a WASM
runtime would wrap them a third time without touching the core.

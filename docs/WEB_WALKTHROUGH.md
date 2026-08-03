# Web viewer walkthrough

A guided tour of the browser viewer over fixtures committed in this repo. Every step below
is runnable as written; the architecture behind what you see is
[23-web-app.md](23-web-app.md). Total time: about ten minutes.

## 0. Build and serve

```sh
make agni                          # engine CLI -> bin/agni
cd web && pnpm install && pnpm build && cd ..   # the web bundle (static/app.js is a build artifact, not committed)

bin/agni serve web \
  --mount demo=web/testdata \
  --mount fixtures=cmd/agni/testdata/conformance \
  --addr :8080
```

Open http://localhost:8080/. The positional `web` argument is the web asset directory;
designs enter only through `--mount name=path`, and each mount becomes a root in the file
tree. Add mounts pointing at your own design folders the same way.

## 1. Browse the tree

The Files panel shows both mounts. Every entry the engine can read carries a format label
(edif / kicad / ...) from the reader registry; files no reader claims are hidden. Folders
are directory URLs; the tree, like everything else here, is deep-linkable.

## 2. Render a multi-sheet design

Open `fixtures/sheetnav.fires.kicad_sch`, or jump straight to it:

    http://localhost:8080/files/fixtures/sheetnav.fires.kicad_sch

Two sheets nest under the file in the tree ("Sheet Nav Root" and its child "Power"; the
hierarchy walk resolved the sub-sheet file). Click between them; the URL's `?sheet=`
follows, so refresh and back/forward restore exactly this sheet.

What to try on any open design:

- **WebGL / SVG / Native** buttons switch the renderer. SVG is the default (full text
  fidelity); WebGL draws the packed tier-2 geometry with a DOM text overlay; Native shells
  out to the format's own tool and is enabled per tool with `--enable-native`.
- **The layout selector** is the geometry axis, orthogonal to the renderer: `faithful`
  draws the author's coordinates; the rest are auto-layouts of the netlist graph. On an
  auto-layout, the Details panel gains the conversion report (how each component drew:
  device-class glyph, generic box, provided symbol, unresolved), and the "Provided symbols"
  toggle swaps glyphs for the design's own artwork.
- Pan by drag, zoom by wheel, in both renderers. Each (renderer, file, sheet, layout) slot
  remembers its view.

`demo/demo.eds` is the faithful-EDIF counterpart: real schematic geometry, faithful-only
(an `.eds` has no netlist reader wired, so no diff or checks; the registry's capability
split is per extension).

## 3. Diff two revisions

Open side A, then arm the Compare control (top left) and pick side B in the file tree
(the button reads "pick file B in the tree"; Esc cancels):

    http://localhost:8080/files/demo/diffdemo/rev_a.kicad_sch    ->  Compare…  ->  click rev_b.kicad_sch

The pair covers the whole change taxonomy
(docs/18): R4 added, R2 removed, R3's value 10k → 22k, net OLD deleted, NEWNET added, HARD
hard-changed (its member set changed), and SIG renamed to SIG_CLK; rename recovery works
from identical connectivity under the new name.

- The **Changes panel** lists them; clicking a change locates it: both panes pan/zoom to
  the entity, tinted per side (added / removed / changed).
- **Overlay mode** stacks both revisions in one pane. It is gated by an alignment check,
  and the `.edn` twin pair (`rev_a.edn` / `rev_b.edn`) refuses it BY DESIGN: netlist-only
  formats render via auto-layout, whose node positions shift when the node set changes, so
  "shared components moved"; the refusal is the feature. The KiCad pair passes because
  faithful geometry preserves author coordinates.
- Only components tint on the KiCad pair's canvases: KiCad wire geometry carries no net
  names yet (WS1-022), so net-level changes list in the panel but do not paint.

## 4. Run the checks

Open the showcase board (the deliberate-violations half of the conformance pair):

    http://localhost:8080/files/fixtures/showcase.fires.kicad_pro

Note it is the `.kicad_pro` you open: the project read applies the external→global power
resolution the power rules need; a bare `.kicad_sch` read keeps the conservative net facts
and those rules skip themselves.

- **Rules panel**: the catalog, grouped by category, with per-design availability. The
  checkboxes are the active subset; results refresh as you toggle.
- **Checks panel**: the findings (ten on this board across six rules: bulk-cap,
  decoupling-present, esd-protection, i2c-pull-up, input-protection, test-point-coverage).
  Clicking one focuses it: every finding keeps its outline highlight and the focused
  subject gains a translucent bounding box on top (highlight shapes, WS9-017), in whichever
  renderer is showing. Net-subject highlights paint right on this faithful KiCad render, the
  showcase wires carry uuids, so WS1-022 resolves their names (SCL, USB_D+, USB_D-, VBUS, ...);
  no auto-layout needed. The two exceptions are the `+3V3` and `GND` rails: they are tapped at
  power-symbol pins with no drawn wire, so there is nothing to key a wire highlight on (on any
  layout).
- **Expectations panel**: the board's `.expect.yaml` sidecar says what SHOULD fire (six
  rules, with a `why` per rule), reconciled against what did: matched / missing / pending.
  This is the visual face of the conformance harness, the answer key that proves the checks
  found exactly the planted violations; `showcase.passes` is the anti-false-positive twin
  that must stay silent.
- **Report panel**: the severity-organized pivot, the same canonical shape
  `agni check --format report` prints.

## 5. Search the design (Query panel)

The **Query panel** (tabbed with Checks) runs ad-hoc datalog queries over the open design —
the same engine as `agni query`, in the browser. Type a query and press Run (or
⌘/Ctrl+Enter):

    component-on-net(?r,?n), net.max_voltage(?n,?v), ?v < 30 => ?r, ?n

- Don't know the vocabulary? Below the box, every relation is a **click-to-insert chip**
  grouped by kind (Netlist / Board / Datasheet / Predicates / Overlay). Clicking
  `component-on-net` inserts `component-on-net(?ref_des, ?net)` at the cursor; hover for the
  signature + description. The catalog comes from the engine (`QueryService.ListRelations`),
  so overlay-registered relations show up too.
- Results render as a table; each row's toggle expands its **provenance** (the facts that
  produced it), so citations stay out of the way until you want them.
- A malformed query shows its parse error inline, in the panel — it never leaves the panel.
- v1 evaluates on the server over the netlist facts; the datasheet `param` relation is not
  wired into the viewer yet, so datasheet joins stay on the CLI (see the userguide's
  querying page).

## 6. Deep links, recapped

Everything above is URL-addressable, so any state you reach is shareable:

    /files/<mount>/<path>            a design (add ?sheet= ?mode= ?layout= ?sym=1)
    /files/<mount>/<dir>/            a folder, expanded in the tree

Back/forward walk your navigation history; a pasted link restores file, sheet, renderer,
and layout.

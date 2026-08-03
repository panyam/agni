# xschem grammar (ingested subset)

What the `xschem` package parses out of xschem `.sch`/`.sym` files and how each construct maps
to the neutral IR (`agni.v1.ir`). This is the subset we read, not the full format. Same
posture as the KiCad grammar and the EDIF primers: document the contract between the format and
the reader.

## 1. Object stream (parse.go)

An xschem file is a flat stream of objects, one per logical line. The first character is the
object type; the rest are its fields. Fields are bare whitespace-delimited words or
brace-delimited attribute blocks `{...}`. A brace block may span physical lines, so an object
can too; `logicalLines` splits on newlines only at brace depth zero.

```
object = TYPE field*
field  = WORD | "{" text "}"
```

Types the reader acts on (others are parsed and ignored):

```
v {xschem version=...}              file header (also the format sniff, IsXschem)
N x1 y1 x2 y2 {lab=NAME}            wire segment; lab= is the net name it carries
C {SYMREF} x y rot flip {props}     component/symbol instance
B layer x1 y1 x2 y2 {props}         box; in a .sym, a box with name=/pinnumber= is a pin
```

`rot` is 0-3 (90-degree CCW steps); `flip` is 0/1 (mirror about the y-axis). `props` is a
`key=value` list; values may be `"quoted"` or bare. See `props()`.

## 2. Symbols (symbol.go)

A `.sym` is the same object stream. A pin is a `B` (box) whose props carry `name=`/`pinnumber=`;
its connection point is the box centre. `pinnumber` becomes the pin designator, `name` the pin
name.

## 3. Netlist assembly (netlist.go + internal/netgraph)

`transform` places a symbol's local pin at its schematic coordinate: flip (negate x) then rotate
`rot` then translate by the instance origin. Coordinates are scaled by 2 onto an integer grid
(xschem uses half-integers). `netgraph` then unions connection points by grid coincidence and
merges nodes that share a net name (net labels connect by name across the sheet).

Component classification:
- **label symbols** (`gnd`, `vdd`, `ipin`, `opin`, `lab_pin`, ...) name a net at their origin
  (they connect at (0,0)); their `lab=` is the net name. Not components.
- **annotation symbols** (`title`, `code`, `spice_probe`, `launcher`, ...) are skipped.
- everything else with a `name=` is a component; `name=` is the reference designator.

## 4. IR mapping

| xschem | IR |
|--------|----|
| `C` instance (non-label, non-annotation), `name=` | `Component.ref_des` (one `ComponentSection`) |
| distinct `SYMREF` | `PartType` (`kind = "xschem-symbol"`), in library `"xschem"` |
| `.sym` pin (`B` with `pinnumber=`) | `PartType.Pin` (`designator` = pinnumber, `name` = name) |
| `value=` / `device=` / `model=` / `footprint=` | `ComponentSection.attributes` (value also on `Component`) |
| `N ... {lab=NAME}` + label symbols | `Net.name`, with `Connection{component_ref, pin_ref}` per resolved pin |
| `name=` (instance) | `Provenance.native_id` (`native_id_kind = "xschem-name"`) |

Fidelity: lossy-bounded. Without a symbol library (`Read`) nets carry names but no connections;
`ReadWithSymbols` resolves pins and produces a connected netlist.

## 5. Faithful geometry (geometry.go)

`ReadSchematicGeometry` produces the drawing (for the WebGL/SVG faithful renderers), separate
from the netlist. A `.sym`'s drawing objects become `geom.Shape`s and its instance becomes a
`geom.SymbolPlacement`; the renderer applies the placement transform to the symbol-local shapes.

| xschem | geom |
|--------|------|
| `L` (line) / `P` (polygon) | `Shape` `KIND_POLYLINE` |
| `B` (box, non-pin) | `Shape` `KIND_RECT` |
| `A` (arc) | `Shape` `KIND_ARC` (3 points: start, mid, end) |
| a resolved `.sym` | `SymbolDef` (+ `Asset` `KIND_SYMBOL` with its origin) |
| `C` instance | `SymbolPlacement` (transform below) |
| `.sym` `T {@name}` / `T {@value}` | `SymbolPlacement.Field` (Reference/Value), placed at the transformed template position |
| `N ... {lab=NAME}` | `WireGeometry` grouped by net |
| `T` / label-symbol `lab=` | `Label` |

Coordinates: xschem is Y-down, geom is Y-up, so every coordinate is negated in Y (and scaled).
The placement transform maps onto geom's contract accordingly: an xschem `flip` is a y-axis
mirror (`mirror_y`), and an xschem rotation `r` becomes a `(360-90*r)`-degree geom rotation,
because negating Y reverses the sense of rotation. An unresolved symbol renders as a placeholder
box so the sheet still draws.


## Dangling-endpoint diagnostics (WS1-013)

The reader surfaces wire endpoints that terminate on nothing (no pin, anchor, or other
wire endpoint) as `ir.InputDiagnostics.dangling_endpoints`, in the geometry frame the
viewer draws (xschem's netgraph grid is scaled by 2; dangles are un-scaled back to native geometry coords on emission). Emission is GATED on full symbol resolution: an unresolved external `.sym`
drops its pins, so a wire end meant to land on one would read as a phantom dangle — one
unresolved placement suppresses the whole design's dangles (the conservative zero-false-
positive gate). Wires carry no per-wire id, so the endpoint location is the finding's
subject.

The endpoint-on-BODY diagnostic (`no_junction_endpoints`, the missing-junction T-tap) is
**KiCad-only**: xschem auto-connect a mid-span touch, so there is no missing junction to flag —
running that detection here would flag legal connections.

## Pin directions & external nets (WS1-021)

A symbol pin's `dir` attribute maps to the neutral vocabulary: `in`/`out`/`inout` ->
input/output/inout (xschem has no power-pin concept, so `power-input-not-driven` stays N/A
on xschem — the enrichment reaches floating-input and output-output-conflict instead).

The supply symbols `gnd`/`vdd`/`vss` and the hierarchy ports `ipin`/`opin`/`iopin` mark
their net `External`: a supply rail's source or a port's continuation lies outside the
single-sheet read, so the power/absence rules should not treat the net as fully seen.
Plain net labels (`lab_pin`/`lab_wire`) are sheet-local and stay unmarked.

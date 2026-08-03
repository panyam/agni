# gEDA gschem grammar (ingested subset)

What the `geda` package parses out of gEDA gschem `.sch`/`.sym` files and how each construct
maps to the neutral IR (`agni.v1.ir`). This is the subset we read, not the full format. Same
posture as the KiCad grammar and the EDIF primers.

## 1. Object stream (read.go + parse.go)

gEDA is line-oriented. Each object is one line whose first field is its type. Two shapes span
multiple lines:
- a **text** (`T`) or **path** (`H`) object: a header line whose last field is a content-line
  count `n`, followed by `n` content lines (an attribute text is `key=value` on that line);
- an **attribute block** `{ ... }`: after an object, a `{` line, then `T` objects, then `}`.

Types the reader acts on (others are parsed and ignored):

```
v RELEASE VERSION                        file header (also the sniff, IsGeda)
C x y sel angle mirror BASENAME.sym      component instance; followed by an attribute block
N x1 y1 x2 y2 color                      net segment (bare; may have a { netname=.. } block)
U ...                                    bus segment (treated like N)
T x y ... n                              text; a standalone netname= names a nearby net
P x1 y1 x2 y2 color pintype whichend     pin (in a .sym); whichend picks the connect endpoint
```

`angle` is 0/90/180/270 (CCW); `mirror` is 0/1 (reflect about the y-axis).

## 2. Symbols (symbol.go)

A `.sym` is the same stream. A pin is a `P` object; its connection point is `(x1,y1)` when
`whichend=0`, else `(x2,y2)`. `pinnumber` (from the attached block) is the pin designator. A
power symbol also carries a symbol-level `net=NAME:pin` attribute naming its supply net.

## 3. Netlist assembly (netlist.go + internal/netgraph)

`transform` places a symbol's local pin: mirror (negate x) then rotate `angle` then translate by
the instance origin. gEDA coordinates are integers, used directly on the grid. `netgraph` unions
connection points by coincidence and merges nodes sharing a net name.

gEDA net segments are unlabelled, so net names come from:
- `netname=` texts, snapped to the nearest wire endpoint;
- power/ground taps (`gnd-1`, `vcc-1`, ...): resolved through their `.sym` (the connect pin is
  offset from the origin, unlike xschem) and named from the instance `net=`, else the symbol's
  `net=`, else convention (`GND`/`VCC`/...).

Annotation symbols (`title-*`, `spice-directive-1`, ...) are skipped.

## 4. IR mapping

| gEDA | IR |
|------|----|
| `C` instance (non-power, non-annotation), `refdes=` | `Component.ref_des` (one `ComponentSection`) |
| distinct `BASENAME` | `PartType` (`kind = "geda-symbol"`), in library `"geda"` |
| `.sym` pin (`P` + `pinnumber=`) | `PartType.Pin` |
| `value=` / `device=` / `model-name=` / `footprint=` | `ComponentSection.attributes` |
| net (wires + netname/power taps) | `Net.name`, with `Connection{component_ref, pin_ref}` per resolved pin |
| `refdes=` | `Provenance.native_id` (`native_id_kind = "geda-refdes"`) |

Fidelity: lossy-bounded. Without a symbol library (`Read`) only `netname=` nets are known by
name; `ReadWithSymbols` resolves pins and produces a connected netlist (nets without a name get
a synthetic `N$<n>`).

## 5. Faithful geometry (geometry.go)

`ReadSchematicGeometry` produces the drawing (for the WebGL/SVG faithful renderers), separate
from the netlist. gEDA is Y-up like geom, so coordinates pass through unscaled and the placement
transform maps directly (`angle` is a geom CCW rotation, `mirror` is `mirror_y`).

| gEDA | geom |
|------|------|
| `L` (line) | `Shape` `KIND_POLYLINE` |
| `B` (box, x,y + w,h) | `Shape` `KIND_RECT` |
| `V` (circle) | `Shape` `KIND_CIRCLE` |
| `A` (arc) | `Shape` `KIND_ARC` |
| a resolved `.sym` | `SymbolDef` (+ `Asset` `KIND_SYMBOL`) |
| `C` instance | `SymbolPlacement` |
| instance-block attribute `T x y … refdes=/value=` | `SymbolPlacement.Field`, placed at the attribute text's own coords (justify/size/visibility from the `T`) |
| `N` (wire) | `WireGeometry` |
| `T netname=` | `Label` |
| `G` (picture) | `Image` (+ `Asset` `KIND_IMAGE`) |

An embedded `G` picture's base64 bytes are read as the run of pure-base64 lines after the
filename (the format stores no length); an external picture yields a bbox-only `Image`. An
unresolved symbol renders as a placeholder box.


## Dangling-endpoint diagnostics (WS1-013)

The reader surfaces wire endpoints that terminate on nothing (no pin, anchor, or other
wire endpoint) as `ir.InputDiagnostics.dangling_endpoints`, in the geometry frame the
viewer draws. Emission is GATED on full symbol resolution: an unresolved external `.sym`
drops its pins, so a wire end meant to land on one would read as a phantom dangle — one
unresolved placement suppresses the whole design's dangles (the conservative zero-false-
positive gate). Wires carry no per-wire id, so the endpoint location is the finding's
subject.

The endpoint-on-BODY diagnostic (`no_junction_endpoints`, the missing-junction T-tap) is
**KiCad-only**: gEDA auto-connect a mid-span touch, so there is no missing junction to flag —
running that detection here would flag legal connections.

## Pin directions & power-rail drivenness (WS1-021)

A symbol pin's `pintype` maps to the neutral pin-direction vocabulary: `pwr` ->
`power_in` (a placed part's power/ground pin draws supply — gEDA does not distinguish
power_in from power_out, so a rare power-output pin is miscategorized, tolerated because
its rail is marked External), `in`/`out`/`io`/`pas` -> input/output/inout/passive, the
rest UNSPECIFIED. This makes the pin-type rules (power-input-not-driven, floating-input,
output-output-conflict) reachable on gEDA designs.

A power/ground SUPPLY symbol (`gnd`/`vcc`/`vdd`/`vss`) marks its net `External`, matching
KiCad's power-symbol semantics: the rail is a global supply whose source may lie off the
read. gEDA has no separate PWR_FLAG driver directive, so the supply symbol alone carries
this. External (not `power_driven`) is deliberate — it keeps power-input-not-driven quiet
on a tapped rail without the bulk-cap noise a driven mark would add on sim-oriented designs.

# gEDA gschem grammar (ingested subset)

This page lists what the `geda` package parses out of gEDA gschem `.sch`/`.sym` files, and how
each construct maps to the neutral IR (`agni.v1.ir`). This is the subset we read, not the full
format. The KiCad grammar and the EDIF primers take the same posture.

## 1. Object stream (read.go + parse.go)

gEDA is line-oriented. Each object is one line whose first field is its type. Two shapes span
multiple lines:
- a **text** (`T`) or **path** (`H`) object is a header line whose last field is a content-line
  count `n`, followed by `n` content lines (an attribute text is `key=value` on that line);
- an **attribute block** `{ ... }` follows an object as a `{` line, then `T` objects, then `}`.

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
- power/ground taps (`gnd-1`, `vcc-1`, ...). The reader resolves each tap through its `.sym`,
  since the connect pin is offset from the origin, unlike xschem. It names the net from the
  instance `net=`, else the symbol's `net=`, else convention (`GND`/`VCC`/...).

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

Fidelity is lossy-bounded. `Read` has no symbol library, so only `netname=` nets are known by
name. `ReadWithSymbols` resolves pins and produces a connected netlist, and a net without a name
gets a synthetic `N$<n>`.

## 5. Faithful geometry (geometry.go)

`ReadSchematicGeometry` produces the drawing for the WebGL/SVG faithful renderers, separately
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

The format stores no length for an embedded `G` picture, so the reader takes its base64 bytes
from the run of pure-base64 lines after the filename. An external picture yields a bbox-only
`Image`. An unresolved symbol renders as a placeholder box.

## 6. Dangling-endpoint diagnostics (WS1-013)

Some wire endpoints terminate on nothing, with no pin, anchor, or other wire endpoint there.
The reader surfaces them as `ir.InputDiagnostics.dangling_endpoints`, in the geometry frame
that the viewer draws. Emission is GATED on full symbol resolution. An unresolved external
`.sym` drops its pins, so a wire end meant to connect to one would read as a phantom dangle.
One unresolved placement therefore suppresses the whole design's dangles. This is the
conservative zero-false-positive gate. Wires carry no per-wire id, so the endpoint location is
the finding's subject.

This reader emits no endpoint-on-BODY diagnostic. That diagnostic, `no_junction_endpoints`,
flags the missing-junction T-tap, and it is **KiCad-only**. gEDA auto-connects a mid-span
touch, so there is no missing junction to flag, and running that detection here would flag
legal connections.

## 7. Pin directions and power-rail drivenness (WS1-021)

A symbol pin's `pintype` maps to the neutral pin-direction vocabulary. `pwr` becomes
`power_in`, because a placed part's power/ground pin draws supply. gEDA does not distinguish
power_in from power_out, so a rare power-output pin is miscategorized. That is tolerated,
because its rail is marked External. `in`/`out`/`io`/`pas` map to input/output/inout/passive,
and the rest to UNSPECIFIED. With this mapping the pin-type rules (power-input-not-driven,
floating-input, output-output-conflict) can run on gEDA designs.

A power/ground SUPPLY symbol (`gnd`/`vcc`/`vdd`/`vss`) marks its net `External`, matching
KiCad's power-symbol semantics, since the rail is a global supply whose source may lie off the
read. gEDA has no separate PWR_FLAG driver directive, so the supply symbol alone sets this
mark. Marking the net External rather than `power_driven` is deliberate. External keeps
power-input-not-driven quiet on a tapped rail. A driven mark would also add bulk-cap noise on
sim-oriented designs, and External does not.

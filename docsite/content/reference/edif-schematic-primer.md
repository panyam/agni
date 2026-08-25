---
title: "EDIF schematic primer"
description: "The .eds schematic export: geometry, symbols, orientation, and the keys that join it back to the netlist IR."
---

Companion to the [EDIF netlist primer](../edif-primer/) (which covers the `.edn` netlist) and to
the geometry sidecar work. This page studies the `.eds` SCHEMATIC export the way the netlist
primer studied the `.edn`, so the geometry sidecar proto and reader can be designed against
ground truth.

Source file: a 62MB real-world `.eds` SCHEMATIC export (proprietary, kept outside the repo),
`edifVersion 2 0 0`, `edifLevel 0`, exported by Siemens xDX Designer.

## 1. What a schematic is (the concepts)

A schematic is the human-facing **drawing** of a design. It encodes the same connectivity as the
{{ explainable "netlist" }}, but as a diagram a person reads. The vocabulary:

| Concept | What it is | In EDIF |
|---|---|---|
| **Sheet / page** | One drawing page. A design is many sheets (this one has 82). | `(page ...)` |
| **Symbol** | The reusable picture of a part type (a resistor zig-zag, an IC box with pins). Drawn once, placed many times. | `(symbol ...)` inside a cell view |
| **Pin** | A connection point on a symbol, at a fixed local coordinate. Wires attach here. | `(portImplementation ... (connectLocation ...))` |
| **Placement / instance** | A symbol dropped on a sheet at some position/rotation, standing for one physical part. | `(instance ...)` inside a page |
| **Ref-des** | The {{ explainable "reference-designator" }} of a placement (`R12`, `J1900`). Names the physical part and joins the drawing to the netlist. | `(designator (stringDisplay "J1900" ...))` |
| **Wire / net segment** | A drawn line (polyline) connecting pins. Belongs to a net. | `(figure NET (path (pointList ...)))` |
| **Net** | An electrical node: all the pins and wires at the same potential. | `(net ... (joined ...))` |
| **Junction / dot** | A filled dot where wires that cross are electrically joined. | `(dot (pt ...))` |
| **Off-page connector** | A tag that continues a net onto another sheet without a drawn wire. | `(offPageConnector ...)` |
| **Label / annotation** | Free text on the sheet (net names, notes, table titles, block names). | `(annotate (stringDisplay ...))` |
| **Title block** | The border/metadata frame (sheet name, revision, date). It is just another placed symbol. | `(instance ... (viewRef ... (libraryRef Borders)))` |
| **Bus** | A {{ explainable "bus" }} drawn as one thick line. Present in the style palette (`WIRE_BUS`) and not central to a first render. | `figureGroup BUS` |

For rendering, **a symbol is defined once, as shapes plus pin positions in symbol-local
coordinates, and a placement is that symbol plus a transform.** To draw a placed part you look up
its symbol, apply the placement transform, and stroke the shapes. Wires are drawn straight from
their own point lists, since they already carry absolute sheet coordinates.

{{ includeFile "figures/edif-symbol-placement.svg" }}

## 2. `.eds` is a superset of `.edn`

The schematic export contains **everything the netlist has, plus graphics**. The two files
describe one design. The instance internal ids (`&04428I78`) are the same in both, so the geometry
sidecar can key back to the netlist IR.

| Content | `.edn` netlist | `.eds` schematic |
|---|---|---|
| Part-type library (cells, ports, directions) | yes | yes |
| Placed instances + ref-des | yes | yes |
| Net connectivity (joined pins) | yes | yes (duplicated) |
| Component properties | yes | yes (often richer) |
| Symbol graphics (shapes, pin coordinates) | no | **yes** |
| Placement geometry (position, rotation) | no | **yes** |
| Wire routing (polylines, junctions) | no | **yes** |
| Sheets / pages, title blocks, annotations | no | **yes** |
| Drawing style palette (colors, line widths, text heights) | no | **yes** |
| Display metadata (what/where labels show) | no | **yes** |
| Off-page connectors, back-annotation | no | **yes** |

Connectivity is read from the lighter `.edn` and geometry from the `.eds`. Connectivity is not
re-derived from the `.eds`. The two are joined by key at render time.

## 3. Coordinate system and units

| Fact | In this file |
|---|---|
| **Coordinates are integers** | In EDIF distance units. The header's `(scale 1 (e 1 -8) (unit DISTANCE))` declares 1 unit = 1e-8 m, so **10 nanometers**, and `59690000` units = 596.9 mm. Store the raw integers and record `unit_nm = 10` once, so the renderer can convert to mm or pixels. |
| **Points** | `(pt X Y)`, with Y increasing upward (schematic convention). Symbol shapes frequently use negative Y, because the symbol origin is a top reference and pins hang below it. |
| **Page size** | `(pageSize (rectangle (pt 0 0) (pt 86360000 55880000)))`, so 863.6 mm by 558.8 mm for this design. |
| **Bounding boxes** | `(boundingBox (rectangle (pt) (pt)))` appears on symbols and sheets, and is the natural key for the spatial index and for viewport culling. |

**float32 precision trap (renderer).** `float32` represents integers exactly only up to
2^24 = 16,777,216, and coordinates here reach ~8.6e7. Uploading raw units as `float32` GPU
attributes loses precision and misaligns wires. Keep int32 through storage, where the full range
fits comfortably (~8.6e7 against a limit of ~2.1e9), and either use integer vertex attributes
converted in-shader or rebase per sheet, subtracting the sheet origin, before converting to float.

<details>
<summary>Why integers rather than floats</summary>

The EDIF spec stores geometry as integer counts of a database unit plus a scale, and so do GDSII,
Gerber, ODB++ and KiCad. The reason is exactness on a manufacturing grid. Two wire endpoints, or a
pin and a wire, must be the *same* point to be electrically connected. Integers give bit-exact
equality where floats give near-misses (`0.1 + 0.2 != 0.3`) and ambiguous serialization, which
fights round-trip fidelity (see [Ingestion and IR](../../architecture/ingestion-and-ir/)).
Integers are kept end to end.

</details>

## 4. Document structure (top to bottom)

```
(edif DxD
  (edifVersion 2 0 0) (edifLevel 0) (keywordMap ...)
  (status (written (timestamp ...) (author ...) (program ...)))

  (library <Name>                         ; one or more
    (technology                           ; the DRAWING STYLE palette
      (numberDefinition (scale ... (unit DISTANCE)) (gridMap ...))
      (figureGroup BOX  (color ...))      ; per figure class: color, pathWidth, textHeight
      (figureGroup PIN  (color ...))
      (figureGroup NET  (color ...)) ...)
    (cell (rename &id "PartType")          ; a PART TYPE
      (cellType GENERIC)
      (view (rename &id "view")
        (viewType SCHEMATIC)
        (interface (designator "J?") (port ...) ...)   ; pins (same as netlist)
        (symbol ...))))                                ; THE GRAPHIC (section 5)

  (design <Name>
    (cellRef <top> (libraryRef <lib>))
    (property ...)                         ; design-level attributes
    (viewMap (instanceBackAnnotate ...))   ; back-annotation (section 6)
    ... (contents ... (page ...) ...)))    ; the SHEETS live in the top cell view
```

The sheets are inside the top design cell's SCHEMATIC view `(contents ...)`. Each `(page ...)`
holds the placed instances, the routed nets, the annotations, and the off-page connectors for
that sheet.

## 5. The geometry we extract (the render subset)

This is what the sidecar proto models. Everything else (section 6) is dropped or kept opaque.

### 5a. Symbol definition (drawn once per part type)

```
(symbol
  (boundingBox (rectangle (pt 0 -9144000) (pt 1016000 0)))
  (figure BOX (rectangle (pt 0 -9144000) (pt 762000 0)))        ; a shape
  (figure ARC (openShape (curve (arc (pt s) (pt mid) (pt e))))) ; an arc shape
  (portImplementation
    (name &1 (display ...))
    (connectLocation (figure GRAPHICS (dot (pt 1016000 -254000))))  ; PIN COORDINATE
    (figure PIN (path (pointList (pt 762000 -254000) (pt 1016000 -254000)))) ; pin stub
    (keywordDisplay designator (display ...)))
  (keywordDisplay cell (display ...)))
```

- Shapes come from `(figure GROUP <shape>)`. The `GROUP` (BOX, PIN, NET, LINE, ...) is a style
  hint that maps to the technology palette (color/width).
- **Pin coordinates** are the `connectLocation` `(dot (pt X Y))`, in symbol-local units. This is
  where a wire attaches. The `figure PIN` path is just the little stub line drawn from the body
  to the pin end.
- `name &1` is the port internal id. `keywordDisplay`/`propertyDisplay` say where the pin number
  and part attributes render (label placement can be deferred initially).

### 5b. Shape kinds observed

| EDIF | Meaning | Points |
|---|---|---|
| `(rectangle (pt) (pt))` | axis-aligned box | 2 (min/max corners) |
| `(path (pointList (pt)...))` | open polyline | N in order |
| `(circle (pt) (pt))` | circle | 2 (defines radius) |
| `(openShape (curve (arc (pt) (pt) (pt))))` | circular arc | 3 (start, mid, end) |
| `(dot (pt))` | single point (junction / connect location) | 1 |

### 5c. Placement (symbol dropped on a sheet)

```
(instance (rename &04428I78 "$28I78")
  (viewRef H055MTD2 (cellRef (name PE660095 ...) (libraryRef Connector)))  ; which symbol
  (transform (orientation MY) (origin (pt 63500000 36830000)))             ; where + how
  (portInstance &20551 (designator (stringDisplay "3" ...)))               ; pin-number labels
  (designator (stringDisplay "J1900" ...)))                                ; REF-DES
```

- `viewRef -> cellRef -> libraryRef` selects the symbol to draw. Note `cellRef` may be a bare
  atom or a `(name X (display ...))` form. Handle both.
- `transform` = `(origin (pt X Y))` plus an `(orientation ...)` (section 7). Rare
  `(scaleX ...)`/`(scaleY ...)` also appear (7 each in this file).
- `(designator (stringDisplay "J1900" ...))` is the ref-des. Unlike the netlist, it wraps a
  `stringDisplay` (it carries its own on-sheet label position).

### 5d. Wire routing (per net, per sheet)

Nets nest. There is an outer logical net, then inner physical net-segment groups that carry the
drawn wires.

```
(net GND
  (joined (globalPortRef GND) (portRef &2 (instanceRef &04428I71)) ...)   ; logical membership
  (net (rename &04428N58 "$28N58")
    (joined (portRef &2 (instanceRef &04428I71)) ...)
    (figure NET (path (pointList (pt 59690000 33528000) (pt 59690000 33020000))))  ; a WIRE
    (figure NET (path (pointList ...)))                                            ; another
    ...))
```

Each `(figure NET (path (pointList ...)))` is one wire polyline. The net name is the join key
back to the IR net.

### 5e. Sheet and free graphics

```
(page (rename HEADERS_055055... "HEADERS -- HMTD and 9_5mm")
  (pageSize (rectangle (pt 0 0) (pt 86360000 55880000)))
  (commentGraphics
    (figure (figureGroupOverride BOX) (rectangle (pt) (pt)))   ; decorative boxes
    (annotate (stringDisplay "SWITCH A" (display ... (origin (pt 27178000 38608000))))))  ; text
  (instance ...) (net ...) (offPageConnector ...))
```

`annotate`/`stringDisplay` are free text labels. `commentGraphics` are non-electrical decorations
(grouping boxes, notes).

## 6. Everything else in the file (beyond geometry)

The `.eds` carries a lot that does **not** go in the geometry sidecar. Cataloged so the
deliberate drops are known (fidelity is lossy-bounded, a render subset):

| Category | Constructs | Why we drop / defer it |
|---|---|---|
| **Connectivity** | `net`, `joined`, `portRef`, `instanceRef`, `globalPortRef`, `offPageConnector` | Already in the core IR from the `.edn`. We take only the wire polylines, not the logical graph. (Off-page connectors are a candidate to keep later for cross-sheet navigation.) |
| **Attributes** | `property` (`string`/`integer`/`boolean`), `owner` | Belong in the core IR (component attributes), not geometry. |
| **Display metadata** | `display`, `figureGroupOverride`, `visible`, `justify`, `orientation`, `textHeight`, `keywordDisplay`, `propertyDisplay` | Controls where each attribute label prints. We can render a subset (net names, ref-des, values) later, not needed for a first faithful render. |
| **Style palette** | `technology`, `figureGroup`, `color`, `pathWidth`, `borderPattern`, `gridMap` | Drawing style. We can carry `figure_group` as a style hint and resolve colors in the renderer, and the full palette is optional. |
| **Header / status** | `status`, `written`, `timestamp`, `author`, `program`, `keywordMap` | Provenance only. |
| **Back-annotation** | `viewMap`, `instanceBackAnnotate`, `portBackAnnotate` | Data pushed back from other tools (print order, sheet totals, pin types). Not render geometry. |
| **Opaque extras** | `userData` | Vendor-specific flags (e.g. `visibleName`). Keep opaque if at all. |

**Note on connectivity in the `.eds`:** it is present and consistent with the `.edn`, so in
principle the schematic alone could feed both the IR and the geometry. We keep the split
(connectivity from `.edn`, geometry from `.eds`) because the `.edn` is 6x smaller and the netlist
reader already parses it, and because the sidecar architecture wants geometry to be independently
sourced (see [Ingestion and IR](../../architecture/ingestion-and-ir/)).

## 7. Orientation semantics

`transform` orientation codes (counts in this file), applied to symbol-local coordinates before
translating by `origin`:

| Code | Meaning | Count |
|---|---|---|
| `R0` | no rotation | 74003 |
| `R90` | rotate 90 CCW | 15060 |
| `R180` | rotate 180 | 2929 |
| `R270` | rotate 270 CCW | 4002 |
| `MY` | mirror across Y axis (flip X) | 1698 |
| `MX` | mirror across X axis (flip Y) | 110 |
| `MXR90` | mirror X then rotate 90 | 738 |
| `MYR90` | mirror Y then rotate 90 | 163 |

As 2x2 matrices on symbol-local `(x, y)` (Y-up), then add `origin`:

```
R0    [ 1  0; 0  1]      R90   [ 0 -1; 1  0]
R180  [-1  0; 0 -1]      R270  [ 0  1;-1  0]
MX    [ 1  0; 0 -1]      MY    [-1  0; 0  1]
MXR90 = R90 * MX         MYR90 = R90 * MY
```

Pins transform the same way, so a pin's absolute position is
`transform(orientation) * connectLocation + origin`. This is how wire endpoints line up with pins
at render time.

## 8. The join contract (keys back to the core IR)

The sidecar never contains the IR. It references the IR by stable keys, resolved at render time
(see [Ingestion and IR](../../architecture/ingestion-and-ir/)).

| Geometry element | Key | Joins to core IR |
|---|---|---|
| `SymbolPlacement` | `ref_des` (+ `source_id` = instance `&id`) | `ComponentInstance.ref_des` (`&id` is the robust key, since ref-des repeats for multi-section parts) |
| `SymbolDef` | `cell_ref`, `library_ref`, `view_ref` | `ComponentInstance.cell_ref` / `library_ref` |
| `WireGeometry` | `net` name | `Net.name` |
| `PinPoint` | `port_ref` | `Port.designator` / net `PortRef.port_ref` |

`source_id` (the EDIF `&id`) is the crux. It is identical across `.edn` and `.eds`, so it is the
unambiguous join even where display names collide.

**The symbol join needs a third key beyond cell and library.** A placement references a cell by
display name *or* internal `&id`, a library likewise (`(rename Ferrite_Bead "Ferrite Bead")`), and
a **view** by id (`(viewRef &..._D... (cellRef (name &cellid) ...))`). A multi-section cell, say a
connector with A/B/C/D banks, defines one SCHEMATIC view per bank and each bank is its own symbol.
So the reader emits one `SymbolDef` per view, keyed by `(cell_ref, library_ref, view_ref)`, and
normalizes every reference to the display name.

Builtin GRAPHIC cells (GND, no-connect, off-page) are the exception. They keep their geometry
under `(view (contents (figure ...)))` with no `(symbol ...)` node at all. Pin-number labels come
from each `portImplementation`'s `(name X (display (origin ...)))`, and off-page connector net
names from a page-level `(portImplementation (name X (display ...)))`.

## 9. Grammar sketch (the schematic subset we read)

EBNF-ish, restricted to the render subset. `ID` = identifier or `(rename &id "disp")`, `INT` =
integer, `STR` = quoted string. Constructs not listed are skipped by the reader.

```
schematic     = "(edif" NAME header library+ design ")"
header        = edifVersion edifLevel keywordMap status
library       = "(library" NAME edifLevel technology cell+ ")"
technology    = "(technology" numberDefinition figureGroup+ ")"
numberDefn    = "(numberDefinition" scale+ gridMap ")"
scale         = "(scale" INT [ "(e" INT INT ")" | INT ] "(unit" UNIT ")" ")"
figureGroup   = "(figureGroup" NAME styleAttr* ")"
styleAttr     = color | pathWidth | textHeight
cell          = "(cell" ID "(cellType" TYPE ")" view ")"
view          = "(view" ID "(viewType" ("SCHEMATIC"|"NETLIST"|"GRAPHIC") ")"
                       interface [ symbol ] [ contents ] ")"
interface     = "(interface" [ designator ] port* ")"
port          = "(port" ID [ "(direction" DIR ")" ] [ designator ] ")"

symbol        = "(symbol" [ boundingBox ] figure* portImpl* keywordDisplay* ")"
figure        = "(figure" [ GROUP ] shape* ")"
shape         = rectangle | path | circle | dot | openShape
rectangle     = "(rectangle" pt pt ")"
path          = "(path (pointList" pt+ "))"
circle        = "(circle" pt pt ")"
dot           = "(dot" pt ")"
openShape     = "(openShape (curve" arc+ "))"
arc           = "(arc" pt pt pt ")"          // start, mid, end
pt            = "(pt" INT INT ")"
portImpl      = "(portImplementation (name" ID [display] ")"
                       connectLocation figure* keywordDisplay* ")"
connectLoc    = "(connectLocation (figure GRAPHICS (dot" pt ")))"

design        = "(design" NAME cellRef property* [ viewMap ] contents ")"
contents      = "(contents" [ boundingBox ] [ commentGraphics ] page+ ")"
page          = "(page" ID pageSize [ commentGraphics ] instance* net* offPage* ")"
pageSize      = "(pageSize (rectangle" pt pt "))"
commentGraph  = "(commentGraphics" (figure | annotate)* ")"
annotate      = "(annotate (stringDisplay" STR display "))"

instance      = "(instance" ID viewRef transform portInstance* [ designator ] property* ")"
viewRef       = "(viewRef" NAME "(cellRef" cellName "(libraryRef" NAME ")" ")" ")"
cellName      = NAME | "(name" NAME display ")"
transform     = "(transform" [ orientation ] [ origin ] [ scaleX ] [ scaleY ] ")"
orientation   = "(orientation" ("R0"|"R90"|"R180"|"R270"|"MX"|"MY"|"MXR90"|"MYR90") ")"
origin        = "(origin" pt ")"
designator    = "(designator" (STR | "(stringDisplay" STR display ")") ")"

net           = "(net" [ NAME ] joined (net | wire)* ")"    // outer logical, inner physical
joined        = "(joined" (portRef | globalPortRef)* ")"
portRef       = "(portRef" ID "(instanceRef" ID ")" ")"
wire          = "(figure NET" path ")"
offPage       = "(offPageConnector" ID ")"
```

## 10. Gotchas (schematic-specific, on top of the netlist primer's)

| Gotcha | What it means |
|---|---|
| **Symbol vs placement split** | Shapes and pin coordinates live once in the cell's `(symbol ...)`, in symbol-local coordinates. A placement is only a transform plus a `cellRef`, so you must join placement to symbol to draw anything. |
| **Nets nest** | The outer `(net NAME (joined ...))` is the logical signal. The wires live in inner `(net (rename ...) (joined ...) (figure NET ...))` groups. Collect wires from the inner nets, and take the join key, the net name, from the outer one. |
| **`cellRef` is polymorphic** | It can be a bare atom or `(name X (display ...))`. |
| **`designator` wraps `stringDisplay` here** | Not a bare string, as it is in the netlist. |
| **Pins need the transform** | A wire endpoint only matches a pin after you apply the placement orientation and origin to the symbol-local `connectLocation`. |
| **Volume is per-design, not per-view** | ~121k figures and ~149k points across 82 sheets. Per sheet that is a few thousand primitives, and the renderer loads one sheet at a time. See [Geometry and rendering](../../architecture/geometry-and-rendering/) for why the proto models bulk geometry as packed columnar arrays rather than object-per-point. |
| **Y is up** | Symbol shapes commonly use negative Y below a top-origin. |

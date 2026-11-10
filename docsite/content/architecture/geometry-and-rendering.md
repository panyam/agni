---
title: "Geometry sidecar and rendering"
description: "How schematic and board geometry are represented as a keyed sidecar and rendered in the browser at scale."
---

Diff, rules, and simulation work on connectivity, and they never need pixels. A web viewer does. So Agni keeps drawing geometry in a separate artifact, the geometry sidecar. Drawing geometry means symbol shapes, placements, wire routing and pin coordinates. The sidecar references the [core IR](../ingestion-and-ir/) by stable keys and joins to it at render time. This page covers how that geometry is represented and how it renders in the browser at scale, meaning 100k+ primitives per design across dozens of sheets.

## The problem

A web renderer needs geometry. The constraint is the volume, not the frame rate. The reference design has roughly 121k figures and 149k points across 82 sheets. The naive path uses one proto message per point and one draw object per primitive. It allocates hundreds of thousands of objects on both sides of the wire and again on the GPU, and those allocations break it.

## Three representations, kept distinct

Collapsing the API, the wire format and the compute layout into one type would be a mistake. Agni keeps them separate, and each one matches a different access pattern.

{{ includeFile "figures/geometry-three-tiers.svg" }}

### Tier 1, the logical contract (the public API)

Nested, ergonomic proto messages, sized in the thousands: `SymbolDef`, `SymbolPlacement{ref_des, transform}`, `PinPoint{port_ref, x, y}`, `WireGeometry{net, ...}`, `SheetGeometry`. Single reads ("give me placement R12"), list reads ("placements on sheet 3"), picking results and IR-keying use this tier, and other tools consume it. It is stable and readable, and storage or packing details never leak into it. The proto is the single source of truth for this tier.

### Tier 2, the columnar transport (the storage and wire-optimized form)

A separate message that carries the vertex streams as a **`bytes` blob** with a defined little-endian columnar layout, addressed by index back to tier 1. The bulk path requests it explicitly ("give me the packed buffers for sheet N"), and it is never mixed into tier 1. Tier 2 crosses to the GPU.

Coordinates here are int32 and sheet-relative (rebased). The reason is the consumer, not disk size. The whole design is about 1.2MB versus 2.4MB, which is immaterial. WebGL2 and GLSL ES 3.0 have no 64-bit vertex attribute and no `int64` or `double` in shaders, so the blob that feeds GPU buffers is physically 32-bit. Storing int64 here would need a narrowing pass on load and would defeat the zero-copy envelope. The blob uses int32 rather than float32, because int32 is exact to 2^31 while float32 loses exactness at 2^24, below the coordinate range. Rebasing per sheet subtracts the sheet's minimum corner from every coordinate. The sheet extent then bounds every value, whatever the magnitude of the absolute coordinates. So int32 is provably safe, and the GPU never meets the float32 precision trap. Tier 1 is still int64. It is read-only, small and range-safe, and it bakes no unit assumption into the public API. Narrowing happens only in tier 2.

### Tier 3, the in-memory compute form (Go)

The reader builds this form, and a server-side spatial index or culler operates on it. Columnar and allocation-light (`struct { Xs, Ys []int32; ... }`). This is a derived projection of the proto contract, not a second schema.

## Why not just use proto everywhere

Vanilla protobuf-go generates `[]*Point` for a `repeated Point`, so bulk geometry becomes about 149k heap objects with pointer chasing, and marshal or unmarshal touches every varint. That shape is fine for a wire or disk contract. It is wrong when a renderer or spatial index iterates over it. It also gains nothing at the browser edge. Bulk data is copied into a typed array at the browser boundary anyway, and in the plan at the time that boundary was a WASM boundary. So proto is the boundary format and tier 3 is the compute format.

## Making wire serialization nearly free

Tier 2 is a `bytes` field holding a packed columnar buffer. This removes the mapping tax on the hot path:

- **In memory (tier 3)** the columnar arrays' backing store *is* the byte slice, or a trivial view over it. Mapping to proto is a slice assignment. No per-element copy, no `[]*Point`.
- **To serialize**, proto encodes a `bytes` field as tag, length, and a raw copy, which is one memcpy rather than 149k varint encodes.
- **To deserialize**, you get the slice back and reinterpret it as columnar arrays.
- **In the browser** the blob arrives as a `Uint8Array`, and you make `Int32Array` or `Float32Array` views over it and hand them to GPU buffers, zero-copy on the JS side.

The cost is that the blob is opaque to proto tooling, which can neither introspect its fields nor evolve unknown fields inside it. That layout is versioned explicitly with a `layout_version`, which is acceptable for a render-only vertex stream. Elsewhere, unknown-field retention motivates strict proto modeling. That retention is a property of the lossless IR, and it does not apply to this lossy-bounded render subset. The layout spec is still authoritative and versioned, so the cross-language no-drift intent still holds.

<details>
<summary>Why FlatBuffers and Cap'n Proto were considered and not adopted</summary>

They are serialization formats. They do not replace the Go-WASM-to-JS bridge generator, which was the plan at the time. That generator emits the service exposure, the TS client facades and the duplex presenter scaffolding. Proto was kept for two reasons.

1. **The zero-copy win is marginal here.** FlatBuffers and Cap'n Proto read fields in place with no parse step. For the vertex tier the `bytes` envelope already gets that, because the blob is the columnar layout, TS makes typed-array views over it zero-copy, and Go assigns the slice. Their only extra is structured random access inside the blob, and a flat vertex pool with an owned layout does not need it. Adopting a new toolchain would save one memcpy of a few MB per sheet.
2. **Neither ships a Go-WASM bridge generator.** Adopting one means hand-writing the presenter-service exposure, the TS clients and the boundary wiring, all of which the planned wasmjs pipeline would generate. It also means running a second IDL alongside the proto IR, so the netlist IR and the geometry sidecar would split across two schema toolchains. One schema toolchain across the system beats a per-vertex micro-optimization.

If profiling later shows the tier-2 or boundary copy is a real bottleneck, FlatBuffers could be adopted just for the vertex blob while keeping proto for the contract and the bridge, since tier 2 sits behind the contract and the swap is localized. That is premature now.

</details>

## Proto contract sketch

Tier 1 (logical) lives in a package `agni.v1.geom`, a separate artifact never imported by diff, rules, or simulation:

```proto
message SchematicGeometry {           // the sidecar for one design
  string design_ref = 1;              // joins to ir.Design.name
  int64  unit_nm = 2;                 // nanometers per source unit (10 for this EDIF)
  repeated SymbolDef symbols = 3;     // symbol library, keyed by cell_ref
  repeated SheetGeometry sheets = 4;
  Provenance prov = 16;
}
message SymbolDef {                   // reusable graphic per part type
  string cell_ref = 1; string library_ref = 2;
  BBox bbox = 3;
  repeated Shape shapes = 4;
  repeated PinPoint pins = 5;         // connectLocation dots, symbol-local
  Provenance prov = 16;
}
message SymbolPlacement {             // a symbol dropped on a sheet
  string ref_des = 1;                 // joins to ir.ComponentInstance.ref_des
  string cell_ref = 2; string library_ref = 3;
  Transform transform = 4;
  Provenance prov = 16;               // source_id = EDIF instance &id (unique join key)
}
message WireGeometry { string net = 1; repeated Polyline polylines = 2; }  // small designs
message SheetGeometry {
  string id = 1; string name = 2; BBox size = 3;
  repeated SymbolPlacement placements = 4;
  repeated WireGeometry wires = 5;    // tier-1 form; tier-2 PackedSheet is the scale path
  repeated Label labels = 6;
}
message Point { int64 x = 1; int64 y = 2; }
message BBox  { Point min = 1; Point max = 2; }
// Format-neutral placement: origin + rotation + mirror + optional scale (not EDIF's
// fixed orientation codes), so other-format readers map onto it. EDIF's 8 codes and
// scaleX/Y map exactly (MXR90 = mirror_x + rotation 90, etc.).
message Transform { Point origin = 1; int32 rotation_deg = 2; bool mirror_x = 3; bool mirror_y = 4; double scale_x = 5; double scale_y = 6; }
message Shape { Kind kind = 1; repeated Point points = 2; int64 radius = 3; string figure_group = 4; }
message PinPoint { string port_ref = 1; Point loc = 2; string source_id = 3; }
message Polyline { repeated Point points = 1; }
message Label { string text = 1; Point origin = 2; int64 height = 3; string justify = 4; int32 rotation_deg = 5; }
```

The sidecar is a format-neutral geometry IR, not an EDIF dump. The primitives (shapes, points, polylines, placements, pins, labels, sheets) are universal 2D schematic vector constructs, and the transform is a general rotation, mirror, and scale rather than EDIF's fixed orientation enum. So KiCad, xschem, gEDA and other readers can populate the same sidecar, and the other readers include Altium via export and OrCAD. This mirrors the many-readers-one-IR idea. Symbol graphics come from the source file itself (EDIF embeds them as vectors), so the renderer needs no external symbol-asset library for the faithful view. Only the netlist-graph fallback and text glyphs are drawn by Agni.

Tier 2 (columnar transport) is a separate message, requested per sheet:

```proto
message PackedSheet {
  string sheet_id = 1;
  int32  layout_version = 2;          // owned here; bump on layout change
  int64  origin_x = 6; int64 origin_y = 7;  // sheet min corner; blob coords are relative to this
  bytes  vertices = 3;                // int32 LE pairs, sheet-relative: [x0,y0,x1,y1,...]
  bytes  primitives = 4;              // fixed-width records: kind,group,first_vertex,vertex_count
  repeated PrimitiveKey keys = 5;     // O(primitives): primitive index -> ref_des / net (for picking)
}
```

The reader emits tier 1 always. Tier 2 is generated for the scale path (per-sheet GPU upload) and shares the same underlying vertex data. Tier 1's `Point`, `Shape`, and `Polyline` are for reads and small designs. They are not the bulk carrier.

## Reader plan

- A reader exposes a geometry entry point that returns `*geom.SchematicGeometry`. The entry point is `ReadSchematic` in EDIF and `ReadSchematicGeometry` elsewhere. It reuses the format's parser and does not touch the netlist extractor.
- It parses into the tier-3 columnar Go form, then maps to tier-1 proto, and to tier-2 when the renderer needs it.
- Fidelity is lossy-bounded (a render subset). It drops the connectivity graph, properties, style palette, display metadata, and back-annotation, and keeps shapes, pin points, placements with transforms, wire polylines, labels, and sheets.
- The sidecar is heavy (about 62MB for the reference board), so it is produced server-side or by the CLI only.
- Join keys are `ref_des` plus `source_id` (the instance id), the `net` name, and `port_ref`.

## Renderer plan

- **The SVG verification backend came first.** A pure-Go `SheetSVG(geometry, sheet)` renders a sheet to SVG through a small zero-dependency element builder. It is the eyeball and golden instrument, not the production renderer. It proved the geometry model against the real 62MB board before any browser code existed. The proof covered symbol placement and orientation math, pins meeting wire endpoints, per-view (multi-section) symbol selection, and the annotation layer (pin numbers, title-block and symbol text, off-page connector net names). WebGL2 is a second backend over the same render layer.
- **The browser renderer is WebGL2 from the start**, not a per-sprite scene graph, because a scene graph does not scale to 100k+ vector primitives. Static geometry is uploaded once as GPU buffers from the tier-2 blob, and a small dynamic overlay (selection, hover) crosses per frame.
- **The viewer loads one sheet at a time** (a few thousand primitives), not the whole design. Server-side spatial indexing and culling are optional later additions.
- **Camera and picking are view-local.** Pan and zoom are an affine transform in the view. Picking uses a view-local spatial index over what was drawn and emits semantic intents (`ComponentSelected(refDes)`, `NetHovered(net)`). The presenter never sees pixels.
- **Renderer runtime is TS in-process**, which superseded an original WASM plan. A continuously dragged canvas is a high-frequency surface. The presenter pattern keeps such surfaces in TS to avoid per-event boundary cost, so the WebGL renderer and camera are plain TS. There is no Go/WASM presenter for the canvas. The server is still authoritative for parsing, geometry and style, and the client only draws.
- **A fallback view** draws a {{ explainable "netlist" }}-graph diagram derived from the IR alone (auto-layout) when geometry is absent. It synthesizes a `SchematicGeometry`, so the same two renderers draw it (see Auto-layout node drawing below).

## WebGL renderer as built

The WebGL path draws a schematic sheet as the SVG backend draws it, and its picks, highlights and area select match SVG's (agni issues 1330 to 1336). A board draws its zones as the same translucent wash (agni issue 1334). The visual diff draws in WebGL too, when the canvas does (agni issue 1335). Each side draws its revision's packed sheet and lights its changes over the keys it holds, and the union draws the new revision with the old revision's removed parts as ghosts. A ghost is the old sheet's matched primitives, lines as thin quads and fills as they are, moved by the difference between the two sheets' packed origins (`ghostOverlays` in `web/src/canvas.ts`). Each sheet is packed about its own origin, so the two sides mirror pan and zoom through the point at the middle in the design's own coordinates rather than the sheet's. WebGL2, upload-once, view-local camera, and per-sheet loading all held. The runtime and a few mechanics differ from the plan:

- **Join by embedded keys, not a separate IR load.** `PackedSheet` carries `PrimitiveKey{ref_des, net}` inline, so picking reads the key off the packed primitive with no second IR fetch.
- **Coordinates are Y-up end to end.** geom is Y-up and WebGL NDC is Y-up, so the camera matrix does **not** flip Y. An early bug did flip it, and the sheet rendered upside-down against the SVG oracle. The SVG backend flips once, only because SVG pixel space is Y-down.
- **Worksheet furniture is synthesized in the packer**, in world units, under a frame group. The furniture is the frame border, the zone-ruler ticks, and the title-block box and dividers. The packer draws it only when the sheet has a page, meaning `sheet.Size` is set. It mirrors the SVG worksheet drawing, and the frame extends the packed bounds so the camera fits the whole page. Ruler and title-block text is not geometry (see the overlay below). Both renderers size the title block, its text and its indent by one formula, in SheetSVG's drawing pixels (`titleBlockPx`), and the packer turns them into world units through the sheet's scale (agni issue 765). Before, WebGL's block was 32% of the frame's width and SVG's 36%.
- **Text draws as an SVG overlay.** A GPU line pipeline draws no glyphs. Labels travel on the wire as packed labels. Each carries a world position, height, rotation, justify and color, computed server-side with the same transform math as SVG. The client draws them as a second SVG layer over the canvas. Each `<text>` is placed once in a Y-flipped world space (`y -> -y`), so glyphs are upright. Pan and zoom are a single CSS transform on the layer, which the GPU composites with one style write per frame. Both layers read the same camera in the same `requestAnimationFrame`, so text is locked to geometry and never drifts. The layer is `pointer-events:none` and shows only in WebGL mode. Text metrics are browser-owned, so there is no glyph atlas. There is one font per sheet. A per-element font has to wait until the IR carries one.
- **Both renderers share one palette.** Colors and the default font are injectable data, resolved server-side and consumed by both backends. SVG draws from the style directly, and the WebGL side receives group colors (indexed by group), a background color, a font family, and a per-label color on the wire, so the WebGL code holds no palette of its own. `agni serve --theme default|dark` recolors both backends from one style. The two agree by construction.

## The geometry proto is a render contract

`geom.SchematicGeometry` decouples **producers** from **renderers**, and no one format owns it.

- The producers are the faithful readers (EDIF's `ReadSchematic` for `.eds`, and `ReadSchematicGeometry` in the KiCad, xschem and gEDA readers) and the auto-layout path (grid and layered).
- The renderers are the SVG backend and the tier-2 packer that feeds WebGL.

{{ includeFile "figures/geometry-producers-renderers.svg" }}

A new source plugs into the contract and every renderer inherits it, and a new renderer consumes every source. Adding the KiCad reader lit up faithful KiCad rendering on the SVG backend with no renderer change. There is a corollary. When two producers feed the same field into one renderer, they have to reconcile their conventions (see `justify` below). The shared contract surfaces the inconsistency instead of hiding it.

## KiCad coordinate conventions

KiCad uses two coordinate frames, and conflating them is the trap.

{{ includeFile "figures/kicad-coordinate-frames.svg" }}

- **Library symbol graphics are Y-up** (like the geom contract). Map lib-local points straight through (mm to nm, no Y flip).
- **The schematic sheet is Y-down.** Flip Y only for sheet-level coordinates: placement origins, wires, labels, junctions.
- **Rotation is negated** (`geom = 360 − kicad`). Converting the sheet frame to Y-up is a reflection, which inverts rotation direction. Mirror axes and origin translation are unchanged by the flip. If the reader also flips lib points, or does not negate rotation, every rotated symbol comes out mirrored or overlapped. An upside-down {{ explainable "ground" "GND" }} is the tell.
- Units convert from KiCad mm to nm (times 1e6, exact at KiCad's 0.0001mm grid), and `unit_nm = 1`.
- KiCad hides `#`-prefixed references, the `#PWR` and `#FLG` power and flag virtuals. They are not parts, so the reader leaves the placement's {{ explainable "reference-designator" "ref-des" }} empty and a power symbol carries its net in `net_anchor` instead (see [Symbols that name a net](#symbols-that-name-a-net)).

The cheap correctness signal for the transform math is the pin-on-wire coincidence rate. That rate is the fraction of placed pin connect-points that fall on wire endpoints. It is not sensitive to rotation *direction* on symmetric 2-pin parts (the pins just swap), so the asymmetric symbol bodies need an eyeball check too.

## OrCAD Capture coordinate conventions

Capture draws symbols and pages in one frame, Y down, in units of a hundredth of an inch, so `unit_nm`
is 254000. The reader negates every Y, the symbol's and the page's alike, so a quarter turn reads
anticlockwise in the contract with no change to its count.

A placement stores the corner of its symbol's box and a quarter-turn count, and Capture turns the
symbol inside that box. So a turn moves the corner the transform starts from. A connection point
(x, y) in a symbol w wide and h tall lands at (x, y), (y, w−x), (w−x, h−y) or (h−y, x) from the
stored corner, for 0 to 3 turns. The page also stores every placed pin's own position. So the
reader's transform is checked against the file itself. `TestEveryPlacedPinLandsWhereThePageStoresIt`
puts each of Clay R6's 573 pins through `geomath.PlacePin` and needs each to land exactly where the
page says. A mirror bit is read as the format notes describe it, flipping x to w−x before the turn,
and no public design checks it, because Clay R6 places nothing mirrored.

A power symbol, port or off-page connector stores an anchor as well as a box, and the two can differ.
A supply's anchor sits above its box, so the symbol is placed from the box's corner, never the anchor.
`TestEveryPowerSymbolAndPortConnectsAtAWireEnd` holds each one's connection point to a wire end or a
pin on its page. A title block stores its position from the page's bottom-right corner.

Capture stores no junction dots, so the reader draws one wherever three wire ends meet or a wire ends
on another's span. It draws a cross on every pin the page flags not connected, as Capture does, and
Clay R6 flags eight. A block's pin names sit inside the block along their edge, and a name on the top
or bottom edge is written at 270, reading upward as drawn. The renderer turns a label at 90 the
other way up, which once ran those names out of the block. A part shows its pins' names and numbers as the last two bytes of its cached
record say. Bit 0 shows the names and bit 2 hides the numbers, so a resistor shows neither.

## Render-fidelity fields

These were added to the contract so a reader-produced sheet renders like the source tool.

- `SheetGeometry.shapes` holds free sheet graphics not owned by a symbol (junction dots, no-connect markers, notes). KiCad fills it from junctions, no-connects and graphics, and EDIF fills it from comment graphics.
- `Shape.fill` (`UNSPECIFIED` / `OUTLINE` / `BACKGROUND` / `COLOR`, plus `fill_color`) makes solid symbol bodies render filled. It is not a bool, because KiCad distinguishes fill types. The placement transform must propagate it.
- **`justify` has one canonical form**, `"<h> <v>"` (h left/center/right, v top/middle/bottom). Each reader maps its native codes (EDIF `LOWERLEFT` to `"left bottom"`, while KiCad tokens are already canonical). The SVG backend applies `text-anchor` (horizontal) and `dominant-baseline` (vertical).
- `SymbolPlacement.fields` and `PinPoint.name` put instance text (Reference, Value, custom) on the placement as structure, rather than leaving it as loose sheet labels. Each field is a `Field{name, value, origin, justify, visible, ...}`. A consumer then knows which text is which field, and that helps the visual diff. Sheet labels are then only genuine free text (net labels, notes).

## Symbols that name a net

A ground glyph, a rail symbol or a hierarchy port is drawn like a part and is not one. Its job is to
give the net at its pin a name. `SymbolPlacement.net_anchor` carries that name, and a placement that
sets it leaves `ref_des` empty. A ref-des on such a glyph would join to no `ir.Component`, so a
viewer could select it and then ask about a part that does not exist. An ordinary component
placement leaves `net_anchor` empty. Every schematic format has the construct and spells it
differently, so the contract holds one neutral field and each reader translates its own spelling
into it (CONSTRAINTS C1). No consumer needs to know how any format writes a power symbol.

| Reader | Anchored placements, and where the name comes from | Left unanchored |
|---|---|---|
| KiCad | a `#`-prefixed reference (`#PWR`), named by its `Value` field | `PWR_FLAG`, which asserts a net is driven and names nothing |
| xschem | a label symbol (`gnd`, `vdd`, `vss`, `lab_pin`, `lab_wire`, `ipin`, `opin`, `iopin`), named by its `lab` property | every other symbol |
| gEDA | a power symbol (`gnd`, `vcc`, `vdd`, `vss` and the `<rail>-plus-N` and `<rail>-minus-N` rails), named by the instance `net=`, else the symbol's own `net=`, else the conventional supply for the family (`GND`, `VCC`, `VDD`, `VSS`) | every other symbol |
| EDIF `.eds` | a placement with no designator whose cell name, or one of whose field values, is a net on the same sheet | a no-connect, a sheet port, and every placement that has a designator |
| OrCAD Capture `.DSN` | a power symbol, port or off-page connector, named by the net of the first wire that ends inside its stored box | one that touches no wire reaching a pin |

The anchor is a net name and joins the netlist by name, the way `WireGeometry.net` does. Each
reader takes it from the same fact its netlist side uses, so the glyph's key and the net it joins
cannot disagree. KiCad's netlist read turns the same `Value` into a rank-0 name anchor (see
[KiCad connection-point semantics](../net-solving/#kicad-connection-point-semantics)). gEDA's
geometry resolves the name through the same three-step fallback as `resolveAnchors` in the netlist
read, and the two must change together. For xschem, `read.go` anchors the `lab` name at the
symbol origin.

EDIF works by matching rather than by a table, because the spellings are inconsistent even within
one export. On one measured `.eds`, 3329 of 6102 placements carry no designator. Across five real
schematics, `GND` and `DGND` glyphs carry no fields and are named by their cell (5772 placements,
every one matching a net on its sheet), while `PWR` and `PWR_2` carry the name in a "Global Signal
Name" field (1666 placements). A table of cell names would have caught the first group and missed
the second. The match also classifies. A no-connect (1653 on the first export) asserts a pin is
open and names nothing, so neither its cell nor its fields match a net and it gets no anchor. Sheet
ports carry no name anywhere. There are 4752 of them, as `in_flat` and `out_flat`. Each port takes
its name from the wire it touches, so the reader leaves ports anonymous rather than guessing. A
placement with a designator is never anchored, so a part is still a part when its value happens to
equal a net name.

The SVG backend keys an anchored placement's graphics as its net (`data-kind="net"` and
`data-net`). A wire carries the same key, so clicking a ground symbol selects its net. It emits no pin
pick target for an anchor, because that target would carry an empty ref-des and resolve to nothing.
The tier-2 packer keys an anchored placement's primitives the same way, by `net` with no `ref_des`,
and its pins with nothing, so a WebGL click on a ground symbol selects its net too (agni issue 764).

## Text stays readable and inside its box

Readers carry a source's text orientation and sizing faithfully. The render layer makes that text legible. The SVG backend and the WebGL overlay share that layer, so the two agree by construction. These rules live in the render layer, and both backends apply them.

- **Text is drawn upright**, the way every EDA viewer (Eeschema, Altium, OrCAD, and the tool that authored the EDIF) draws it. Some runs would read upside down, meaning the normalized magnitude of their angle is over 90. A symbol placed at 180 degrees does this, and so does a net label with its own 180-degree orientation. Such a run is turned a further 180 and its justify is flipped on both axes, so it is still anchored to the same corner. Vertical text (plus or minus 90) is left alone, so the KiCad-90 parity holds. Without this, 180-degree-placed connectors on a real headers sheet rendered their ref-des and half their net-stub labels upside down.
- **A caption is condensed to fit its box, never shrunk.** A symbol caption with no source text height fell back to a fixed size and spilled past its symbol. Now, when it would overflow, it is condensed horizontally to the drawn body-box width through SVG `textLength` with `lengthAdjust="spacingAndGlyphs"`. The body-box width is the widest boxed rect, not the bounding box the pin stubs widen. Condensing keeps the font height instead of dropping it to a few pixels. The packed label carries the budget to the overlay so it condenses identically.

- **Every size comes from the drawing.** No text is sized from a constant. A run with no source height falls back to the sheet's own median text height (`defaultTextHeight`), shared with the WebGL path so both backends draw height-less text the same. The legibility floor and the absurd-height ceiling are **fractions of the drawing**, not pixel counts, because a bound is a claim about how much of a sheet one run may occupy.
- **A justify anchors the whole block, not its first line.** A multi-line run stacks *upward* from a bottom anchor, both ways from a centered one, and downward only from a top anchor. It matters beyond looks, because a tool that bottom-anchors its notes places the NEXT note relative to that same bottom.

<details>
<summary>The measurements behind those two rules</summary>

A flat 40px ceiling failed in both directions. On a sparse auto-layout, where the drawing zooms until symbol bodies are ~444px tall, it pinned every net label to 9% of the body it labels where the layout asked for 33%, and on a dense faithful sheet it sat at 2.5% while a real title measured 2.2%, a hair from truncating it.

One export spaced a 3-line note and the note below it exactly two line pitches apart, which prints as a blank line. Growing the first downward put its last line 2.10px from the second note's anchor, where a blank line at that font is 21.4px.

</details>

Beware that `rsvg-convert` (librsvg) silently ignores `textLength` and `lengthAdjust`, so an offline PNG of the SVG backend still shows a caption overflowing. Browsers honor it, and the viewer's SVG and WebGL output is browser-consumed, so it is correct in the product. Verify condensing in a real browser, not via an rsvg PNG or a golden.

### The font is a name, never a file

Schematic formats do not record a font. One EDIF export names 24,523 text heights and zero fonts, typefaces or point sizes. So "use whatever the source uses" is not available. A PDF of that design is a *different program's* rendering, and its face was that program's choice.

`Style.Font` is therefore a CSS family list resolved by the viewer, and the engine loads and ships no font file. It leads with the face the authoring tools print in and falls back to a **metric-compatible** substitute, so a machine without the first lays text out at identical advance widths rather than drifting. Family names in it are **single-quoted**, because `core/svg`'s `Attr` writes its value verbatim inside double quotes, so a double-quoted family name emits invalid XML on the SVG root element.

### A source text height may not be an em size

`Label.height` and friends mean **glyph height**, and the renderer maps it straight to `font-size`. A format that spells text size differently is converted **in the reader**, because that is a fact about the format rather than a rendering preference.

In EDIF, `textHeight` is a **line pitch**. The authoring tool stacks a component's field rows exactly `textHeight` apart. Over 2377 instances of one export, 3161 of the same-column row gaps were exactly 1.000x that row's `textHeight`. So mapping it onto `font-size` leaves zero leading and consecutive rows touch. `readers/edif` divides by the tool's line height to recover an em. KiCad states a glyph height directly and is correctly left alone.

### A pin's number is a join key, not a caption

`PinPoint.port_ref` holds the **physical pin designator**, joining to `ir.Port.designator`, and `core/render/pack.go` feeds it to `PrimitiveKey.pin` so a finding can target one pin. If a reader puts a port *name* there, it draws the wrong text and breaks that join. That is one bug with two symptoms.

Formats differ in where they keep the two. KiCad states the number directly on the pin. EDIF splits them, with the `portImplementation` naming the PORT and the number living on the cell interface's `(port X (designator "N"))`. A pin also carries a text height like any other run, and may state **two** independent text positions (the name on its own display, the number on a `keywordDisplay designator`), so `PinPoint` carries both `label_origin` and `number_origin`.

KiCad states neither position, so its reader lays them out by KiCad's own rule (agni 1344). The symbol's `(pin_names (offset N))` decides it. With an offset above 0, the name sits N past the pin's inner end, inside the body, and the number sits above the middle of the pin line. With an offset of 0, the name sits above the middle of the line and the number below it. A vertical pin's text runs along the pin, so `text_rotation_deg` is 90. `orient-jetson-fet.kicad-text.txt` holds kicad-cli's own placement of every pin run on two Jetson parts, and `TestPinTextSitsWhereKiCadDrawsIt` holds both to within 0.1 mm.

**Pin text is placed by direction, never by adding angles.** A placement's `rotation_deg` is counter-clockwise, and the angle `drawText` takes is clockwise on screen. A mirror also reverses a run. So `pinTextAt` in `core/render/pintext.go` carries the run's direction and its up through the placement transform, turns the run to read left to right or bottom to top, and reflects a run beside its pin line back onto the side the symbol drew it on. Both backends call it. A pin number above its line is then still above it on a part turned 180 or mirrored, as KiCad draws it.

### A body fill draws under the lines

A KiCad shape's fill type is the file's, and its colour is the theme's. `background` means the theme's symbol-body colour, which is `Style.ShapeFill` here and pale yellow in KiCad. KiCad paints every body fill on a layer under the symbol's lines. So the reader puts a symbol's background-filled shapes ahead of its other shapes, keeping file order within each group. A transistor whose body is a filled circle otherwise covers the channel drawn inside it. A two-point line encloses nothing, so the reader drops its fill whatever type it names.

### A KiCad label's text sits off its wire

KiCad keeps schematic text readable, so a label at 180 reads left to right and one at 270 reads bottom to top. The file's justify is already the justify of the run as read, so `(at x y 180)` with `(justify right)` ends at its anchor and runs left. The reader therefore hands the renderer 0 or 90, never 180 or 270. Handing it 180 turned the run upright a second time, flipped that justify, and ran the label across its wire.

KiCad also draws a label's text a little away from where it meets its wire, and the distance depends on the label's kind. A local label is lifted by its text offset plus the pen, 0.27 heights. A hierarchical label or a sheet pin moves along its reading direction by one text width plus the offset, 1.15 heights, to leave room for its arrow. A global label moves along it by a margin of 0.375 heights, plus three quarters of a height when its flag has an arrow at the wire end. The reader places `Label.origin` where the text goes and keeps the wire point in `Label.anchor`, so a consumer joining a label to its wire reads `anchor` first. `orient-labels.kicad_sch` puts every kind at every angle, and `TestLabelTextSitsWhereKiCadDrawsIt` holds each run to kicad-cli's export.

A global label is drawn inside a flag whose ends show the signal's direction (`Label.flag`). The renderer draws the outline from `anchor` with KiCad's shape, and sizes its length from its own text width rather than KiCad's stroke font, so the box fits the text a reader sees. `TestGlobalLabelFlagsStartWhereKiCadDrawsThem` holds the wire end of every flag to kicad-cli's corners.

### A sheet pin's arrow sits inside its sheet

KiCad draws a sheet pin as the hierarchical label it stands for, seen from the parent sheet. So its arrow is the label's arrow turned a half turn, inside the sheet's box, with input and output swapped, since a signal entering the sheet leaves the parent. The reader draws it as a sheet shape beside the pin's name, and keeps a dot only for a pin type that KiCad draws no arrow for. `TestSheetPinArrowsMatchKiCad` holds ten pins, on all four sides, to kicad-cli's outlines.

### KiCad text markup draws, and the text stays as written

KiCad writes `~{RESET}` for RESET with an overbar, `V_{OUT}` for a subscript and `I^{2}C` for a superscript, and `{slash}` and its siblings for a character a name cannot hold. The reader leaves the text as written, because a net label's text is also the net's name, and sets `SchematicGeometry.text_markup` to `kicad` instead. The renderer splits each single-line text into runs (`markupRuns`). The SVG backend writes them as `<tspan>`s, an overbar as `text-decoration="overline"` and a subscript or superscript at 70% off the baseline. The packer sends the runs in `PackedLabel.runs`, so the overlay draws them the same way without parsing the markup itself. A multi-line text prints as written in both backends. A geometry from another format leaves `text_markup` empty, so a brace in its text prints as a brace.

### A placed pin can pick its alternate name

A KiCad pin can carry alternate functions, and a placed part picks one with `(pin "G39" (alternate "CAN0_DIN"))`. kicad-cli draws the picked name where the library's would go. The library's name lives on the shared `SymbolDef`, so the reader puts the pick on the placement, in `SymbolPlacement.pin_names` keyed by pin number, and both backends draw it in place of the symbol's name.

### A do-not-populate part is crossed out

`SymbolPlacement.dnp` marks a part that the schematic draws and the board leaves unfitted, read from KiCad's `(dnp yes)`. Both backends draw KiCad's red cross over the part's body, which is its artwork without the pin legs. The netlist keeps the same flag in the part's attributes, where `component.dnp` reads it.

## Board geometry sidecar

There are two geometries and two sidecars, one per physical medium.

- **Schematic-page geometry** is `SchematicGeometry`, the subject above. It covers symbol shapes, wire runs and labels.
- **Board geometry** is `BoardGeometry`, in the same `agni.v1.geom` package. It covers component placement, pads, routed copper, {{ explainable "via" "vias" }}, zones, the layer stackup and the board outline. The two share a package, so the primitive vocabulary (`Point`, `Polyline`, `BBox`, `Provenance`) is shared rather than duplicated.

The board sidecar follows every rule established above. It is a separate keyed artifact, never imported by diff, rules, or simulation. It joins to the netlist IR at consumption time by stable keys. Placements join by `ref_des`. A pad's `number` matches the connection's `pin_ref`, so `(ref_des, number)` joins a copper land to its netlist pin. Routed copper joins by net name, through `NetCopper`, the board analogue of `WireGeometry`. The sidecar holds no schematic net id. A served board gets one per net at render time instead, from the design's netlist through `render.WithNetIDs` (agni issue 1440), so a highlight naming a net by id joins the board as it joins a schematic. The first producer is the KiCad reader, over the same s-expr parse as the netlist reader. IPC-2581 is the second producer and ODB++ the third, both described below. Both sit behind the same proto, which keeps the one-contract, N-producers property.

**Silkscreen and legend text travel in `BoardText`**, which carries each {{ explainable "footprint" }}'s ref-des and value plus free graphic text such as the title block, so both board renderers draw them, matching what KiCad shows. Text is universal across board formats (IPC-2581 legend, ODB++, Gerber), so it lives in the shared contract, not in a reader, and the IR never overfits one format. Every `BoardText` is board-frame absolute. The reader composes a footprint's local text offset through the placement transform, with the same composer as the pads, and that composition pins text to its part. Free text is authored absolute already. Glyph angle is absolute too, because KiCad saves a footprint text's angle as drawn rather than relative to its footprint, so the footprint's own angle never enters it. That angle folds under KiCad's default keep-upright, so text on a rotated footprint never renders inverted. Unlocked text and free graphic text are exempt, so a deliberately mirrored back-side title is still mirrored. Hidden source text is dropped. **A board text draws as KiCad draws it** (agni issue 1366). Its size is its glyph height, so both
renderers draw the em at `height / capHeightEm`, 0.716 for Arial, and it takes its layer's colour.
Each side's silkscreen, fab and courtyard layer has a colour of its own, as KiCad's do, so the
back's legend reads apart from the front's, and a footprint's outline takes the same colour as
its text. Both styles use the colours of KiCad's default theme, read off kicad-cli's export. The
light style darkens the few that fall under 1.5:1 on its page to 2:1, in the same hue, since
KiCad's pale front silk all but vanishes there. Every reader names its layers in KiCad's vocabulary, so an ODB++, Allegro or IPC-2581 board
takes the same colours. A footprint field that gives no
size is 1.27 mm, and one that names no layer is a Reference on F.SilkS or a Value on F.Fab, even on
a back footprint. kicad-cli is the oracle. `hack/kicad_text_oracle.sh` writes how it strokes each
text on the fixture board, gateway and Jetson, and `TestBoardTextAgainstKiCad` holds the reader to
it. A footprint's user text draws too, with `${REFERENCE}` and `${VALUE}` filled from its fields,
since most libraries put the designator inside a part's outline that way. Some footprints carry a
placeholder reference such as `REF**`, as a logo or a panel tab does. Such a footprint is no part,
but its artwork and user text still draw, owned by no part. **A KiCad text runs as long as KiCad's
stroke font runs** (`core/render/strokefont.go`). The reader marks the text `font: kicad_stroke`
and records its pen width. The renderer holds each glyph's advance and bearings, measured off
kicad-cli, and moves the run to where KiCad's ink starts. Both renderers then stretch Arial to the
ink's length, through `textLength` in the SVG and `PackedLabel.width` in the viewer.
`TestBoardRunsAgainstKiCad` holds both ends of every run to the same oracles, within 0.1 mm. A text
of several lines draws one line at a time, 1.61 heights apart, with the block centred on its anchor.
**Back-side text draws mirrored** about its anchor in `BoardSVG` (agni issue 1327), as KiCad draws it, because seen from the front it sits on the copper reversed. A board view turned to the bottom view then reads it the right way round. The WebGL board text does not mirror yet (#1368).

**Silkscreen and fab graphics travel in `BoardGraphic`** the same way, which carries a footprint's silk and fab body outlines, courtyards, and polarity marks, plus free graphics that are not the board edge (the edge is still `BoardOutline`). It reuses `geom.Shape` for the geometry itself (polyline, rect, circle, with arcs approximated to polylines under the same lossy bound), adds only a stroke width, carries the source layer verbatim, and, like `BoardText`, pre-composes footprint graphics to board coordinates so they sit on their part. Both renderers draw them in a silk group, and per-layer visibility is the same client-side concern as the copper strata. This is universal across board formats, so it is a shared-contract field rather than a reader's, and a second producer fills it too. Filled zone regions and per-side silk and fab default-visibility remain a later refinement.

The board's tier-2 form is `render.PackBoard`. It projects a `BoardGeometry` into `PackedSheet`, the same envelope as the schematic, and adds board layer groups and a filled-triangle primitive kind. Each copper layer, each copper layer's zones and each graphics layer packs into a group of its own, numbered from 13 up, and `render.BoardLayers` lists which groups make up each row a viewer can hide (agni 1351). Copper takes the colours of KiCad's default theme. KiCad colours an inner layer by its number, so `InN.Cu` takes entry N of `Style.CopperInners`, and a board's In5 draws in In5's colour whichever inner layers it has. An inner layer named otherwise takes the entry for its place among the inner layers (agni 1401). The SVG board, the packed groups and the rows' swatches all read it through `copperColors`, so each inner layer reads apart in both renderers. A board's silkscreen text carries its layer's group in `PackedLabel.group`. A zone fills as a wash of its layer's copper colour at `zoneWashOpacity`, as the SVG board draws it. The packer triangulates its outline into the zone's own group, and `PackedSheet.group_alphas` gives that group its opacity, so overlapping zones darken in WebGL as in SVG (agni issue 1334). A zone past the group limit shares its copper's group and keeps its outline. The web canvas then decodes both through one path. Coordinates are nanometers (`unit_nm=1`), Y-up, matching the KiCad schematic reader's convention. Rotations are carried verbatim from the source, and composition with the Y-flip is the renderer's concern. Fidelity is lossy-bounded to a render and DRC subset, which leaves out zone fill polygons, teardrops, and 3D references, keeps zone outlines as authored, and approximates outline arcs as polylines. It gives the geometric DRC class (clearance, width, annular ring) its data tier, and a board viewer its contract.

An arc track stays an arc (agni issue 1358). A `TrackSegment` that sets `mid` is the circular arc through `a`, `mid` and `b`, the way `geom.Shape` spells an arc, and a full circle arrives as two halves. KiCad writes the mid point itself, and ODB++, IPC-2581, Altium and Allegro store an arc by its center, so those readers compute the mid point with `geomath.CenterArc`. The SVG draws the curve as a path through the mid point. Every other consumer reads a segment through `geomath.TrackPath`, which returns the two ends of a straight run and the chords of an arc, each spanning at most a tenth of a radian. Those consumers are the packed transport, the copper raster and the check model's `BoardSeg` list, so the width and clearance rules measure the curve. Reading only `a` and `b` would measure the chord across it, so `TestEveryTrackSegmentConsumerSeesArcs` fails on a new consumer that ranges over the segments without `TrackPath`. `geomath.TrackLength` gives the length along the curve.

A pad's shape can differ by layer, so a board carries a padstack table (agni issue 1356). A through-hole pad is often larger on the outer layers than on the inner ones, and a plane layer clears an antipad around a hole whose net it does not join. `BoardGeometry.padstacks` lists each padstack's land on every copper layer, and a `Pad` or `Via` names its entry in `padstack`. The entry's layers are the board's own layers as placed, so a part on the bottom names an entry of its own (`C40D32@bottom`) and no consumer turns a padstack over itself. Allegro stores padstacks, and its reader also fills each layer's thermal relief and antipad. IPC-2581 and ODB++ store each layer's lands instead, so their readers gather a pin's lands across the layers. ODB++ names no padstack, so its entries are numbered. Thermal and antipad are provisional under C9, because IPC-2581 and ODB++ draw both as cut-outs in the plane polygon, which agni issue 1357 reads. The renderers draw each land in its own layer's group, so a reader who shows one layer sees that layer's lands at their own size. They draw an antipad as a dashed outline on a plane layer whose zones do not carry the pad's net. A pad or via with no padstack draws as before, from its own shape. A land can sit off its pin, and a custom land has an outline (agni issue 1407). `PadShape.offset` and `PadShape.outline` are in the pad's own frame. A pad's angle is in its footprint's frame, so a land turns with the pad and then with the part. `geomath.PlaceLand` composes that, and the SVG renderer, the packer and the copper raster all draw through it. KiCad and ODB++ write a pad's board angle, and their readers subtract the part's angle on the way in. Allegro stores a land's offset and a custom pad's shape in its padstack. IPC-2581 writes both in the package pin's primitive. KiCad 9 writes a padstack per pad, with a shape for the inner layers or for each named layer. `TestAnOffsetLandSitsWhereTheExportPutsIt` holds parasdr's offset lands to the board positions its IPC-2581 export writes, on both sides of the board. Allegro wrote parasdr's board, its IPC-2581 and its ODB++X, and `TestPinLandsAgreeAcrossParasdrsReads` holds the three reads to one another pin by pin. `TestParasdrsCopperAgreesAcrossItsReads` holds both exports' tracks, vias, pad nets and drills to the board itself (agni issue 1383). Allegro's IPC-2581 writes a track's width inline, holds each drill hit in a Set with no net, and names its drill layer by the copper span it joins (`IPC2581DRILL_1-6`), and its ODB++X leaves a via's `USAGE` in force over the tracks that follow it, so each reader handles those shapes. The IPC-2581 export draws every land the padstack defines, and the ODB++X export leaves out an inner land that nothing on its layer joins, as Allegro's films do. The BeagleBone AI's padstacks are held to the extract Allegro wrote (`TestAllegroPadstacksMatchTheExtract`).

A board's filled copper is a `CopperShape` under its net, beside the tracks and vias (agni issue 1357). A shape is the metal as etched: an outline, and each hole a region inside it with no copper. A plane is one shape whose holes are its clearances and the gaps between its thermal spokes. A `Zone` is something else, the region a designer drew to be filled, whose outline overlaps other nets' tracks, so a consumer that wants copper reads the shapes and never the zones. Allegro stores its fills as ETCH shapes with their voids, Altium as regions in `Regions6`, IPC-2581 as contours with cutouts and ODB++ as surfaces with holes, and their readers keep them. The foxBMS boards' `.PcbDoc` reads hold to Altium's own ODB++ exports shape for shape and area for area (`TestAltiumPoursMatchTheirODBExport`, agni 1410). The query layer sees both views: `board.copper(net, layer, kind)` for tracks, vias and pours as etched, and `board.zone(net, layer)` for the zones drawn. KiCad recomputes a fill from the zone whenever it fills, so its reader keeps the outline alone; DECISIONS.md records where that line is drawn. The SVG draws a shape as one path with an even-odd fill, in its layer's group, under the tracks. The packer triangulates it with earcut (`geomath.Earcut`, a port of mapbox's), so WebGL fills it the same way. The Sheets panel's Pours row hides every layer's shapes. A KiCad zone on several layers reads as one zone per layer. The copper raster draws the shapes, and its Gerber reader now fills regions and erases with clear polarity, since Allegro plots a plane as a region and cuts its voids with clear flashes. With both, `TestAllegroCopperMatchesItsFilms` finds agni drawing all of every film's copper.

**Both renderers draw board copper at its true physical width**, floored to a *physical* minimum (about 25µm in board space), never to a fixed output-pixel constant. The SVG backend strokes `max(width, minStroke) * scale`, and the WebGL packer tessellates the same floored width. An earlier output-pixel floor of a fixed 0.8px clamped every sub-pixel trace to one width on a scaled-to-fit board. Dense copper merged into a blob, and relative trace widths disappeared. The board-space floor keeps thickness proportional to the copper at every zoom, as EDA viewers draw. WebGL was already faithful, because browser GL line width is about 1px, so the packer already drew tracks as triangle quads rather than lines and only the SVG backend needed converging. The scope is board copper. Schematic wire strokes keep a fixed pixel width (line-art at readable zoom, no blobbing), and pad and via size-floors are unchanged (discrete-feature visibility, a separate concern).

**{{ explainable "bus" "Buses" }} draw distinctly.** `WireGeometry.kind` (unset = wire, `KIND_BUS`, `KIND_BUS_ENTRY`) lets the readers flag a bus trunk or entry so both renderers style it apart from a net wire. The SVG backend strokes it thicker in the bus color, and the WebGL packer tessellates it to true-width triangle quads in a distinct bus group with the same "GL lines are about 1px, so widen via quads" path copper takes. The kind is format-neutral. KiCad sets it today from `bus` and `bus_entry`. A bus carries no net, so its member nets are unmodeled.

<details>
<summary>How a bus finding highlights its trunk, and what happens when the bus is not drawn at all</summary>

The packed palette only grows to cover the bus group on a sheet that actually packs a bus, so a bus-less sheet's bytes are unchanged. On click, a `bus-not-modeled` finding highlights its drawn trunk by recoloring the outline in place. The highlight is keyed by the bus name and gated on the bus kind, so a same-named net wire is not caught. It recolors in place rather than taking the net-focus path, since a bus is already thick and the net-focus path shape would mis-tessellate the WebGL quads. The reader names the bus wire from its source label (KiCad's range-label-on-wire, with gEDA and xschem inline). That label is also the finding subject, so the join is by name.

Some buses are undrawable, such as a bus alias, an EDIF `array`, or a hierarchical port with no drawn wire. An undrawable bus shows a server-authoritative "bus not drawn" note instead, computed from the drawn-bus-name index.

</details>

### IPC-2581 as the second producer, and the contract's first second-format audit

The IPC-2581 board reader is the second `BoardGeometry` producer. The proto was designed from one producer, KiCad. So the second producer also audits whether the fields are overfit. The overfit rule only applies once a second vendor's data goes into those fields. Every outcome of the audit was resolved without a proto change.

- **It added no new fields.** IPC-2581's stackup carries per-layer material and thickness, but no board rule or renderer consumes them, so `BoardLayer` keeps only `kind` (IPC-2581's `layerFunction` verbatim, the same discipline as KiCad's `kind` word). The richer stackup is in the netlist IR's `ir.Stackup`, ledgered until a consumer needs it.
- **Padstack def and instance resolve to the same inline `Pad`.** IPC-2581 references pad shapes by id from a primitive dictionary, where KiCad inlines them per footprint. The producer resolves the indirection and emits the identical flattened `Pad` (shape word, size, footprint-local position), confirming the inline mapping holds for a def-and-instance source.
- **The frame needs no flip.** IPC-2581 is Y-up like the geom contract, so coordinates map directly with **no** Y negation (KiCad is Y-down and negates). Sides normalize into the contract's KiCad-style vocabulary (`TOP` to `F.Cu`, `BOTTOM` to `B.Cu`) so the format-neutral renderer's front and back classifier works unchanged, the N-producers-one-vocabulary property in miniature.
- **The board-availability gate was generalized.** The check layer's `board.` gate keyed on `SourceFormat == "kicad-pcb"`, and it now tests a set of board formats. The authoritative per-design gate remains the Model's board tier: empty means the board rules are silent.

The scope merged in two parts. Placements, pads, layers and outline came first. Routed copper and vias came second. A track's interleaved straight and arc steps are decoded in document order, and each arc step becomes one arc segment. A via is a drilled hole marked as a via, with the co-located copper pad as the annular. With both parts, all four board DRC rules run on IPC-2581 (track-width, copper-clearance, hole-size, annular-width).

**The second producer now matches the first.** At first the IPC producer lagged the contract and left empty some fields that KiCad filled. The gap closed entirely into existing fields, with no proto change.

**Two render-faithfulness bugs surfaced on real Allegro exports**, and both passed unit fixtures and showed only on a corpus PNG render. The general lesson holds for any second producer. A construct can be copper on one layer and drawing geometry on another, so classify by the source layer function, not by the element name.

<details>
<summary>Which fields closed the parity gap, and what the two bugs were</summary>

Component value goes to `ir.Component` attributes. Silk and fab graphics (marking, outline, assembly-drawing packages, composed per placement) go to `BoardGraphic` (IPC encodes silk as vector geometry, not string text, so `BoardText` is legitimately empty for this producer, a genuine format difference). Copper plane and pour fills go to `Zone` (authored outline, cutouts dropped under the lossy bound). Via layer spans go to `Via.layer_from/to`. User-primitive pads (their own units) become real pad extents. The work pinned two gotchas. The fill is nested under a features contour, not a direct child. A fixture test passed while the real board rendered zero zones, and only the corpus-render rule caught it. A drill layer's span lives on the layer, not the hole. Non-via copper, fill cutouts, and the padstack def-and-instance indirection remain ledgered (no consumer yet), and stackup materials are a later refinement.

Cadence Allegro writes `clockwise="TRUE"/"FALSE"` in uppercase, and a case-sensitive `== "true"` read every clockwise arc as counter-clockwise, so it swept the long way and ballooned outline and copper arcs. Parse it case-insensitively. Second, a `Zone` is a copper pour, so the zone extraction must gate on the layer being a copper function (conductor or plane). Without the gate, document (fab and assembly-drawing) contours leaked in as copper and blew up the render bounds.

</details>

### ODB++ as the third producer

The ODB++ reader (`readers/odb`, agni issue 1250) fills the same contract, again with no proto change. A job is a directory, so the reader takes an `fs.FS` rooted at it rather than one `io.Reader`, and the registry enters it by its `matrix/matrix` file (or a `.tgz` or `.zip` of the whole tree). Four points about it are worth knowing before changing it.

- **A feature's net comes from eda/data, never from geometry.** A features file draws copper with no net on it. eda/data lists each net's subnets and, under each, the features by layer and index (`FID C <layer> <index>`). A toeprint subnet is a pad, and a trace, via or plane subnet is routed copper. The reader keeps every record's index, a barcode's included, because eda/data points at that index.
- **A pad's footprint-local offset is computed, not read.** The components file gives each pin's board position, so the reader inverts the contract's `at + M(R(rotation) * offset)` to get the offset. A pad then composes back onto exactly the point the job put it at, whatever angle convention its producer used. ODB++ angles are clockwise and are negated into the contract's counter-clockwise.
- **Layers are renamed into the KiCad-style vocabulary**, as IPC-2581's are. The first copper layer is `F.Cu`, the last `B.Cu`, and the ones between `In1.Cu` upward. A silkscreen, mask or paste layer takes its side from where the matrix stacks it.
- **A pad eda/data says little about is found by where it is** (agni issue 1268). Sometimes a toeprint's subnet names no features at all, as when Altium points every pin of a shield named `SH` at the first one. Such a toeprint takes the copper pad at its position. A non-plated hole sits on a drill layer that eda/data never ties to a pin, and it takes the drill hit there. A custom land is a surface rather than a pad feature, and takes the surface's extent with the shape `custom`. If a toeprint names only paste or mask features, it is an opening rather than a land, and it takes no copper. Otherwise a stencil window over a thermal pad would take the pad's copper.

The oracle is KiCad's own exporter. The same board read from its `.kicad_pcb` and from `kicad-cli pcb export odb` has to agree (`TestODBReadsAsItsKiCadSource` over the fixtures, `TestODBChecksAsItsKiCadSource` for every finding, and `make odb-oracle` over the six samples boards against `readers/formats/testdata/odb_oracle.baseline`). Every pin's net, every pad's position and every track and via agree to within 2 microns, and the baseline holds what the exporter itself changes (a footprint marked exclude-from-position left out, a duplicate designator renamed). The cross-check found three faults in the KiCad board reader, since fixed. It dropped arc tracks (2,316 on Jetson, agni issue 1260), read a slotted drill's `oval` keyword as its size, and turned every footprint by the negated angle (agni issue 1261). The last fault put pad 1 where pad 2 is on a front part at 90 or 270 degrees. It misplaced the pads of every back-side part too, except the ones that happened to be symmetric. KiCad stores a flipped footprint's items already mirrored, so the reader now un-mirrors them into the contract's footprint-local frame and lets the composer mirror once.

Altium Designer writes foxBMS's boards, and they give the reader jobs from another tool (the `odb-altium` samples artifact, in the gate). Altium names each pin with its part (`J500-1`), groups features in `FGR` records that are not nets, and writes no `misc/info`, so the reader strips the designator, skips the groups and names a job for its folder or archive. With no netlist of those boards to diff against, `TestODBAltiumJobsHoldTogether` holds each job's two tiers to each other instead.

## Auto-layout node drawing

The netlist-graph fallback (`agni render --layout=grid|layered`) has no source geometry, so it synthesizes a `SchematicGeometry` from the IR. The assembly step decides what to draw at each component node through a pluggable **`SymbolSource`**, so the layout is fixed while node artwork varies.

- **`Registry`** draws classified synthetic glyphs, keyed on open string class ids each with a hand-authored glyph (resistor, capacitor, inductor, ferrite, diode, led, tvs, fuse, connector, test point, crystal, ic, transistor, ground). Multi-pin bodies (ic, connector) carry no per-pin terminals, so their edges attach at the node center like the box does.

  **The glyph follows the class the ingestion passes stamped.** `classify.Stamp` fills
  `ir.Component.device_classes` on every read, from part text, the project's own `lexicon.class`
  patterns and the refinements a glob cannot express, and `classify.StampClassesFromSpecs` adds what
  only a datasheet can establish where the read carries a params corpus. The drawing reads that
  rather than deciding again, so a part the engine calls a `tvs` is drawn as a TVS, and a query answer
  and a picture of the same board cannot disagree about what a component is (agni issues 701 and
  710). Where the set holds more than one class the drawing takes the head of
  `classify.BySpecificity`. `check.Model` resolves `component.class` with the same ordering, so the
  two are one answer rather than two that happen to agree. Three cases sit around that default. A
  user rule wins outright (`--class sym=class`, `--class-file`), because naming a glyph for a symbol
  is saying what to draw. A component carrying no stamp falls back to the built-in rule table. The
  table matches the resolved part or symbol name first and then the ref-des letter prefix on a
  startswith match, so `RE1` and `Cout` still classify. A hand-built `ir.Design` takes this path,
  and so does any host that skipped the loader. And only a class that reaches no glyph draws the
  generic box.

  A class with no glyph of its own draws through its family, which is already in the stamped set:
  a zener carries `["zener", "diode"]` and draws as a diode. `glyphAliases` covers the rest, where
  the set holds no drawable tag (`clock`, `test_connector`, `ideal_diode_controller`,
  `current_limit_switch`) or the class
  arrives outside a set from a user rule. These are drawing conventions and not classification
  claims, so they are kept out of `classify`'s family table. In that table a test connector is
  deliberately NOT a connector for the protection rules, even though it is drawn as one.
  `TestEveryStampedClassDraws` holds every shipped class to reaching a glyph, so a new class cannot
  quietly draw as a box while every rule and query knows what it is.
- **`FaithfulSource`** draws the design's own symbols (from a geometry sidecar) re-laid-out, falling back per ref to the Registry and then the box. `--symbols=faithful` selects it. A symbol that failed to load counts as unresolved, not provided.

Two layout properties keep the output legible, both applied in assembly (no per-strategy change, because grid and layered both place on integer pitch multiples):

- **Size-aware packing** groups nodes by distinct X (columns) and Y (rows) and sizes each cell to `max(pitch, symbolSize + gutter)`. A uniform glyph grid keeps `pitch`, and only a large (faithful) symbol expands its own column or row, so mixed sizes pack tightly without overlap.
- **Edges are pin-accurate.** A net's hyperedge star runs from each connection's *pin*. The pin's world position is the placed node origin plus the symbol's pin point whose `port_ref` matches the connection's pin. When the symbol has no such pin, the star falls back to the node center. A net where three or more pins meet gets a junction dot at the centroid. Auto-layout placements carry no rotation, so `origin + pin.Loc` is the world point.

`agni render --report` and the `GetReport` web API serve a conversion report. It explains how each
component mapped. For each component it gives the device class and whether it drew a glyph, the
generic box (a class with no glyph, or no class at all), a provided symbol, or an unresolved
fallback. It calls out the box list and the unresolved list. The unresolved list points at the
symbol libraries a run searched, the design's project descriptor and `--symbol-path`. xschem and
gEDA need `--symbol-path`, because their symbol artwork lives in external `.sym` files.
The report names the class and the cell separately, because they answer different questions and can
legitimately differ. A thermistor is reported as a thermistor and drawn with the resistor glyph. The
kind is decided on the CELL for that reason. Reading the class instead would call a box a glyph the
moment a stamped class had no artwork.

**A render that came up short says so.** Suppose a placement's symbol did not resolve. The placement
contributes no shapes, so it drops out of the document along with the entity keys that make it
pickable. The annotation pass still draws its reference designator. The sheet then shows every ref
des, every wire and the title block, and every component on it is silently unclickable, so a reader
sees `C1` printed, clicks it, gets nothing, and reasonably concludes agni knows nothing about `C1`.

`SchematicGeometry.undrawn` carries those placements, filled where geometry is produced (both the
faithful read and the auto-layout) through `geomath.SymbolFor`, the same resolution the renderer
performs. The two have to share it. `core/validate` used to run a stricter join of its own. It
counted a placement as unresolved even when the renderer's cell-only fallback resolved it. A
shortfall report built on that join would have named placements that draw perfectly well. A banner
that is usually wrong teaches a reader to distrust the one time it is right (agni issue 354).

`agni render` prints the shortfall to stderr, grouped by the missing symbol reference because one
missing library commonly costs every part drawn from it. The viewer shows it as a notice over the
canvas. Both are silent when nothing is undrawn.

The report reads the design through the same configured read the drawing does. It has to. The report would REPORT an unresolved symbol. Suppose it were built from a read that could not see the project's declared library. It would then diagnose a problem it had caused itself (agni issue 347).

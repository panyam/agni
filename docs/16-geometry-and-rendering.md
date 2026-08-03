# Geometry sidecar and scalable rendering

See [README](README.md). Enforceable rules in [/CONSTRAINTS.md](../CONSTRAINTS.md).
Builds on the sidecar decision in
[13-ingestion-ir-architecture](13-ingestion-ir-architecture.md) (geometry is a keyed
sidecar, not in the core IR), the stack decision in
[14-stack-and-architecture](14-stack-and-architecture.md), and the presenter contract
in [15-presenter-contract](15-presenter-contract.md). The format ground truth is in
[edif-schematic-primer.md](edif-schematic-primer.md). Drives roadmap workstream WS7 and
the data half of WS1-003.

This doc records how we represent schematic geometry and how we render it in the
browser at scale (100k+ primitives per design, 82 sheets).

## The problem

A web renderer needs geometry (symbol shapes, placements, wire routing, pin
coordinates). Diff / rules / sim do not. Geometry volume, not frame rate, is the
constraint: ~121k figures and ~149k points across 82 sheets. The naive path (one proto
message per point, one draw object per primitive) allocates hundreds of thousands of
objects on both sides of the wire and on the GPU, which is what actually breaks.

## Three representations, kept distinct

The mistake is collapsing the API, the wire format, and the compute layout into one
type. We keep three, matched to three access patterns.

### Tier 1: logical contract (the public API)

Nested, ergonomic proto messages, sized O(thousands): `SymbolDef`,
`SymbolPlacement{ref_des, transform}`, `PinPoint{port_ref, x, y}`,
`WireGeometry{net, ...}`, `SheetGeometry`. This is what single reads
("give me placement R12"), list reads ("placements on sheet 3"), picking results, and
IR-keying use, and what other tools consume. Stable and readable. **Storage/packing
details never leak in here.** Proto is the single source of truth for this tier
(honors C2's no-drift intent).

### Tier 2: columnar transport (the storage/wire-optimized form)

A separate message carrying the vertex streams as a **`bytes` blob** with a defined
little-endian columnar layout, addressed by index back to tier 1. Requested explicitly
by the bulk path ("give me the packed buffers for sheet N"), never mixed into tier 1.
This is what crosses to the GPU.

**Coordinates here are int32, and sheet-relative (rebased).** Not to save disk (the
whole design is ~1.2MB vs ~2.4MB, immaterial), but because the consumer forces it:
WebGL2 / GLSL ES 3.0 has no 64-bit vertex attribute and no `int64`/`double` in shaders,
so the blob that feeds GPU buffers is physically 32-bit. Storing int64 here would force
a narrowing pass on load, defeating the zero-copy envelope. int32 over float32 because
int32 is exact to 2^31 while float32 dies at 2^24 (below our ~8.6e7 range, section on
units). Rebasing per sheet (subtract the sheet min corner) bounds values by the sheet
extent regardless of absolute-coordinate magnitude, making int32 provably safe and
avoiding the float32 precision trap at the GPU. Tier 1 stays int64 (read-only, O(few),
range-safe, no unit assumption baked into the public API); narrowing happens only here.

### Tier 3: in-memory compute form (Go)

What the reader builds and what a server-side spatial index / culler operates on.
Columnar, allocation-light (`struct { Xs, Ys []int32; ... }`). This is a **derived
projection of the proto contract, not a second schema.** Proto is the boundary format,
not the compute format.

## Why not just use proto everywhere

Vanilla protobuf-go generates `[]*Point` for `repeated Point`, so bulk geometry becomes
~149k heap objects with pointer chasing, and marshal/unmarshal touches every varint.
That is fine as a wire/disk contract but wrong as the thing a renderer or spatial index
iterates. It also buys nothing at the browser edge, because bulk data is copied into a
typed array at the WASM boundary regardless (`CopyBytesToGo` / `CopyBytesToJS`). So
proto is the **boundary format**, and tier 3 is the **compute format**.

## Making wire serialization nearly free (the bytes envelope)

Tier 2 is a `bytes` field holding a packed columnar buffer. This removes the mapping tax
on the hot path:

- **In-memory (tier 3):** the columnar arrays' backing store *is* the byte slice (or a
  trivial view over it). Mapping to proto = assign the slice; no per-element copy, no
  `[]*Point`.
- **Serialize:** proto encodes a `bytes` field as `tag + length + raw copy`. One memcpy,
  not 149k varint encodes.
- **Deserialize:** you get the slice back and reinterpret it as columnar arrays.
- **Browser:** the blob arrives as a `Uint8Array`; make `Int32Array` / `Float32Array`
  views over it and hand them to GPU buffers. Zero-copy on the JS side.

**Cost we accept:** the blob is opaque to proto tooling (no field-level introspection,
no unknown-field evolution *inside* the blob). We own its layout versioning via an
explicit `layout_version`. This is acceptable for a render-only vertex stream. The C2
rationale that most motivates strict proto modeling (unknown-field retention for the
lossless IR, docs/13) is IR-specific and does not apply to a lossy-bounded render
subset. C2's cross-language no-drift intent still holds and is honored by keeping the
layout spec authoritative and versioned.

**Why not FlatBuffers / Cap'n Proto:** they are serialization formats, not a
replacement for `protoc-gen-go-wasmjs`, which is a Go-WASM-to-JS **bridge generator**
(it emits the service exposure to `js.Global()`, the TS client facades, and the duplex
presenter-service scaffolding C3 mandates). Two reasons we keep proto:

1. **The zero-copy win is marginal here.** FlatBuffers/Cap'n Proto read fields in place
   with no parse step. For our vertex tier the `bytes` envelope already gets that: the
   blob is our columnar layout, TS makes typed-array views over it zero-copy, Go assigns
   the slice. The only extra they offer is structured random access inside the blob,
   which a flat vertex pool with a layout we own does not need. We would adopt a new
   toolchain to save one memcpy of a few MB per sheet.
2. **Neither ships a Go-WASM bridge generator.** Adopting one means hand-writing the
   presenter-service exposure, TS clients, and boundary wiring that the wasmjs pipeline
   generates, plus running a second IDL alongside the proto IR (C2), creating a seam
   between netlist IR and geometry sidecar. One schema toolchain across the system beats
   a per-vertex micro-optimization.

If profiling later shows the tier-2 or boundary copy is a real bottleneck, we can adopt
FlatBuffers **just for the vertex blob** while keeping proto for the contract and the
bridge (tier 2 sits behind the contract, so it is a localized swap). Premature now, and
a STACK_CATALOG-consult decision if ever taken.

## Proto contract sketch

Tier 1 (logical), in a new package `agni.v1.geom`, a separate artifact never imported
by diff / rules / sim:

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

**The sidecar is a format-neutral geometry IR, not an EDIF dump.** The primitives
(shapes, points, polylines, placements, pins, labels, sheets) are universal 2D
schematic vector constructs; the transform is a general rotation/mirror/scale rather
than EDIF's fixed orientation enum. So KiCad, Altium (via export), OrCAD, IPC-2581, and
other readers can populate the same sidecar, mirroring the many-readers-one-IR thesis
(docs/13). Symbol graphics come from the source file itself (EDIF embeds them as
vectors), so the renderer needs no external symbol-asset library for the faithful view;
only the netlist-graph fallback and text glyphs are drawn by us.

Tier 2 (columnar transport), separate message, requested per sheet:

```proto
message PackedSheet {
  string sheet_id = 1;
  int32  layout_version = 2;          // we own this; bump on layout change
  int64  origin_x = 6; int64 origin_y = 7;  // sheet min corner; blob coords are relative to this
  bytes  vertices = 3;                // int32 LE pairs, sheet-relative: [x0,y0,x1,y1,...]
  bytes  primitives = 4;              // fixed-width records: kind,group,first_vertex,vertex_count
  repeated PrimitiveKey keys = 5;     // O(primitives): primitive index -> ref_des / net (for picking)
}
```

The reader emits tier 1 always. Tier 2 is generated for the scale path (per-sheet GPU
upload) and shares the same underlying vertex data. Tier 1's `Point`/`Shape`/`Polyline`
are for reads and small designs; they are not the bulk carrier.

## Reader plan (WS1-003 data half)

- `edif/schematic.go`: `ReadSchematic(io.Reader, sourceFile) (*geom.SchematicGeometry, error)`,
  reusing `sexpr.go`. Does **not** touch `reader.go` (the netlist extractor).
- Parses into the tier-3 columnar Go form, then maps to tier-1 proto (and tier-2 when
  the renderer needs it).
- Fidelity = lossy-bounded (render subset) per C6. Drops connectivity graph, properties,
  style palette, display metadata, back-annotation (see primer §6). Keeps shapes, pin
  points, placements + transforms, wire polylines, labels, sheets.
- Heavy (62MB) so server-side / CLI only (C7). A `cmd/edifgeom` CLI dumps stats and
  serializes the sidecar.
- Join keys: `ref_des` + `source_id` (instance `&id`), `net` name, `port_ref` (primer §8).

## Renderer plan (WS7)

- **SVG verification backend (built first, `render/` + `svg/`).** A pure-Go
  `SheetSVG(geometry, sheet)` renders a sheet to SVG via a small zero-dep `svg/` element
  builder. It is the eyeball/golden instrument, not the production renderer, and it proved
  the geometry model against the real 62MB board before any browser code: symbol placement
  + orientation math, pins landing on wire endpoints, per-view (multi-section) symbol
  selection, and the annotation layer (pin numbers, title-block/symbol text, off-page
  connector net names). WebGL2 is a second backend over the same `render` layer.
- **Backend: WebGL2 from the start.** Not Phaser (its per-sprite scene graph does not
  scale to 100k+ vector primitives). Batched line/instanced rendering: static geometry
  uploaded once as GPU buffers from the tier-2 blob; a small dynamic overlay
  (selection/hover) crosses per frame (C4).
- **Per-sheet loading.** Load one sheet at a time (a few thousand primitives), not the
  whole design. Server-side spatial index / culling optional later.
- **Camera and picking are view-local** (C3): pan/zoom is an affine transform in the
  view; picking uses a view-local spatial index over what was drawn and emits semantic
  intents (`ComponentSelected(refDes)`, `NetHovered(net)`). The presenter never sees
  pixels.
- **Renderer runtime: TS in-process** (superseded the original WASM plan). A continuously
  dragged canvas is exactly the high-frequency surface C7 keeps in TS to avoid per-event
  boundary cost, so the WebGL renderer + camera are plain TS (`web/src/{webgl,camera,
  canvas}.ts`); there is no Go/WASM presenter and no `protoc-gen-go-wasmjs` codegen. The
  server stays authoritative (parsing, geometry, style); the client only draws.
- **Fallback view:** a netlist-graph diagram derived from the IR alone (auto-layout),
  for when geometry is absent. Separate renderer over the same presenter contract.

## WebGL renderer as built (WS7-001, WS7-002)

The WebGL path reaches SVG-backend parity for a schematic sheet. What the plan above got
right (WebGL2, upload-once, view-local camera, per-sheet) held; the presenter runtime and a
few mechanics differ:

- **Join by embedded keys, not a separate IR load.** `PackedSheet` carries
  `PrimitiveKey{ref_des,net}` inline, so picking reads the key off the packed primitive; no
  second IR fetch to join.
- **Y-up end to end.** geom is Y-up and WebGL NDC is Y-up, so the camera matrix does **not**
  flip Y (an early bug did, rendering upside-down vs the SVG oracle). The SVG backend flips
  once only because SVG pixel space is Y-down.
- **Worksheet furniture is synthesized in the packer**, in world units, only when the sheet
  has a page (`sheet.Size`): frame border, zone-ruler ticks, title-block box + dividers, under
  a `groupFrame`. Mirrors the SVG `drawWorksheet`; the frame extends the packed bounds so the
  camera fits the whole page. Ruler/title-block **text** is not geometry (see overlay).
- **Text overlay (a GPU line pipeline draws no glyphs).** Labels ride on the wire as
  `PackedSheet.labels` (`PackedLabel`: world position + height + rotation + justify + color,
  computed server-side with the same transform math as SVG). The client draws them as a
  second **SVG layer** over the canvas: each `<text>` is placed once in a **Y-flipped world
  space** (`y -> -y`, so glyphs stay upright), and pan/zoom is a single **CSS transform** on
  the layer (GPU-composited, one style write per frame). Both layers read the same camera in
  the same `requestAnimationFrame`, so text stays locked to geometry with no drift. The layer
  is `pointer-events:none` and shows only in WebGL mode. Browser-owned text metrics (docs/14),
  so no glyph atlas. One font per sheet (`PackedSheet.font_family`); per-element font waits on
  the IR carrying it.
- **One palette for both renderers (`render.Style`, C12).** Colors and the default font are
  injectable data (`render.Style` + `DefaultStyle`, override via `WithStyle`), resolved
  server-side and consumed by both backends: SVG draws from `Style` directly; the WebGL side
  receives `group_colors` (geometry, indexed by group), `background_color`, `font_family`, and
  per-label `color` on the wire, so `webgl.ts` holds no palette of its own. `agni serve
  --theme default|dark` recolors both backends from one `Style`.

## The geometry proto is a render contract: one proto, N producers -> N renderers

`geom.SchematicGeometry` is not owned by one format. It is the contract that decouples
**producers** from **renderers**:
- Producers: faithful readers (`edif.ReadSchematic` for `.eds`; `kicad.ReadSchematicGeometry`
  for `.kicad_sch`, WS1-008) and the auto-layout path (`graph.Layout`, grid/layered, WS7-003).
- Renderers: `render.SheetSVG` and `render.PackSheet` (tier-2 -> WebGL).

A new source plugs into the contract and every renderer inherits it; a new renderer consumes
every source. Adding the KiCad reader lit up faithful KiCad rendering on the SVG backend with
no renderer change. The corollary bites too: when two producers feed the same field into one
renderer, their conventions must be reconciled (see justify below): the shared contract
surfaces the inconsistency instead of hiding it.

## KiCad geometry reader: coordinate conventions (WS1-008)

KiCad uses **two coordinate frames**, and conflating them is the trap:
- **Library symbol graphics are Y-up** (like the geom contract). Map lib-local points straight
  through (mm -> nm, no Y flip).
- **The schematic sheet is Y-down.** Flip Y only for sheet-level coordinates: placement
  origins, wires, labels, junctions.
- **Rotation is negated** (`geom = 360 - kicad`). Converting the sheet frame to Y-up is a
  reflection, which inverts rotation direction. Verified against `ApplyTransform` (CCW in
  Y-up). Mirror axes and origin translation are unchanged by the flip. Flipping lib points
  too, or not negating rotation, mirrors/overlaps every rotated symbol (an upside-down GND is
  the tell).
- Units: KiCad mm -> nm (x1e6, exact at KiCad's 0.0001mm grid); `unit_nm = 1`.
- `#`-prefixed references (`#PWR`, `#FLG` power/flag virtuals) are hidden by KiCad; drop them
  for display. `pl.RefDes` stays the picking key.

The pin-on-wire coincidence rate (fraction of placed pin connect-points landing on wire
endpoints) is the cheap correctness signal for the transform math, but it is NOT sensitive to
rotation *direction* on symmetric 2-pin parts (the pins just swap), so eyeball the asymmetric
symbol bodies too.

## Render-fidelity fields (WS7-016)

Added to the contract so a reader-produced sheet renders like the source tool:
- `SheetGeometry.shapes`: free sheet graphics not owned by a symbol (junction dots,
  no-connect markers, notes). KiCad junctions/no-connects/graphics; EDIF commentGraphics.
- `Shape.fill` (`UNSPECIFIED`/`OUTLINE`/`BACKGROUND`/`COLOR` + `fill_color`): solid symbol
  bodies render filled. Not a bool: KiCad distinguishes fill types. `PlaceShape` must
  propagate it through the placement transform.
- **Canonical `justify`**: `"<h> <v>"` (h left/center/right, v top/middle/bottom). Each reader
  maps its native codes (EDIF `LOWERLEFT` -> `"left bottom"`; KiCad tokens are already
  canonical). The SVG backend applies `text-anchor` (horizontal) AND `dominant-baseline`
  (vertical).
- `SymbolPlacement.fields` (`Field{name,value,origin,justify,visible,...}`) + `PinPoint.name`:
  instance text (Reference/Value/custom) is structured on the placement, not loose sheet
  labels, so a consumer knows which text is which field (for the WS9 visual diff). Sheet
  labels are now only genuine free text (net labels, notes).

## Text stays readable and inside its box (WS7)

Readers carry a source's text orientation and sizing faithfully; making that text *legible*
is the render layer's job, shared by the SVG backend and the WebGL overlay (C12: the two agree
by construction). Two rules live in `render` and are applied by both `drawText` (SVG) and
`collectLabels` (the overlay's label source):

- **Upright text (`readableText`).** No run is ever drawn upside down, the way every EDA viewer
  (Eeschema, Altium, OrCAD, the tool that authored our EDIF) draws it. A run whose angle would
  read upside down (normalized magnitude > 90, e.g. a symbol placed `R180` or a net label with
  its own `R180` orientation) is turned a further 180 and its `justify` flipped on both axes, so
  it stays anchored to the same corner. Vertical text (`+/-90`) is left alone, so the KiCad-90
  `rotate(-90)` parity holds. Without this, R180-placed connectors on a real-corpus
  headers sheet rendered their ref-des and half their net-stub labels upside down.
- **Caption fits its box (condense, don't shrink).** A symbol caption with no source text height
  (e.g. a "Net Splitter" label) fell back to a fixed size and spilled past its symbol. It is now
  condensed horizontally to the drawn body-box width (`captionWidth` = widest `figureGroup=="BOX"`
  rect, not the pin-stub-widened bounding box) via SVG `textLength` + `lengthAdjust="spacingAndGlyphs"`,
  keeping the font height instead of dropping it to a few pixels, and only when it would overflow.
  `PackedLabel.max_width` carries the budget to the overlay so it condenses identically.

**Gotcha:** `rsvg-convert` (librsvg) silently ignores `textLength`/`lengthAdjust`, so an offline
PNG of the SVG backend still shows a caption overflowing. Browsers honor it, and the viewer's
SVG/WebGL output is browser-consumed, so it is correct in the product. Verify condensing in a real
browser (e2e screenshot), not via an rsvg PNG or golden. Repro fixture: `edif/testdata/upsidedown.eds`
(an R180 connector + R180 net stub with R0 controls); see PR 54.

## Fidelity

Reader declares **lossy-bounded (render subset)** per C6. It is not a round-trip
oracle; the schematic drawing is not reconstructed byte-for-byte, only the render
subset is extracted.

## Board geometry sidecar (WS1-006)

There are two geometries and two sidecars, one per physical medium:

- **Schematic-page geometry**: symbol shapes, wire runs, labels: `SchematicGeometry`
  (this document's subject so far).
- **Board geometry**: component placement, pads, routed copper, vias, zones, the layer
  stackup, and the board outline: `BoardGeometry` (`geom_board.proto`, same
  `agni.v1.geom` package so the primitive vocabulary, `Point`, `Polyline`, `BBox`,
  `Provenance`: is shared rather than duplicated).

The board sidecar follows every rule established above: a separate keyed artifact, never
imported by diff/rules/sim, joined to the netlist IR at consumption time by stable keys, 
`ref_des` for placements (with pad `number` matching `ir.Connection.pin_ref`, so
`(ref_des, number)` joins a copper land to its netlist pin) and net **name** for routed
copper (`NetCopper`, the board analogue of `WireGeometry`). The first producer is the
KiCad reader (`kicad.ReadBoardGeometry`, over the same s-expr parse as the netlist
reader); IPC-2581 and ODB++ producers slot in behind the same proto, keeping the
one-contract/N-producers property.

**Silkscreen / legend text (WS7).** `BoardText` carries the board's placed strings, each
footprint's ref-des and value, plus free `gr_text` such as the title block, so both board
renderers (`BoardSVG` and the packed/WebGL path) draw them, matching what KiCad shows.
Text is universal across board formats (IPC-2581 legend, ODB++, Gerber), so it lives in the
shared contract, not a reader (C9: IR fields earn their place, so the IR never overfits one format). Every `BoardText` is **board-frame absolute**: the
reader composes a footprint's local text offset through the placement transform with
`geomath.ComposePlacement`: the *same* composer `padWorld` uses for pads, which is what
pins text to its part, and free text is authored absolute already. Glyph angle folds under
KiCad's default *keep_upright* so text on a rotated footprint never renders inverted (free
`gr_text` is exempt, so a deliberately mirrored back-side title stays mirrored). Hidden
source text is dropped.

**Silkscreen / fab graphics (WS7).** `BoardGraphic` carries the non-copper artwork the same
way: a footprint's silk/fab body outlines, courtyards, and polarity marks (`fp_line`/`fp_arc`/
`fp_circle`/`fp_poly`/`fp_rect`), plus free `gr_*` graphics that are not the board edge (the
edge stays `BoardOutline`). It reuses `geom.Shape` for the geometry itself (POLYLINE/RECT/
CIRCLE, arcs approximated to polylines under the same C6 bound), adds only a stroke `width`,
carries the source `layer` verbatim, and (like `BoardText`) pre-composes footprint graphics to
board coordinates through `geomath.ComposePlacement` so they sit on their part. Both renderers
draw them in a silk group; per-layer visibility is the same client-side concern as the copper
strata. Universal across board formats (IPC-2581 legend, ODB++, Gerber silk are all line/arc/
region artwork), so it is a shared-contract field a second producer fills, not a reader's.
Filled zone regions and per-side silk/fab default-visibility remain a later refinement.

Tiering: only **tier 1** exists for the board today. The columnar packed transport for
high-volume copper (the `PackedSheet` analogue) is deliberately deferred until its
consumer, the WS7 board renderer, exists; the tier split above is the design it will
follow. Coordinates are nanometers (`unit_nm=1`), Y-up, matching the KiCad schematic
reader's convention; rotations are carried verbatim from the source and composition with
the Y-flip is the renderer's concern (documented on the proto). Fidelity is
lossy-bounded (render/DRC subset): arc tracks, zone fill polygons, teardrops, and 3D
references are out; zone outlines are as authored; outline arcs are approximated as
polylines.

What it enables: the WS3-008 geometric DRC class (clearance, width, annular ring) gets
its data tier, and a WS7 board viewer gets its contract.

**Copper stroke width (both renderers).** Board copper renders at its TRUE physical width,
floored to a *physical* minimum (`minStrokeNm`, ~25µm in board space), never to a fixed
output-pixel constant: the SVG backend strokes `max(width, minStrokeNm)*scale` and the WebGL
packer's `quadPts` tessellates the same floored width. An output-pixel floor (the earlier
`strokePx=0.8`) clamped every sub-pixel trace to one width on a scaled-to-fit board, merging
dense copper into a blob and erasing relative trace widths; the board-space floor keeps
thickness proportional to the copper at every zoom, as EDA viewers draw. WebGL was already
faithful: browser GL line width is ~1px, which is exactly *why* the packer draws tracks as
triangle quads rather than lines, so only the SVG backend needed converging. Scope is board
copper; schematic wire strokes stay a fixed pixel width (line-art at readable zoom, no
blobbing), and pad/via size-floors stay (discrete-feature visibility, a separate concern).

**Buses draw distinctly (WS7-042).** `WireGeometry.kind` (unset=wire, `KIND_BUS`, `KIND_BUS_ENTRY`)
lets the readers flag a bus trunk/entry so both renderers style it apart from a net wire: the SVG
backend strokes it thicker (`busStrokePx`) in the bus color (`Style.Bus`), and the WebGL packer
tessellates it to true-width triangle quads in a distinct `groupBus` (12, past the board strata in
the shared group space) with `quadPts`, the same "GL lines are ~1px, so widen via quads" path copper
takes. The kind is format-neutral (KiCad sets it today from `bus`/`bus_entry`; a bus carries no net,
so its member nets stay unmodeled, WS1-034). The packed palette only grows to cover `groupBus` on a
sheet that actually packs a bus, so a bus-less sheet's bytes are unchanged. A `bus-not-modeled`
finding HIGHLIGHTS its drawn trunk on click, keyed by the bus NAME (`WireGeometry.Net`, gated on the
bus kind so a same-named net wire is not caught), via OUTLINE (recolor in place — a bus is already
thick, so the net-focus PATH shape is unneeded and mis-tessellates the WebGL quads). The reader names
the bus wire from its source label (KiCad range-label-on-wire; gEDA/xschem inline), which is also the
finding subject, so the join is by name (WS7-042b; not the uuid an early cut assumed — WS1-034 Phase 2
made bus detection name-keyed). An UNDRAWABLE bus (a `bus_alias`, an EDIF `array`, a hierarchical port
with no drawn wire) shows a server-authoritative "bus not drawn" note instead, computed in
`AnnotateSheets` from the drawn-`KIND_BUS`-name index (WS7-042c).

### Second producer: IPC-2581, and the contract's first second-format audit (WS1-023)

`ipc2581.ReadBoardGeometry` is the second `BoardGeometry` producer. The proto was designed
from one producer (KiCad), so the second is also the audit that proves the fields are not
overfit, the C9 overfit rule only bites when a second vendor's data lands in them. The
audit's outcomes, all resolved without a proto change:

- **No new fields earned.** IPC-2581's stackup carries per-layer material and thickness, but
  no board rule or renderer consumes them, so `BoardLayer` keeps only `kind` (IPC-2581's
  `layerFunction` verbatim, the same discipline as KiCad's `kind` word). The richer stackup
  stays in the netlist IR's `ir.Stackup` and is ledgered for when a consumer earns it.
- **Padstack def/instance resolves to the same inline `Pad`.** IPC-2581 references pad shapes
  by id from a primitive dictionary (`StandardPrimitiveRef` → `EntryStandard`), where KiCad
  inlines them per footprint. The producer resolves the indirection and emits the identical
  flattened `Pad` (shape word, size, footprint-local `at`), confirming the inline mapping holds
  for a def/instance source.
- **Frame.** IPC-2581 is Y-up like the geom contract, so coordinates map directly with **no**
  Y negation (KiCad is Y-down and negates). Sides normalize into the contract's KiCad-style
  vocabulary (`TOP`→`F.Cu`, `BOTTOM`→`B.Cu`) so the format-neutral renderer's front/back
  classifier works unchanged, the "N producers → one vocabulary" property in miniature.
- **The board-availability gate generalized.** `check.Available`'s `board.` gate keyed on
  `SourceFormat == "kicad-pcb"`; it now tests a `boardFormats` set. The authoritative
  per-design gate remains the Model's board tier (empty → rules silent).

Scope landed across two PRs: placements, pads, layers, and outline first; then routed copper
, tracks (interleaved straight/arc steps decoded in document order, arcs approximated as
16-chord polylines) and vias (drilled `Hole[platingStatus=VIA]` with the co-located copper pad
as the annular). That lights all four board DRC rules on IPC-2581 (track-width, copper-clearance,
hole-size, annular-width).

**Full producer parity (WS1-031, PRs 152/153).** The second producer initially lagged the
contract: fields KiCad filled sat empty on IPC. That gap is now closed, all into existing
fields with no proto change: component **VALUE** (`Component>NonstandardAttribute`) →
`ir.Component` attrs; silk/fab **graphics** (`Package` `Marking`/`Outline`/`AssemblyDrawing`,
composed per placement) → `BoardGraphic` (IPC encodes silk as vector geometry, not string text,
so `BoardText` legitimately stays empty for this producer, a C9 format difference); copper
plane/pour **fills** (`Set>Features>Contour`) → `Zone` (authored outline, cutouts drop per C6);
via layer **spans** (drill-`Layer` `<Span>`) → `Via.layer_from/to`; and user-primitive **pads**
(`UserPrimitiveRef`→`DictionaryUser`, its own units) → real pad extents. Two gotchas the work
pinned: the fill is nested under `Set>Features>Contour`, not a direct child (a fixture test
passed while the real board rendered zero zones, caught only by the corpus-render rule), and a
drill layer's `<Span>` lives on the `Layer`, not the `Hole`. Non-via copper lands, fill cutouts,
and the padstack def/instance indirection remain ledgered (no consumer yet); stackup materials
are WS1-036.

**Two render-faithfulness bugs on real Allegro exports (PR 154).** Both passed unit fixtures
and showed only on a corpus PNG render. (1) Cadence Allegro writes `clockwise="TRUE"/"FALSE"`
in UPPERCASE; a case-sensitive `== "true"` read every clockwise arc as counter-clockwise, so it
swept the long way and ballooned outline/copper arcs, parse with `strings.EqualFold`. (2) A
`Zone` is a copper pour, so `zones()` must gate on the `Set`'s layer being a copper function
(`CONDUCTOR`/`PLANE`); without the gate, `DOCUMENT` (fab/assembly-drawing) contours leaked in as
copper and blew up the render bounds. General lesson for any second producer: a construct that
is copper on one layer is drawing geometry on another: classify by the source `layerFunction`,
not by the element name.

## Auto-layout node drawing (WS7-025..032)

The netlist-graph fallback (`graph.Layout`, `agni render --layout=grid|layered`) has no source
geometry, so it synthesizes a `SchematicGeometry` from the IR. `graph/assemble` decides what to
draw at each component node through a pluggable **`SymbolSource`** (`graph/sources.go`), so the
layout stays fixed while node artwork varies:

- **`Registry`** (`graph/symbols.go`): classified synthetic glyphs. Classification is data-driven
  and user-extensible: an ordered `ClassRule` list matches the resolved part/symbol name first,
  then the ref-des letter prefix on a *startswith* (so `RE1`/`Cout` still classify), mapping to
  open string class ids each with a hand-authored glyph (resistor, capacitor, inductor, ferrite,
  diode, led, tvs, fuse, connector, test_point, crystal, ic, transistor, ground, the same
  vocabulary as the check Model's component.class fact, docs/19). Only unrecognized parts draw
  the generic box; multi-pin bodies (ic, connector) carry no per-pin terminals, so their edges
  attach at the node center like the box does. Callers layer rules on the defaults
  (`--class sym=class`, `--class-file`).
- **`FaithfulSource`**: the design's own symbols (from a geometry sidecar) re-laid-out, falling
  back per ref to the Registry then the box. `--symbols=faithful` selects it. A symbol that failed
  to load (`Asset.placeholder`) counts as unresolved, not provided.

Two layout properties keep the output legible, both applied in `assemble` (no per-strategy change,
because `grid` and `layered` both place on integer `pitch` multiples):

- **Size-aware packing** (`graph/compact.go`): group nodes by distinct X (columns) and Y (rows),
  size each cell `max(pitch, symbolSize + gutter)`. A uniform glyph grid stays at `pitch`; only a
  large (faithful) symbol expands its own column/row, so mixed sizes pack tightly without overlap.
- **Pin-accurate edges**: a net's hyperedge star runs from each connection's *pin*: the placed
  node origin plus the symbol's `PinPoint` whose `port_ref` matches the connection's pin, with a
  node-centre fallback when the symbol has no such pin. A net where three or more pins meet gets a
  `KIND_DOT` junction at the centroid. Auto-layout placements carry no rotation, so `origin +
  pin.Loc` is the world point (matching the renderer's `PlacePin`).

**Conversion report.** `graph.BuildReport` (behind `agni render --report` and the `GetReport` web
API) explains how each component mapped: its device class and whether it drew a glyph, the generic
box (unmapped class), a provided symbol, or an unresolved fallback, with call-outs for the box
list and the unresolved list (which points at `--symbol-path`, needed for xschem/gEDA whose symbol
artwork lives in external `.sym` files).

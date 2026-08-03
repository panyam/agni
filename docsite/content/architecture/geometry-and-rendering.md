---
title: "Geometry sidecar and rendering"
description: "How schematic and board geometry are represented as a keyed sidecar and rendered in the browser at scale."
---

Diff, rules, and simulation work on connectivity, and they never need pixels. A web viewer does. So Agni keeps drawing geometry (symbol shapes, placements, wire routing, pin coordinates) in a separate artifact, the geometry sidecar, that references the [core IR](../ingestion-and-ir/) by stable keys and joins to it at render time. This page covers how that geometry is represented and how it renders in the browser at scale, meaning 100k+ primitives per design across dozens of sheets.

## The problem

A web renderer needs geometry. The volume is the constraint, not the frame rate: roughly 121k figures and 149k points across 82 sheets in the reference design. The naive path, one proto message per point and one draw object per primitive, allocates hundreds of thousands of objects on both sides of the wire and again on the GPU, and that is what breaks.

## Three representations, kept distinct

The mistake would be collapsing the API, the wire format, and the compute layout into one type. They are kept separate, matched to three different access patterns.

### Tier 1: logical contract (the public API)

Nested, ergonomic proto messages, sized in the thousands: `SymbolDef`, `SymbolPlacement{ref_des, transform}`, `PinPoint{port_ref, x, y}`, `WireGeometry{net, ...}`, `SheetGeometry`. This is what single reads ("give me placement R12"), list reads ("placements on sheet 3"), picking results, and IR-keying use, and what other tools consume. It is stable and readable, and storage or packing details never leak into it. The proto is the single source of truth for this tier.

### Tier 2: columnar transport (the storage and wire-optimized form)

A separate message that carries the vertex streams as a **`bytes` blob** with a defined little-endian columnar layout, addressed by index back to tier 1. The bulk path requests it explicitly ("give me the packed buffers for sheet N"), and it is never mixed into tier 1. This is what crosses to the GPU.

Coordinates here are int32 and sheet-relative (rebased). The reason is not disk size (the whole design is about 1.2MB versus 2.4MB, immaterial) but the consumer. WebGL2 and GLSL ES 3.0 have no 64-bit vertex attribute and no `int64` or `double` in shaders, so the blob that feeds GPU buffers is physically 32-bit. Storing int64 here would force a narrowing pass on load and defeat the zero-copy envelope. int32 rather than float32 because int32 is exact to 2^31 while float32 loses exactness at 2^24, below the coordinate range. Rebasing per sheet (subtracting the sheet's minimum corner) bounds every value by the sheet extent regardless of absolute-coordinate magnitude, which makes int32 provably safe and avoids the float32 precision trap at the GPU. Tier 1 stays int64, since it is read-only, small, range-safe, and bakes no unit assumption into the public API. Narrowing happens only in tier 2.

### Tier 3: in-memory compute form (Go)

What the reader builds and what a server-side spatial index or culler operates on. Columnar and allocation-light (`struct { Xs, Ys []int32; ... }`). This is a derived projection of the proto contract, not a second schema. Proto is the boundary format, not the compute format.

## Why not just use proto everywhere

Vanilla protobuf-go generates `[]*Point` for a `repeated Point`, so bulk geometry becomes about 149k heap objects with pointer chasing, and marshal or unmarshal touches every varint. That is fine as a wire or disk contract but wrong as the thing a renderer or spatial index iterates. It also buys nothing at the browser edge, because bulk data is copied into a typed array at the WASM boundary regardless. So proto is the boundary format and tier 3 is the compute format.

## Making wire serialization nearly free

Tier 2 is a `bytes` field holding a packed columnar buffer. This removes the mapping tax on the hot path:

- **In-memory (tier 3):** the columnar arrays' backing store *is* the byte slice, or a trivial view over it. Mapping to proto is a slice assignment. No per-element copy, no `[]*Point`.
- **Serialize:** proto encodes a `bytes` field as tag, length, and a raw copy. One memcpy, not 149k varint encodes.
- **Deserialize:** you get the slice back and reinterpret it as columnar arrays.
- **Browser:** the blob arrives as a `Uint8Array`, and you make `Int32Array` or `Float32Array` views over it and hand them to GPU buffers. Zero-copy on the JS side.

The cost is that the blob is opaque to proto tooling: no field-level introspection, no unknown-field evolution inside the blob. That layout is versioned explicitly with a `layout_version`, which is acceptable for a render-only vertex stream. The unknown-field retention that motivates strict proto modeling elsewhere is a property of the lossless IR and does not apply to this lossy-bounded render subset. The cross-language no-drift intent still holds, honored by keeping the layout spec authoritative and versioned.

FlatBuffers and Cap'n Proto were considered and not adopted. They are serialization formats, not a replacement for the Go-WASM-to-JS bridge generator that emits the service exposure, the TS client facades, and the duplex presenter scaffolding. Two reasons proto stays:

1. **The zero-copy win is marginal here.** FlatBuffers and Cap'n Proto read fields in place with no parse step. For the vertex tier the `bytes` envelope already gets that: the blob is the columnar layout, TS makes typed-array views over it zero-copy, and Go assigns the slice. The only extra they offer is structured random access inside the blob, which a flat vertex pool with an owned layout does not need. Adopting a new toolchain would save one memcpy of a few MB per sheet.
2. **Neither ships a Go-WASM bridge generator.** Adopting one means hand-writing the presenter-service exposure, the TS clients, and the boundary wiring the wasmjs pipeline generates, plus running a second IDL alongside the proto IR, which creates a seam between the netlist IR and the geometry sidecar. One schema toolchain across the system beats a per-vertex micro-optimization.

If profiling later shows the tier-2 or boundary copy is a real bottleneck, FlatBuffers could be adopted just for the vertex blob while keeping proto for the contract and the bridge, since tier 2 sits behind the contract and the swap is localized. That is premature now.

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

The sidecar is a format-neutral geometry IR, not an EDIF dump. The primitives (shapes, points, polylines, placements, pins, labels, sheets) are universal 2D schematic vector constructs, and the transform is a general rotation, mirror, and scale rather than EDIF's fixed orientation enum. So KiCad, Altium (via export), OrCAD, IPC-2581, and other readers can populate the same sidecar, mirroring the many-readers-one-IR idea. Symbol graphics come from the source file itself (EDIF embeds them as vectors), so the renderer needs no external symbol-asset library for the faithful view. Only the netlist-graph fallback and text glyphs are drawn by Agni.

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

- A reader exposes `ReadSchematic(io.Reader, sourceFile) (*geom.SchematicGeometry, error)`, reusing the format's parser, and does not touch the netlist extractor.
- It parses into the tier-3 columnar Go form, then maps to tier-1 proto, and to tier-2 when the renderer needs it.
- Fidelity is lossy-bounded (a render subset). It drops the connectivity graph, properties, style palette, display metadata, and back-annotation, and keeps shapes, pin points, placements with transforms, wire polylines, labels, and sheets.
- The sidecar is heavy (about 62MB for the reference board), so it is produced server-side or by the CLI only.
- Join keys are `ref_des` plus `source_id` (the instance id), the `net` name, and `port_ref`.

## Renderer plan

- **SVG verification backend (built first).** A pure-Go `SheetSVG(geometry, sheet)` renders a sheet to SVG through a small zero-dependency element builder. It is the eyeball and golden instrument, not the production renderer, and it proved the geometry model against the real 62MB board before any browser code existed: symbol placement and orientation math, pins landing on wire endpoints, per-view (multi-section) symbol selection, and the annotation layer (pin numbers, title-block and symbol text, off-page connector net names). WebGL2 is a second backend over the same render layer.
- **WebGL2 from the start**, not a per-sprite scene graph (which does not scale to 100k+ vector primitives). Static geometry is uploaded once as GPU buffers from the tier-2 blob, and a small dynamic overlay (selection, hover) crosses per frame.
- **Per-sheet loading.** One sheet at a time (a few thousand primitives), not the whole design. Server-side spatial indexing and culling are optional later additions.
- **Camera and picking are view-local.** Pan and zoom are an affine transform in the view. Picking uses a view-local spatial index over what was drawn and emits semantic intents (`ComponentSelected(refDes)`, `NetHovered(net)`). The presenter never sees pixels.
- **Renderer runtime is TS in-process** (this superseded an original WASM plan). A continuously dragged canvas is exactly the high-frequency surface kept in TS to avoid per-event boundary cost, so the WebGL renderer and camera are plain TS. There is no Go/WASM presenter for the canvas. The server stays authoritative for parsing, geometry, and style, and the client only draws.
- **Fallback view:** a netlist-graph diagram derived from the IR alone (auto-layout), for when geometry is absent. It is a separate renderer over the same presenter contract.

## WebGL renderer as built

The WebGL path reaches SVG-backend parity for a schematic sheet. WebGL2, upload-once, view-local camera, and per-sheet loading all held. The runtime and a few mechanics differ from the plan:

- **Join by embedded keys, not a separate IR load.** `PackedSheet` carries `PrimitiveKey{ref_des, net}` inline, so picking reads the key off the packed primitive with no second IR fetch.
- **Y-up end to end.** geom is Y-up and WebGL NDC is Y-up, so the camera matrix does **not** flip Y (an early bug did, rendering upside-down versus the SVG oracle). The SVG backend flips once, only because SVG pixel space is Y-down.
- **Worksheet furniture is synthesized in the packer**, in world units, only when the sheet has a page (a set `sheet.Size`): frame border, zone-ruler ticks, and the title-block box and dividers, under a frame group. It mirrors the SVG worksheet drawing, and the frame extends the packed bounds so the camera fits the whole page. Ruler and title-block text is not geometry (see the overlay below).
- **Text overlay.** A GPU line pipeline draws no glyphs. Labels ride on the wire as packed labels (world position, height, rotation, justify, and color, computed server-side with the same transform math as SVG). The client draws them as a second SVG layer over the canvas: each `<text>` is placed once in a Y-flipped world space (`y -> -y`, so glyphs stay upright), and pan and zoom are a single CSS transform on the layer (GPU-composited, one style write per frame). Both layers read the same camera in the same `requestAnimationFrame`, so text stays locked to geometry with no drift. The layer is `pointer-events:none` and shows only in WebGL mode. Text metrics are browser-owned, so there is no glyph atlas. There is one font per sheet, and a per-element font waits on the IR carrying it.
- **One palette for both renderers.** Colors and the default font are injectable data, resolved server-side and consumed by both backends. SVG draws from the style directly, and the WebGL side receives group colors (indexed by group), a background color, a font family, and a per-label color on the wire, so the WebGL code holds no palette of its own. `agni serve --theme default|dark` recolors both backends from one style. The two agree by construction.

## The geometry proto is a render contract

`geom.SchematicGeometry` is not owned by one format. It is the contract that decouples **producers** from **renderers**.

- Producers: faithful readers (EDIF's `ReadSchematic` for `.eds`, KiCad's `ReadSchematicGeometry` for `.kicad_sch`) and the auto-layout path (grid and layered).
- Renderers: the SVG backend and the tier-2 packer that feeds WebGL.

A new source plugs into the contract and every renderer inherits it, and a new renderer consumes every source. Adding the KiCad reader lit up faithful KiCad rendering on the SVG backend with no renderer change. The corollary bites too: when two producers feed the same field into one renderer, their conventions have to be reconciled (see `justify` below). The shared contract surfaces the inconsistency instead of hiding it.

## KiCad geometry reader: coordinate conventions

KiCad uses two coordinate frames, and conflating them is the trap.

- **Library symbol graphics are Y-up** (like the geom contract). Map lib-local points straight through (mm to nm, no Y flip).
- **The schematic sheet is Y-down.** Flip Y only for sheet-level coordinates: placement origins, wires, labels, junctions.
- **Rotation is negated** (`geom = 360 − kicad`). Converting the sheet frame to Y-up is a reflection, which inverts rotation direction. Mirror axes and origin translation are unchanged by the flip. Flipping lib points too, or not negating rotation, mirrors or overlaps every rotated symbol (an upside-down GND is the tell).
- Units: KiCad mm to nm (times 1e6, exact at KiCad's 0.0001mm grid), and `unit_nm = 1`.
- `#`-prefixed references (`#PWR`, `#FLG` power and flag virtuals) are hidden by KiCad and dropped for display, but the ref-des stays the picking key.

The pin-on-wire coincidence rate (the fraction of placed pin connect-points landing on wire endpoints) is the cheap correctness signal for the transform math. It is not sensitive to rotation *direction* on symmetric 2-pin parts (the pins just swap), so the asymmetric symbol bodies need an eyeball check too.

## Render-fidelity fields

These were added to the contract so a reader-produced sheet renders like the source tool.

- `SheetGeometry.shapes`: free sheet graphics not owned by a symbol (junction dots, no-connect markers, notes). KiCad junctions, no-connects, and graphics, and EDIF comment graphics.
- `Shape.fill` (`UNSPECIFIED` / `OUTLINE` / `BACKGROUND` / `COLOR`, plus `fill_color`): solid symbol bodies render filled. It is not a bool, because KiCad distinguishes fill types. The placement transform must propagate it.
- **Canonical `justify`**: `"<h> <v>"` (h left/center/right, v top/middle/bottom). Each reader maps its native codes (EDIF `LOWERLEFT` to `"left bottom"`, while KiCad tokens are already canonical). The SVG backend applies `text-anchor` (horizontal) and `dominant-baseline` (vertical).
- `SymbolPlacement.fields` (`Field{name, value, origin, justify, visible, ...}`) plus `PinPoint.name`: instance text (Reference, Value, custom) is structured on the placement rather than being loose sheet labels, so a consumer knows which text is which field (useful for the visual diff). Sheet labels are then only genuine free text (net labels, notes).

## Text stays readable and inside its box

Readers carry a source's text orientation and sizing faithfully, and making that text legible is the render layer's job, shared by the SVG backend and the WebGL overlay so the two agree by construction. Two rules live in the render layer and are applied by both:

- **Upright text.** No run is ever drawn upside down, the way every EDA viewer (Eeschema, Altium, OrCAD, and the tool that authored the EDIF) draws it. A run whose angle would read upside down (normalized magnitude over 90, for example a symbol placed at 180 degrees or a net label with its own 180-degree orientation) is turned a further 180 and its justify flipped on both axes, so it stays anchored to the same corner. Vertical text (plus or minus 90) is left alone, so the KiCad-90 parity holds. Without this, 180-degree-placed connectors on a real headers sheet rendered their ref-des and half their net-stub labels upside down.
- **Caption fits its box (condense, do not shrink).** A symbol caption with no source text height fell back to a fixed size and spilled past its symbol. It is now condensed horizontally to the drawn body-box width (the widest boxed rect, not the pin-stub-widened bounding box) via SVG `textLength` with `lengthAdjust="spacingAndGlyphs"`, which keeps the font height instead of dropping it to a few pixels, and only when it would overflow. The packed label carries the budget to the overlay so it condenses identically.

One gotcha: `rsvg-convert` (librsvg) silently ignores `textLength` and `lengthAdjust`, so an offline PNG of the SVG backend still shows a caption overflowing. Browsers honor it, and the viewer's SVG and WebGL output is browser-consumed, so it is correct in the product. Verify condensing in a real browser, not via an rsvg PNG or a golden.

## Fidelity

The reader declares lossy-bounded (a render subset). It is not a round-trip oracle. The schematic drawing is not reconstructed byte-for-byte, only the render subset is extracted.

## Board geometry sidecar

There are two geometries and two sidecars, one per physical medium.

- **Schematic-page geometry**: symbol shapes, wire runs, labels. This is `SchematicGeometry`, the subject above.
- **Board geometry**: component placement, pads, routed copper, vias, zones, the layer stackup, and the board outline. This is `BoardGeometry`, in the same `agni.v1.geom` package, so the primitive vocabulary (`Point`, `Polyline`, `BBox`, `Provenance`) is shared rather than duplicated.

The board sidecar follows every rule established above. It is a separate keyed artifact, never imported by diff, rules, or simulation, and joined to the netlist IR at consumption time by stable keys: `ref_des` for placements (with a pad `number` matching the connection's `pin_ref`, so `(ref_des, number)` joins a copper land to its netlist pin) and net name for routed copper (`NetCopper`, the board analogue of `WireGeometry`). The first producer is the KiCad reader, over the same s-expr parse as the netlist reader. IPC-2581 and ODB++ producers slot in behind the same proto, keeping the one-contract, N-producers property.

**Silkscreen and legend text.** `BoardText` carries the board's placed strings: each footprint's ref-des and value, plus free graphic text such as the title block, so both board renderers draw them, matching what KiCad shows. Text is universal across board formats (IPC-2581 legend, ODB++, Gerber), so it lives in the shared contract, not in a reader, and the IR never overfits one format. Every `BoardText` is board-frame absolute: the reader composes a footprint's local text offset through the placement transform with the same composer the pads use, which is what pins text to its part, and free text is authored absolute already. Glyph angle folds under KiCad's default keep-upright so text on a rotated footprint never renders inverted (free graphic text is exempt, so a deliberately mirrored back-side title stays mirrored). Hidden source text is dropped.

**Silkscreen and fab graphics.** `BoardGraphic` carries the non-copper artwork the same way: a footprint's silk and fab body outlines, courtyards, and polarity marks, plus free graphics that are not the board edge (the edge stays `BoardOutline`). It reuses `geom.Shape` for the geometry itself (polyline, rect, circle, with arcs approximated to polylines under the same lossy bound), adds only a stroke width, carries the source layer verbatim, and, like `BoardText`, pre-composes footprint graphics to board coordinates so they sit on their part. Both renderers draw them in a silk group, and per-layer visibility is the same client-side concern as the copper strata. This is universal across board formats, so it is a shared-contract field a second producer fills, not a reader's. Filled zone regions and per-side silk and fab default-visibility remain a later refinement.

Only tier 1 exists for the board today. The columnar packed transport for high-volume copper (the `PackedSheet` analogue) is deliberately deferred until its consumer, the board renderer, exists. The tier split above is the design it will follow. Coordinates are nanometers (`unit_nm=1`), Y-up, matching the KiCad schematic reader's convention. Rotations are carried verbatim from the source, and composition with the Y-flip is the renderer's concern. Fidelity is lossy-bounded (a render and DRC subset): arc tracks, zone fill polygons, teardrops, and 3D references are out, zone outlines are kept as authored, and outline arcs are approximated as polylines. What this enables is that the geometric DRC class (clearance, width, annular ring) gets its data tier, and a board viewer gets its contract.

**Copper stroke width (both renderers).** Board copper renders at its true physical width, floored to a *physical* minimum (about 25µm in board space), never to a fixed output-pixel constant. The SVG backend strokes `max(width, minStroke) * scale`, and the WebGL packer tessellates the same floored width. An output-pixel floor (an earlier fixed 0.8px) clamped every sub-pixel trace to one width on a scaled-to-fit board, merging dense copper into a blob and erasing relative trace widths. The board-space floor keeps thickness proportional to the copper at every zoom, as EDA viewers draw. WebGL was already faithful, because browser GL line width is about 1px, which is exactly why the packer draws tracks as triangle quads rather than lines, so only the SVG backend needed converging. The scope is board copper. Schematic wire strokes stay a fixed pixel width (line-art at readable zoom, no blobbing), and pad and via size-floors stay (discrete-feature visibility, a separate concern).

**Buses draw distinctly.** `WireGeometry.kind` (unset = wire, `KIND_BUS`, `KIND_BUS_ENTRY`) lets the readers flag a bus trunk or entry so both renderers style it apart from a net wire. The SVG backend strokes it thicker in the bus color, and the WebGL packer tessellates it to true-width triangle quads in a distinct bus group with the same "GL lines are about 1px, so widen via quads" path copper takes. The kind is format-neutral (KiCad sets it today from `bus` and `bus_entry`, and a bus carries no net, so its member nets stay unmodeled). The packed palette only grows to cover the bus group on a sheet that actually packs a bus, so a bus-less sheet's bytes are unchanged. A `bus-not-modeled` finding highlights its drawn trunk on click, keyed by the bus name (gated on the bus kind so a same-named net wire is not caught), via outline recolor in place, since a bus is already thick and the net-focus path shape would mis-tessellate the WebGL quads. The reader names the bus wire from its source label (KiCad's range-label-on-wire, with gEDA and xschem inline), which is also the finding subject, so the join is by name. An undrawable bus (a bus alias, an EDIF `array`, a hierarchical port with no drawn wire) shows a server-authoritative "bus not drawn" note instead, computed from the drawn-bus-name index.

### Second producer: IPC-2581, and the contract's first second-format audit

The IPC-2581 board reader is the second `BoardGeometry` producer. The proto was designed from one producer (KiCad), so the second producer is also the audit that proves the fields are not overfit: the overfit rule only bites when a second vendor's data lands in them. The audit's outcomes, all resolved without a proto change:

- **No new fields earned.** IPC-2581's stackup carries per-layer material and thickness, but no board rule or renderer consumes them, so `BoardLayer` keeps only `kind` (IPC-2581's `layerFunction` verbatim, the same discipline as KiCad's `kind` word). The richer stackup stays in the netlist IR's `ir.Stackup`, ledgered for when a consumer earns it.
- **Padstack def and instance resolve to the same inline `Pad`.** IPC-2581 references pad shapes by id from a primitive dictionary, where KiCad inlines them per footprint. The producer resolves the indirection and emits the identical flattened `Pad` (shape word, size, footprint-local position), confirming the inline mapping holds for a def-and-instance source.
- **Frame.** IPC-2581 is Y-up like the geom contract, so coordinates map directly with **no** Y negation (KiCad is Y-down and negates). Sides normalize into the contract's KiCad-style vocabulary (`TOP` to `F.Cu`, `BOTTOM` to `B.Cu`) so the format-neutral renderer's front and back classifier works unchanged, the N-producers-one-vocabulary property in miniature.
- **The board-availability gate generalized.** The check layer's `board.` gate keyed on `SourceFormat == "kicad-pcb"`, and it now tests a set of board formats. The authoritative per-design gate remains the Model's board tier: empty means the board rules stay silent.

Scope landed in two parts: placements, pads, layers, and outline first, then routed copper (tracks with interleaved straight and arc steps decoded in document order, arcs approximated as 16-chord polylines) and vias (a drilled hole marked as a via, with the co-located copper pad as the annular). That lights all four board DRC rules on IPC-2581 (track-width, copper-clearance, hole-size, annular-width).

**Full producer parity.** The second producer initially lagged the contract: fields KiCad filled sat empty on IPC. That gap is now closed, all into existing fields with no proto change. Component value goes to `ir.Component` attributes. Silk and fab graphics (marking, outline, assembly-drawing packages, composed per placement) go to `BoardGraphic` (IPC encodes silk as vector geometry, not string text, so `BoardText` legitimately stays empty for this producer, a genuine format difference). Copper plane and pour fills go to `Zone` (authored outline, cutouts dropped under the lossy bound). Via layer spans go to `Via.layer_from/to`. User-primitive pads (their own units) become real pad extents. Two gotchas the work pinned: the fill is nested under a features contour, not a direct child (a fixture test passed while the real board rendered zero zones, caught only by the corpus-render rule), and a drill layer's span lives on the layer, not the hole. Non-via copper, fill cutouts, and the padstack def-and-instance indirection remain ledgered (no consumer yet), and stackup materials are a later refinement.

**Two render-faithfulness bugs on real Allegro exports.** Both passed unit fixtures and showed only on a corpus PNG render. First, Cadence Allegro writes `clockwise="TRUE"/"FALSE"` in uppercase, and a case-sensitive `== "true"` read every clockwise arc as counter-clockwise, so it swept the long way and ballooned outline and copper arcs. Parse it case-insensitively. Second, a `Zone` is a copper pour, so the zone extraction must gate on the layer being a copper function (conductor or plane). Without the gate, document (fab and assembly-drawing) contours leaked in as copper and blew up the render bounds. The general lesson for any second producer: a construct that is copper on one layer is drawing geometry on another, so classify by the source layer function, not by the element name.

## Auto-layout node drawing

The netlist-graph fallback (`agni render --layout=grid|layered`) has no source geometry, so it synthesizes a `SchematicGeometry` from the IR. The assembly step decides what to draw at each component node through a pluggable **`SymbolSource`**, so the layout stays fixed while node artwork varies.

- **`Registry`**: classified synthetic glyphs. Classification is data-driven and user-extensible. An ordered list of class rules matches the resolved part or symbol name first, then the ref-des letter prefix on a startswith match (so `RE1` and `Cout` still classify), mapping to open string class ids each with a hand-authored glyph (resistor, capacitor, inductor, ferrite, diode, led, tvs, fuse, connector, test point, crystal, ic, transistor, ground), the same vocabulary as the check model's component-class fact used by the [rules and checks](../rules-and-checks/) layer. Only unrecognized parts draw the generic box. Multi-pin bodies (ic, connector) carry no per-pin terminals, so their edges attach at the node center like the box does. Callers layer their own rules on the defaults (`--class sym=class`, `--class-file`).
- **`FaithfulSource`**: the design's own symbols (from a geometry sidecar) re-laid-out, falling back per ref to the Registry and then the box. `--symbols=faithful` selects it. A symbol that failed to load counts as unresolved, not provided.

Two layout properties keep the output legible, both applied in assembly (no per-strategy change, because grid and layered both place on integer pitch multiples):

- **Size-aware packing**: group nodes by distinct X (columns) and Y (rows), and size each cell to `max(pitch, symbolSize + gutter)`. A uniform glyph grid stays at `pitch`, and only a large (faithful) symbol expands its own column or row, so mixed sizes pack tightly without overlap.
- **Pin-accurate edges**: a net's hyperedge star runs from each connection's *pin* (the placed node origin plus the symbol's pin point whose `port_ref` matches the connection's pin), with a node-center fallback when the symbol has no such pin. A net where three or more pins meet gets a junction dot at the centroid. Auto-layout placements carry no rotation, so `origin + pin.Loc` is the world point.

A conversion report (behind `agni render --report` and the `GetReport` web API) explains how each component mapped: its device class and whether it drew a glyph, the generic box (unmapped class), a provided symbol, or an unresolved fallback, with call-outs for the box list and the unresolved list (which points at `--symbol-path`, needed for xschem and gEDA whose symbol artwork lives in external `.sym` files).

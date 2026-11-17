---
title: "Drawing a board from agni"
description: "How a front end in any language reads a board's geometry from agni serve, page by page, and what each message means: units, layers, sides, the stack, pads, copper, and the names that map a click back to a part, pin or net."
---
Say a team draws boards in its own GUI, in C++ or Java, and wants agni's boards in it. It asks a
shared `agni serve` for the board's geometry and draws it. This page says what that geometry holds
and how to read it without reading agni's code (agni issue 1634). [Writing an agni
client](../client-contract/) covers connecting and snapshots.

## Fetching the board

`DesignViewService.GetBoardGeometry` returns the board as one `agni.v1.geom.BoardGeometry`, a page at a
time. A large board doesn't fit one message. The Jetson AGX Thor baseboard is 2.7 MB in binary
protobuf and 6.5 MB in JSON. grpc-go, grpc-java and gRPC C++ refuse a message over 4 MB unless the
client raises its limit.

1. Call it with the snapshot and an empty `page_token`. `max_bytes` bounds a page's encoded size.
   It defaults to 1 MiB and is capped at 3 MiB.
2. Merge the page's `board` into an empty `BoardGeometry` with your protobuf library's merge. A merge
   appends repeated fields, so the pages add up to the board. In Go that is `proto.Merge`, in Java
   `Builder.mergeFrom`, in C++ `MergeFrom`, in TypeScript `merge` from `@bufbuild/protobuf`, and in
   Rust prost's `Message::merge` over the page's encoded bytes.
3. Call again with `next_page_token` until it comes back empty.

The first page carries the board's units, layers, stack, outline and padstacks. Later pages carry
whole placements, then nets, zones, texts and graphics. No element is split across pages, so a
net's copper always arrives as one `NetCopper`. An element larger than `max_bytes` arrives alone on
a page of its own. A token belongs to one revision of the design, and a token from another revision
is refused with `invalid_argument`. A design with no board answers `not_found`.

Ask for binary protobuf where the transport allows it. gRPC always sends binary, and Connect sends it
when the client sets binary format. JSON works too, at about two and a half times the size.

`agni board <design>` writes the same message from the command line, as counts, as JSON, or as
binary protobuf with `--format binary -o board.pb`. A test can compare a client's merged pages with
that file.

## Units and frame

Every coordinate is an integer in source units, and `unit_nm` says how many nanometers one unit is.
The KiCad reader writes `unit_nm` 1, so KiCad coordinates are nanometers. Multiply by `unit_nm` before
you assume a unit. Widths, sizes, drills, heights and thicknesses use the same units.

The frame is Y-up, with angles in degrees counterclockwise. A GUI that draws Y-down flips Y once, at
the end.

A packed board, from `GetSheet` with the board's sheet, states the same scale as `units_per_mm`,
and its vertices sit relative to `origin_x` and `origin_y`. BoardSVG states it on its root
element as `data-mm-per-px`, `data-mm-x0` and `data-mm-y0`. [What a front end decides for
itself](../front-end-rules/#measuring-on-a-board) gives the maps.

## Layers and sides

`layers` is the board's layer table, as the file declares it. Each `BoardLayer` has a `number`, a
`name` such as `F.Cu` or `In1.Cu`, and a `kind`, the source's own word, such as `signal`, `power` or
`user`. The table holds the layers a board draws on. Every element names its layer by `name`.

`stackup` is the physical stack from the top of the board to the bottom, dielectrics included. Each
`StackLayer` has a `kind` of copper, dielectric, mask, paste, silk or other, so a GUI tells copper
from dielectric in every format. `layer` names the `BoardLayer` the row is, and is empty for a
dielectric. A thickness the file doesn't give is unset, never 0. `stackup` itself is unset when the
file states no stack.

A part sits on one side. `ComponentPlacement.layer` names that side's copper layer, `F.Cu` or `B.Cu`
for KiCad, and `mirror` is true on the back.

## Parts and pads

A `ComponentPlacement` gives a part's `ref_des`, its origin `at`, its `rotation_deg`, its side, and
its pads. A pad's position and angle are in the footprint's own frame, relative to the part's
origin, so identical footprints compare across placements. To place a pad on the board, rotate its
offset by the part's angle, mirror X when `mirror` is set, then add `at`. The pad's own
`rotation_deg` turns its land within the footprint, before the part's rotation.

A pad's `shape` and `size` give its land on its first layer. A pad that names a `padstack` has a land
per copper layer in `BoardGeometry.padstacks`, and a back-side part names an entry whose layers are
already turned over. A `drill` of 0 means a surface pad.

A part with a 3D model also lists its meshes in `model_instances` (agni issue 1652). Each names a
mesh by key and gives a transform, 12 numbers of a 3x4 matrix in row order, from the mesh's frame to
the footprint's, in millimetres with Z above the board. `GetBoardMeshes` returns the meshes, a page
at a time, each once however many parts share it. To draw a part's model, apply the instance's
transform, then place the result as a pad, and negate Z for a part with `mirror` set so it hangs
below the board. The request's `max_triangles` sets a budget for the whole board. The server
simplifies the largest meshes first, and then leaves out the smallest parts' meshes. Those parts draw
as blocks of their `height`. The first page lists both.

Text and graphics are already in board coordinates. `BoardText` and `BoardGraphic` carry the
`ref_des` of the part that owns them, or "" for free artwork. The board edge is `outline`, as the
file draws it, which is often separate lines and arcs that a GUI stitches into a polygon.

## Copper

`nets` groups routed copper by net. Each `NetCopper` holds the net's track segments, vias and filled
shapes.

- A `TrackSegment` runs from `a` to `b` on one layer, at a `width`. An arc sets `mid` to a point on
  the curve, and a straight run leaves it unset.
- A `Via` spans `layer_from` to `layer_to`. A through via spans the outer copper, and a blind or
  buried one a narrower pair.
- A `CopperShape` is copper as etched, an outline with holes.

A `Zone` is a region the designer marked to fill with a net's copper. It holds the authored outline,
which overlaps other nets' tracks. Draw a zone as intent, a hatch or an outline, and draw
`CopperShape`s as copper.

## Mapping a click back

A GUI that lets someone click the board needs to say what they clicked, in names the rest of agni
answers to. [What a front end decides for itself](../front-end-rules/) says how agni's viewer turns a
click into one subject, and what the server answers once it has.

| Clicked | Name | Use it with |
| --- | --- | --- |
| A part, or its text or graphics | `ref_des` | a component subject, a query's `?c`, `LocateSubjects` |
| A pad | `(ref_des, Pad.number)` | a pin subject, whose `pin` is the pad's number |
| A track, via, shape or zone | `NetCopper.net` or `Zone.net` | a net subject, a query's net |

The served board names a net by the schematic's name where the board and schematic name one net
differently, so a click on copper joins the schematic's net. Copper on no net has the net "".

## What it leaves out

The geometry keeps what a drawing and a geometric check need. It leaves out zone fills a design tool
computes again and teardrops. 3D model meshes come from their own call, `GetBoardMeshes`. [Geometry and rendering](../../architecture/geometry-and-rendering/)
says how each reader fills it, and `protos/agni/v1/geom/geom_board.proto` holds every field's
comment.

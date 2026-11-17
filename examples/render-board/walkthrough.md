---
title: Render a board
description: Read a KiCad board into the board geometry sidecar and render the physical copper
actors:
  - id: You
    label: You
  - id: Agni
    label: Agni engine
---

## The board sidecar, drawn

A `.kicad_pcb` carries two stories, the netlist (which pads join which nets, as in the
`read-and-stats` example) and the physical board (outline, placements with their copper
lands, routed tracks, vias, zones). Agni reads the physical story into the board geometry
sidecar (a peer of the schematic sidecar), and the render layer draws it: copper per layer,
back to front, with the front side on top. This walkthrough reads a bundled board and
renders it, then highlights one net's routed copper. The web viewer uses the same
net-to-copper join to locate findings on the board.

KiCad and IPC-2581 both produce the board sidecar, and this example reads the KiCad one.

## Pick a board {#pick}

> Give a path to a .kicad_pcb (relative to this example folder), or your own board. The
> default is a small synthetic board bundled in ../common/designs (two parts, two routed
> nets, a via, and a ground zone), so no real board ships here.

## Read the sidecar {#read}

> The reader emits nanometers, Y-up, with placements joined to the netlist by ref_des and
> copper grouped per net. The counts below are the sidecar's inventory.

## Render the board {#svg}

> BoardSVG stratifies the document into classed layers (edge, copper-front, copper-back,
> through, zones, labels), so a viewer toggles layer visibility with CSS and never re-renders.
> Open board.svg and you are looking at the physical board: outline, tracks at true width,
> pads, and vias with their drills.

## Highlight a net's copper {#highlight}

> A highlight spec that names a net matches its routed segments and vias plus every pad
> connected to it; the overlay is framed exactly like the base document, so stacking the
> two lines everything up. The web viewer lights up a clicked DRC finding on the board the
> same way.

## Stand the parts on the board {#bodies}

> A board view's stack view stands each part on its side of the board. The engine gives each
> part a footprint, the hull of its courtyard, fab outline or pads, and the height its file
> states. IPC-2581 states a height on each package. This step reads parts-board.ipc2581.xml
> from ../common/designs, where R1's package gives no height.

## Read KiCad heights from the 3D models {#model-heights}

> A KiCad board states no part heights. Each footprint names a 3D model instead, and the reader
> measures how far the STEP model reaches above the pads. It applies the footprint's own scale and
> offset first. This step reads model-heights.kicad_pcb from ../common/designs. U1 and U2 name a
> 0.9 mm block beside the board, and U2 lifts it 0.5 mm and doubles its height. R1 names KiCad's
> library, and this read passes no `--model-path`, so R1 has no height. The board's
> `unread_models` says why. U3 names `${DEMOLIB}/block.step`, a variable only its author's KiCad
> defines. The read maps `DEMOLIB` to a folder, as `--model-var` does, and finds a glTF beside the
> named `.step`, the way Antmicro publishes its library. It stands 1.5 mm tall.

## Draw each part's model mesh {#model-meshes}

> The stack view can draw each part's own model rather than a block. The heights step's read also
> names a mesh for each part that has a model. U1 and U2 draw `3d/block.wrl`, which KiCad's library
> ships beside each STEP, and U2's model transform doubles its Z and lifts it 0.5 mm. U3 draws its
> glTF. The read opens the meshes only when asked, so a check never reads one. Each mesh is read once
> for every part that shares it. A weak device asks for a small triangle budget, and the parts past
> it draw as blocks.

## Skip a value the reader cannot parse {#read-notes}

> An ODB++ job states each part's height as a `.comp_height` attribute on its CMP record. ODB++ 8
> lets the record end with `;ID=<n>` after its attributes, and the reader drops it. This step reads
> record-ids-layout from ../common/designs/odb. U1's height is 0.05 in, so it reads 1.27 mm. U2's
> height reads `tall`. The board still reads, U2 has no height, and the board's `read_notes` names
> the file, the line and the value.

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

A `.kicad_pcb` carries two stories: the netlist (which pads join which nets — the
`read-and-stats` example) and the physical board — outline, placements with their copper
lands, routed tracks, vias, zones. Agni reads the physical story into the board geometry
sidecar (a peer of the schematic sidecar), and the render layer draws it: copper per layer,
back to front, with the front side on top. This walkthrough reads a bundled board and
renders it, then highlights one net's routed copper — the same net-to-copper join the web
viewer uses to locate findings on the board.

Only `.kicad_pcb` produces the board sidecar today; the IPC-2581 producer is the sidecar's
second-format audit (a roadmap ticket).

## Pick a board {#pick}

> Give a path to a .kicad_pcb (relative to this example folder), or your own board. The
> default is a small synthetic board bundled in ../common/designs — two parts, two routed
> nets, a via, and a ground zone — so no real board ships here.

## Read the sidecar {#read}

> The reader emits nanometers, Y-up, with placements joined to the netlist by ref_des and
> copper grouped per net — the counts below are the sidecar's inventory.

## Render the board {#svg}

> BoardSVG stratifies the document into classed layers (edge, copper-front, copper-back,
> through, zones, labels), so a viewer toggles layer visibility with CSS — no re-render.
> Open board.svg and you are looking at the physical board: outline, tracks at true width,
> pads, and vias with their drills.

## Highlight a net's copper {#highlight}

> A highlight spec that names a net matches its routed segments and vias plus every pad
> connected to it; the overlay is framed exactly like the base document, so stacking the
> two lines everything up. In the web viewer this is how a clicked DRC finding lights up
> on the board.

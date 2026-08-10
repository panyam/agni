---
title: "3. See it"
description: "Draw the board, and get a picture of a netlist that has no drawing."
---

A finding names a net. Nobody thinks about a board as a list of net names, so at some point you need
a picture. There are two different situations here and they need different answers.

If your design carries its own geometry, the tool draws that geometry, and the picture is the one
your CAD tool would draw. If your design is a netlist, there is no geometry to draw, because a
netlist records what is connected and never records where anything sits. In that case the tool
computes a layout from the connections.

The tutorial board is the second case. `gateway.edn` is an EDIF netlist.

## Draw a netlist

```
agni render designs/gateway/gateway.edn --layout layered -o gateway.svg
```

```
layout "layered": 19 nodes, 15 nets, 56 segments, 95 crossings, edge length 20826
wrote gateway.svg (sheet "netlist graph", 19 placements, 15 wires)
```

The line it prints is a quality report on the drawing, not on your board. `crossings` counts wires
that cross each other, which is the main thing that makes a generated schematic hard to follow.

This is a drawing of your netlist, not a reproduction of your schematic. Parts sit where the layout
algorithm put them. It is for following connectivity, not for review of the drawing itself.

## Pick a layout

There are five, and which one reads best depends entirely on the board. Rather than guess:

```
agni render designs/gateway/gateway.edn --compare
```

```
layout      nodes  nets  segments  crossings  bends  edge-length  stress
force       19     15    56        43         0      9052         0.346
grid        19     15    56        133        0      7798         0.486
layered     19     15    56        95         0      20826        0.582
orthogonal  19     15    84        104        28     9200         0.457
stress      19     15    56        73         0      7218         0.457
```

For this board `force` has by far the fewest crossings, and `grid` is the worst by that measure
while being the most compact. `orthogonal` is the only one that bends wires into right angles, which
is what a schematic normally looks like, at the cost of more segments and more crossings.

Lower `stress` means the drawn distances better match how far apart things actually are in the
connection graph. There is no single winner. Render the two or three that score well and look at
them.

## Faithful geometry

When the design does carry geometry, drop `--layout` and you get the design's own drawing:

```
agni render my-board.kicad_sch -o board.svg
```

That is the default. `--layout` is what you reach for when there is nothing to be faithful to.

## In the browser

`agni serve` hosts a viewer and the web API on one port:

```
agni serve . --addr :8080
```

Point it at a folder and it lists the designs it can read. The viewer pans and zooms, and its panels
run the same checks the CLI runs, over the same catalog, so the findings you saw in rung 2 appear
against the drawing rather than as a list. Later rungs add tiers to that catalog, and the viewer
picks them up the same way the CLI does.

## Next

Rung 4, which is where the tool stops being generic and starts knowing your team's conventions, is
still being written. Until it lands, [Naming conventions](../../guide/naming-conventions/) in the
guide covers the same ground.

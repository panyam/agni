---
title: "3. See it"
description: "Draw the board, and get a picture of a netlist that has no drawing."
playground: viewer
---

A finding names a net. Nobody thinks about a board as a list of net names, so at some point you need
a picture. There are two different situations here and they need different answers.

If your design carries its own geometry, the tool draws that geometry, and the picture is the one
your CAD tool would draw. If your design is a netlist, there is no geometry to draw, because a
netlist records what is connected and never records where anything sits. In that case the tool
computes a layout from the connections.

The tutorial board is the second case. `gateway.edn` is an EDIF netlist.

## Draw a netlist

{{ agniRun "content/tutorials/runs/03-render-layered.yaml" }}

The line it prints is a quality report on the drawing, not on your board. `crossings` counts wires
that cross each other, and crossings are what mainly make a generated schematic hard to follow.

This is a drawing of your netlist, not a reproduction of your schematic. Parts sit where the layout
algorithm put them. It is for following connectivity, not for review of the drawing itself.

<agni-viewer src="{{.Site.PathPrefix}}/static/designs/gateway-netlist.svg"
             caption="gateway.edn with a computed force layout: no geometry came from the file, every position here was calculated"></agni-viewer>

Compare it against the faithful drawing further down. Same board, same nets, and a completely
different picture, because one was drawn by a person and the other was solved for.

## Pick a layout

There are five, and which one reads best depends entirely on the board. Rather than guess:

{{ agniRun "content/tutorials/runs/03-render-designs-gateway-gateway-edn-compare.yaml" }}

For this board `force` has by far the fewest crossings, and `grid` is the worst by that measure
while being the most compact. `orthogonal` is the only one that bends wires into right angles, which
is what a schematic normally looks like, at the cost of more segments and more crossings.

Lower `stress` means the drawn distances better match how far apart things actually are in the
connection graph. There is no single winner. Render the two or three that score well and look at
them.

## Faithful geometry

When the design does carry geometry, drop `--layout` and you get the design's own drawing. The
tutorial board ships a KiCad view of itself for exactly this:

{{ agniRun "content/tutorials/runs/03-render-kicad-sch.yaml" }}

That is placements and wires read out of the file rather than computed, so the result is the drawing
somebody drew. Faithful is the default. `--layout` is what you reach for when there is nothing to be
faithful to.

<agni-viewer src="{{.Site.PathPrefix}}/static/designs/gateway-schematic.svg"
             caption="gateway.kicad_sch rendered faithfully: every position, wire, and label came out of the file"></agni-viewer>

## The same board, twice

`gateway.edn` and `gateway.kicad_sch` are two views of one design, which raises the obvious
question of whether they still agree. Ask directly:

{{ agniRun "content/tutorials/runs/03-diff-views.yaml" }}

Zero net changes. The two readers converged on the same netlist, and the whole engine rests on that
premise: analysis runs over one internal representation, so the format you started from stops
mattering once the file is read.

The nineteen changed components are library-qualified part-type names, which differ because each
format names its libraries its own way. That is a difference in the files, not in the board.

This is also the practical way to check a CAD migration. Export from the old tool and the new one,
diff the two, and an empty net delta is real evidence the design survived the move.

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

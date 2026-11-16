---
title: "Seeing the board's layer stack"
description: "Draw a board's layers apart in stack order, see which inner layer carries a net and which layers a via joins, and click a track on any layer to find its net on the schematic."
---

Say a net leaves a connector on the top copper and vanishes. You want to know which inner layer it
runs on, and which layers its vias join. A flat board view draws every layer on top of the others,
so an inner track hides under the layers above it. The stack view draws the layers apart instead.

![Jetson's layer stack, drawn apart, with the In2.Cu track of a selected net lit]({{.Site.PathPrefix}}/static/images/guide/board-stack.png)

## Open the stack view

Open the board beside the schematic, as [Cross-probing](../cross-probing/) describes. Press **Stack**
in the board view's bar. The view draws each layer as a plane, from the top silkscreen down to the
bottom one, in the order the board's file stacks them. Each dielectric and solder mask draws as a
pale slab between the copper, and each plated hole draws as a barrel between the layers it joins.
So a blind via from F.Cu to In1.Cu stops at In1.Cu, and a through via runs the whole board.

Press **Stack** again to go back to the flat board. It comes back as you left it, with the same pan
and zoom, side and hidden layers, and in SVG if it was drawing in SVG. The stack view draws in WebGL,
so the button is off in a browser without WebGL2. It is off too for a board whose file states no
stack, such as a hand-written ODB++ job with no layer thicknesses.

## Move around it

- Drag to turn the stack, and Shift-drag to move it.
- Scroll to zoom in on a point.
- **Top**, **Iso**, **Front** and **Side** turn it to look from above, from a corner, at the front
  edge and at the left edge.
- The slider spreads the layers apart. At 0 the stack draws as thick as the board really is, which
  on Jetson is 1.711 mm against a board some 100 mm wide, so it reads as one sheet. Further right,
  every gap grows by the same factor, so a 0.2 mm core still draws twice as tall as a 0.1 mm prepreg.

The **Layers** list hides and shows layers here as it does on the flat board. Hide the top copper and
the layers under it show through.

## Click a track on any layer

Click a track on an inner plane, and the schematic selects its net, as a click on the flat board
does. The click goes to the nearest plane with something under the pointer, so an inner track picks
where nothing on a nearer layer covers it. Click a via's barrel between the planes, and the schematic
selects its net too. A selected net lights on every layer it runs on, its vias' barrels included.

The silkscreen's text sits on its layer, at whatever angle you turn the stack to. The top's text reads
from above and the bottom's from below, so neither shows through the board. Once the view stops
moving, a label that a part's block hides from where you look hides too, a whole label at a time.
Every label shows while you turn the view, and none hides with **Parts** off.

## See the parts on the board

Each placed part stands on its side of the board as a solid block, front parts on the top face and
back parts hanging off the bottom one. The block's outline is the part's courtyard, or its fab
outline, or the area its pads cover. An ODB++ part's fab outline is its package outline, so how
close it comes to the part depends on the tool that wrote the job. Altium and Allegro draw the body,
and KiCad's export draws the area the pads cover. An Altium part's block stands on its 3D bodies'
outlines, and an Allegro part's on its place-bound outline, so both match the outlines their tools
export. A part stands as tall as the board's file says it
is. Click a
part's block and the schematic selects that part, as a click on its pads does. Untick **Parts** in
the bar to hide the blocks and see the layers under them.

![Parts standing on the top face of Jetson's cut at the board's true thickness, with the selected R119 lit]({{.Site.PathPrefix}}/static/images/guide/board-stack-parts.png)

A part keeps its true height when the slider spreads the layers, so it does not grow with the
stack.

IPC-2581, ODB++, Altium and Allegro boards give each part its height, so their parts stand at their
real heights. An Altium part takes the height its component states, or its tallest 3D body's when it
states none. An Allegro part takes the height on its place-bound outline. KiCad keeps a part's height only in its 3D model, and agni does not read the model yet,
so a KiCad part stands at 1 mm in a paler grey. The note beside **Parts** counts the parts at that
default, as "(all at a default 1 mm)".

Click a part's block on its top or on a side. Seen edge on from **Front** or **Side**, its side
picks the part.

## See the board as it is made

**Layers** and **Board** in the bar switch the stack view's look. It opens in Layers, which draws
each layer apart in the editor's colours. Board draws the board as it is made. An
opaque green mask covers each face, the tracks show as a lighter green through it, and the pads draw
in gold. The silkscreen and its text draw white, and the inner layers and the dielectrics hide. The
board's edge draws as a darker wall from one face to the other, round the outline and each hole cut
in it. The slider and the camera work the same in both, and a click picks the same part or net.

Each look cuts a slot in the board's outline out of the board, and a drilled hole 1 mm or wider too,
as a mounting hole is. A mounting hole stands no block, since it is a hole and not a part. agni takes
a footprint for a mounting hole when its pads are all one drilled pin, at least 2 mm wide.

![Glasgow revC3 in the Board look, with its parts coloured by type]({{.Site.PathPrefix}}/static/images/guide/board-stack-look.png)

Each part's block takes a colour from its type, in both looks. ICs and transistors are near black,
resistors dark grey, capacitors tan, connectors ivory, LEDs white and test points gold. A part agni
can't classify stays plain grey. A block at the default height draws a little paler than one whose file
gives a height.

## When the file leaves a thickness out

Each layer draws as thick as the board's file says. A layer with no thickness draws at a typical
value for its kind, 0.035 mm for copper and 0.1 mm for a dielectric. The **Stack** table under the
**Layers** list says so on that row, as "(drawn at 0.1 mm)". Silkscreen and paste draw on the
surface, since KiCad gives them no thickness and a board's stated thickness leaves them out.

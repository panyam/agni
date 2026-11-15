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
stack, such as an Allegro or Altium board today.

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
where nothing on a nearer layer covers it. A selected net lights on every layer it runs on, its vias'
barrels included.

## When the file leaves a thickness out

Each layer draws as thick as the board's file says. A layer with no thickness draws at a typical
value for its kind, 0.035 mm for copper and 0.1 mm for a dielectric. The **Stack** table under the
**Layers** list says so on that row, as "(drawn at 0.1 mm)". Silkscreen and paste draw on the
surface, since KiCad gives them no thickness and a board's stated thickness leaves them out.

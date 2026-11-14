---
title: "Cross-probing a schematic and its board"
description: "Open a design's board beside its schematic, click a part in one and find it in the other."
---

Say you are chasing a fault with the schematic on one side of the screen and the board on the
other. You click a decoupling capacitor on the schematic, and the board view finds its footprint.
The capacitor sits on the bottom of the board, so the board view turns over to show the bottom.

## Open the board beside the schematic

Open the design in the viewer with `agni open <design>` or from the server's file browser. The
canvas shows a schematic sheet. In the Sheets panel, open the Board tile beside the canvas. The board
now sits in a view of its own, and you can dock or float it anywhere.

A click on a part, a pin or a net in either view selects it in both. The other view opens a sheet
that draws it and centres on it. A view whose link is `off` lights the selection where it is and
does not move.

## The board turns to what you select

The board view's bar has a **Follow side** checkbox, on by default. While it is on, each selection
made elsewhere on the page changes the board view in two ways.

- **It turns to the selection's side.** A part on the bottom turns the board over, so you see it from
  underneath. A part on the top turns it back. A selection on both sides keeps the side you have. So
  does a part whose board does not say which side it is on, and so does a net.
- **It shows the copper layers the selection uses.** A part shows its side's copper and silkscreen.
  A net shows every copper layer its tracks, pours and single-layer pads run on, inner layers
  included. Vias and through-hole pads add no layer, since each runs through the whole stack and
  the Through row draws it.

The board view only turns layers on. A layer you hid by hand stays hidden until a selection uses it.
A side you turned to by hand, with the flip buttons, holds until the next selection.

Clear **Follow side** to keep the side and the layers you chose. The board view then behaves as
it did before the checkbox existed. Every board view shares the choice, and the browser keeps it
across reloads.

The sides and layers come from the engine. `LocateSubjects` reads the board and answers each
selection's side and copper layers along with the sheets that draw it (agni issue 1587).

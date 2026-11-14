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

## Step back to what you selected before

Say you follow a fault from a connector pin to a protection diode, then to a regulator, then to its
feedback divider. You want to get back to the connector. Press `[` to step back one selection at a
time, and `]` to step forward again. **← Back** and **Forward →** in the canvas bar do the same, and
so do the arrows in each sheet view's bar. Each step selects that part or net again in every view, and
each view moves to it as its link says.

The viewer keeps the last 100 selections. A new selection made after stepping back drops the ones
ahead of it, as a browser's Back does. Clearing the selection with Escape adds nothing. Opening
another design starts again (agni issue 1588).

## Step through the sheets that draw the selection

Say a net crosses six sheets. Press `.` to open the next sheet that draws it, and `,` to go back.
The view centres on the net each time and reads where you are, such as "3 of 6". The **‹** and **›**
buttons beside that count do the same. The steps follow the design's sheet order and stop at the
first and last sheet. The board is never a step. An area selection steps through every sheet that
draws any part or net in it. With nothing selected, the same keys move to the next and previous
sheet (agni issue 1589).

The keys step the view you last pressed in. That is the canvas until you press in a sheet view
beside it. Then that view steps, and the canvas does not move.

## Keys

| Key | What it does |
| --- | --- |
| `[` | Step back to the previous selection |
| `]` | Step forward to the next selection |
| `,` | Open the previous sheet that draws the selection |
| `.` | Open the next sheet that draws the selection |
| Escape | Clear the selection |

None of these keys does anything while you are typing in a text field, such as the query box. A key
held with Ctrl, Cmd or Alt is left to the browser, so Alt+Left and Cmd+[ still go back a page.

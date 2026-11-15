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

## Read how thick each layer is

Say a net runs on In2.Cu and you want to know how far under the surface it sits. Press **Layers** in
the board view's bar. Each row the board's stack names states its thickness, such as "0.035 mm",
or "thickness unknown" when the stack gives none. Copper, mask and silkscreen rows have one, and Edge,
Through and the fab rows have none. Under the
rows, a **Stack** table lists the whole board from the top down, the dielectrics between the copper
included. Its last line says whether the layers add up to the board thickness the file states. On
Jetson it reads "1.711 mm, as the file states". It turns red when the two differ by more than 10%.
The Board tile in the Sheets panel shows the same list for the canvas.

The values come from the board's stack, which `agni stackup <design>` prints too (agni issue 1603).
KiCad, ODB++ and IPC-2581 boards carry one. An Allegro or Altium board carries none yet, so its list
states no thickness.

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

## Check that the schematic and the board agree

Say you click a resistor on the schematic and nothing lights on the board. The board may lack the
part, or the cross-probe may have failed, and the two look the same. Before you trust a click, open
the agreement control at the right of the canvas bar. It compares the design's schematic with its
board and says how much of each they share, such as "146 of 149 parts on the board". Afterwards the
button reads the short form, such as "146/149 parts · 160/167 nets", until you open another design.

The comparison follows three rules.

- **Parts** match by ref-des.
- **Nets** match by name first. A net the two views name differently matches when both join exactly
  the same pins. Nothing is fuzzy, so a net with one pin more or less does not match by its pins.
- **Pins** match by ref-des and pin number, on the parts both views place. A pin counts once it is on
  a net in each view.

A power symbol's pin is no part, so the comparison leaves it out, and it leaves out a net that joins
no pin.

The panel lists what did not match.

| List | What it means |
| --- | --- |
| Parts only on the schematic | The board has no footprint with that ref-des, so the part is not placed yet or was renamed |
| Parts only on the board | A footprint with no schematic part, such as a mounting hole or a fiducial |
| Nets only on the schematic | No board net has that name or those pins |
| Nets only on the board | No schematic net has that name or those pins |
| Nets whose pins differ | Both views have a net of that name, with different pins. The entry says which pins the board adds or lacks |
| Nets named differently | The views name a net two ways and join the same pins, so the net matches |
| Pins on a net only on the schematic or the board | The pin is on a net in one view, and its pad or pin joins no net in the other |

Click an entry to select it in the view that draws it. A schematic entry moves the canvas, as a step
back does. A board entry lights in the board view, and a board view opens if none is.

A net named differently cross-probes like any other. The board view draws it under the schematic's
name, so a click on either side lights it on both. Clay R6's Allegro board is an example. Allegro
cuts a net's name to 31 characters, so 24 of its nets are named differently on the board, and each
one still lights across.

A design with no board, and a board opened on its own, read "not compared" with the reason. They
never show a count. The engine compares the views when you open the panel, or when a board view first
draws beside the schematic.

`agni diff <design>` prints the same counts and lists, and `--format json` gives the engine's answer
(agni issue 1590).

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

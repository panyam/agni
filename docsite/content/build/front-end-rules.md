---
title: "What a front end decides for itself"
description: "Which answers a front end in any language reads from agni serve, and the few rules it applies itself, each written out with the fixture that holds agni's own viewer to it: picking, highlighting a packed sheet, choosing and stepping sheets, following a selection on a board, and drawing the stack in depth."
---
Say a team builds its own board viewer in C++ against a shared `agni serve`. A reader clicks a
capacitor on the schematic, and the team's board view should turn to the back, show B.Cu, and light
the part, as agni's own viewer does. Most of that comes from the server. This page lists what the
server answers, then the rules a front end still applies itself, so two front ends behave the same
(agni issue 1635). [Writing an agni client](../client-contract/) covers connecting, and [Drawing a
board from agni](../board-geometry/) covers the board's geometry.

The engine answers, and a client draws. A decision moves to the server unless it runs on every
frame, or depends on what one view shows right now. Serving those
would add a round trip to a click or a drag, and on the in-browser engine a call can wait behind a
busy worker.

## What the server answers

| A front end wants to know | The server answers it in |
| --- | --- |
| Which sheets draw a part, pin, net or bus | `DesignViewService.LocateSubjects`, `SubjectSheets.sheet_ids`, schematic sheets in design order and then `board` |
| Why a subject is drawn nowhere | `SubjectSheets.locate_reason` |
| Which side a board view turns to, and which layer rows it shows | `LocateSubjectsResponse.board_follow`, for the whole selection, with the board read |
| Which sheets a finding or a verdict is drawn on | `Finding.sheets`, `Verdict.sheets`. An empty list means drawn nowhere. |
| Each stack row's depth below the top face | `BoardStackView.rows[].top_mm`, `bottom_mm`, with `thickness_defaulted` where the file gives no thickness |
| The depths a plated hole runs between | `BoardStackView.drills[].top_mm`, `bottom_mm`, unset when the stack cannot place the span |
| How tall each part's body draws | `BoardPartBody.drawn_height_mm`, with `height_defaulted` where the file states no height |
| Whether the schematic and the board agree | `DiffService.DiffDesigns` with `views_snapshot` |
| What a highlight lights | `DesignViewService.HighlightSheet`, as an SVG overlay or as packed primitive indices |
| Where a drawn point is on the board, in millimetres | `PackedSheet.units_per_mm` with `origin_x` and `origin_y`, or the `data-mm-per-px`, `data-mm-x0` and `data-mm-y0` attributes on BoardSVG's root |

A board view applies `board_follow` and keeps the reader's own choices. It turns to `side` when
`side` is set, and turns on each row `layers` names. It never hides a row. A "Follow side" control
that is off leaves both alone.

## Picking

Picking turns a click into a subject. It runs on every click, so the front end does it.
[Picking](../../architecture/web-picking/) describes agni's two renderers in full. A front end that
wants to pick as they do applies these rules.

- **A click is a press and release within 3 CSS pixels** (`CLICK_SLOP_PX`). A longer move is a pan.
- **A click samples nine probes**, the point itself and then the eight points 5 CSS pixels out
  across and diagonally (`PROBES`, `PROBE_R`). The ring gives a small target some tolerance, and a
  click between two close wires still misses.
- **The most specific candidate wins.** Among everything the probes hit, a pin beats a component,
  a component beats a bus, and a bus beats a net (`PRIORITY` in `web/src/selection.ts`). Between two
  of one kind, the nearer probe wins. A symbol body is drawn over its own pins, so topmost-wins
  would always give the body.
- **A part on a board picks from its pick area**, the footprint's body rect, which no renderer draws.

The server never picks, so no fixture twins these rules. `web/src/selection.test.ts` and
`web/src/canvaspick.test.ts` hold agni's viewer to them.

## Highlighting a packed sheet

A front end that draws `GetSheet`'s packed sheet can ask `HighlightSheet` for each highlight, or
resolve highlights itself against the sheet's `PrimitiveKey`s and skip the round trip. agni's
viewer resolves them itself, because a highlight changes on every hover. To match the server, a
front end lights these primitives for a spec.

- every primitive keyed to a listed component, its symbol graphics and pins alike;
- every primitive keyed to a listed net name, or to a listed net id;
- every primitive keyed to a listed bus;
- the one primitive keyed to a listed (ref-des, pin) pair, and not the same pin number on another
  part.

It leaves out a part's pick area (packed groups 254 and 255), which nothing draws.

`core/render/testdata/highlight_twin.json` records what `render.HighlightPacked` lights on the
Jetson cut's packed board for a part, a pin, a busy net, a part and a net together, and names the
board lacks. Its keys list each primitive's packed group, so a front end's test can replay the
cases without the packed sheet. `TestHighlightTwin` rewrites the file under `-update`, and
`web/src/highlights.test.ts` holds the viewer's `highlightSheet` to it.

## Choosing a sheet and stepping through sheets

Both rules depend on the sheet a view shows right now, so the front end applies them. The sheets
they choose among come from `LocateSubjects`.

- **A view that follows a selection keeps its sheet when that sheet draws it.** Otherwise it
  opens the first sheet that does, in the design's order, and offers the rest. A view showing the
  board never leaves it for a schematic sheet. When the board does not place the selection, the view
  says so and offers the sheets that draw it (`chooseSheet` in `web/src/sheetfollow.ts`).
- **Stepping walks the sheets that draw the selection**, in the design's order, leaving out the
  board. With nothing selected, it walks every sheet. A step stops at either end rather than
  wrapping. From a sheet that is not a stop, next is the first stop after it and previous the last
  one before it, and a view reads "3 of 6" on a stop or "6 sheets" off one (`sheetStep` in
  `web/src/sheetstep.ts`).

## Drawing the stack in depth

The server places every row, hole and body in millimetres below the board's top face. A front end
turns those depths into its own camera's coordinates. agni's viewer centres the stack on z = 0 with
the top face highest, and spreads it with an explode control that scales every depth by one factor,
so each gap keeps its proportion (`stackZ` and `perMm` in `web/src/stacklayout.ts`). A part's body
draws at the board's true scale whatever the explode, so a part never towers over the board.
`core/render/testdata/jetson_cut_stack_twin.json` holds the Jetson cut's served rows, for a test
that draws them.

## Measuring on a board

A measuring tool maps the cursor to the board on every mouse move, so a front end does it itself.
The server says how. A packed board's vertex plus `origin_x` and `origin_y`, divided by
`units_per_mm`, is a position in millimetres in the board's own frame, Y up. A BoardSVG point
(px, py) in viewBox units lies at `data-mm-x0 + px * data-mm-per-px` and
`data-mm-y0 - py * data-mm-per-px`. agni's viewer snaps a click on a pad to the pad's centre, the
middle of the land's bounds, since every land is a disk or a quad centred on its pad. BoardSVG writes
pad geometry to a thousandth of a pixel for that snap. `web/src/measure.ts` holds both maps, and
`TestBoardPadsSitWhereKiCadPutsThem` and `TestBoardSVGStatesItsMillimetres` hold them to kicad-cli
(agni issue 133).

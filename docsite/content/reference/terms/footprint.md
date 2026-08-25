---
title: "Footprint"
label: "footprint"
summary: "The pattern of copper pads on the board that one part solders onto, together with the outline printed beside it and the area around it that has to stay clear."
level: EE1
---

Every part on a board lands on a patch of copper shaped for it. That patch is the footprint: one pad
per leg, in the right places and at the right size, plus the outline printed on the silkscreen and the
area around it that nothing else may occupy.

{{ includeFile "figures/footprint.svg" }}

It is the layout's half of a part. The schematic carries a symbol, which says what the part is and what
its pins are called. The layout carries a footprint, which says where its metal goes. Neither can be
derived from the other, because a symbol has no dimensions and a footprint has no idea what any of its
pads do.

Two other things get called a footprint, and the confusion costs real boards.

The first is the **package**, the moulded body the vendor ships. That one belongs to the vendor. The
footprint is what you draw to receive it, so a single package has several valid footprints. A
hand-soldered prototype wants longer pads than a reflow line does, and a board that has to survive
vibration wants more copper under the part than one sitting on a desk. Ordering the right package and
drawing the wrong footprint leaves you with parts that will not sit on the board.
[Pins and packages](../../pins-and-packages/) is the longer version of that story.

The second is the [reference designator](../reference-designator/), which names the part rather than
describing its metal. The two are printed next to each other on the silkscreen, and that is most of
why they get merged in conversation. The designator is also what joins the schematic to the board.
`R5` on the sheet becomes `R5` on the layout, and the layout's `R5` is the thing that carries a
footprint.

Because the designator is the join, the rules that protect the join are the ones a footprint depends
on. [`unannotated-components`](../../rules/unannotated-components/) reports parts still carrying a
placeholder like `R?`, and a part with no name has no line on the BOM and no footprint waiting for it.
[`duplicate-ref-des`](../../rules/duplicate-ref-des/) reports two distinct parts claiming one name,
which merges two footprints the layout cannot tell apart.
[`unconnected-component`](../../rules/unconnected-component/) catches the part wired to nothing, which
still occupies its footprint's area and still costs a line on the BOM.
[`symbol-unresolved`](../../rules/symbol-unresolved/) is the schematic-side version: a symbol file that
did not open leaves a placement with a designator and no pins at all.

A [test point](../test-point/) is the smallest footprint there is, one pad with no part above it.

Agni models the footprint itself in the [physical tier of the
IR](../../../architecture/ingestion-and-ir/), which is provisional. The `Footprint` message exists and
no reader fills it in yet. What the board readers do produce is the geometry a footprint places, the
pads and the silkscreen and the courtyard, keyed back to the part by its designator.

**Where the course teaches it:** nowhere yet. The word appears once, inside
[The decision procedure](../../../learn/01-what-a-board-is-made-of/#the-decision-procedure-ee3), where
a resistor between two signals might be "a footprint nobody stuffed", and the course never says what
one is.

---
title: "Footprint"
label: "footprint"
summary: "The pattern of copper pads on the board that one part solders onto, together with the outline printed beside it and the area around it that has to stay clear."
level: EE1
---

Every part on a board lands on a patch of copper shaped for it. That patch is the footprint: one pad
per leg, in the right places and at the right size, plus the outline printed on the silkscreen and the
area around it that nothing else may occupy.

<svg viewBox="0 0 555 300" role="img" aria-labelledby="fp-title" style="width:100%;height:auto;font-family:inherit"><title id="fp-title">Top-down view of an eight-pin footprint: eight copper pads in two rows of four, the silkscreen body outline between them, a pin-one marker, the dashed courtyard around everything, and the part's legs landing on the inner half of each pad.</title><rect x="142" y="80" width="276" height="152" fill="none" stroke="currentColor" stroke-width="1" stroke-dasharray="5 4" opacity="0.35"/><g fill="var(--accent-color)" fill-opacity="0.35" stroke="var(--accent-color)" stroke-width="1.2"><rect x="150" y="96" width="52" height="18"/><rect x="150" y="130" width="52" height="18"/><rect x="150" y="164" width="52" height="18"/><rect x="150" y="198" width="52" height="18"/><rect x="358" y="96" width="52" height="18"/><rect x="358" y="130" width="52" height="18"/><rect x="358" y="164" width="52" height="18"/><rect x="358" y="198" width="52" height="18"/></g><g fill="currentColor" opacity="0.4"><rect x="176" y="101" width="26" height="8"/><rect x="176" y="135" width="26" height="8"/><rect x="176" y="169" width="26" height="8"/><rect x="176" y="203" width="26" height="8"/><rect x="358" y="101" width="26" height="8"/><rect x="358" y="135" width="26" height="8"/><rect x="358" y="169" width="26" height="8"/><rect x="358" y="203" width="26" height="8"/></g><g fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.6"><rect x="202" y="88" width="156" height="136"/><circle cx="218" cy="104" r="5"/></g><text x="280" y="164" text-anchor="middle" fill="currentColor" font-size="13" opacity="0.45" letter-spacing="1">U1</text><g fill="currentColor" font-size="11" text-anchor="middle" opacity="0.75"><text x="164" y="109">1</text><text x="164" y="143">2</text><text x="164" y="177">3</text><text x="164" y="211">4</text><text x="396" y="109">8</text><text x="396" y="143">7</text><text x="396" y="177">6</text><text x="396" y="211">5</text></g><g stroke="var(--accent-color)" stroke-width="1.2" fill="none" opacity="0.9"><polyline points="218,99 218,60 150,60"/><polyline points="280,88 280,62 400,62"/><line x1="150" y1="139" x2="104" y2="139"/><line x1="142" y1="207" x2="104" y2="207"/></g><g fill="var(--accent-color)" font-size="11.5" font-weight="600"><text x="144" y="64" text-anchor="end">pin 1 marker</text><text x="406" y="66">silkscreen outline</text><text x="98" y="143" text-anchor="end">copper pad</text><text x="98" y="211" text-anchor="end">courtyard</text></g><text x="280" y="282" text-anchor="middle" fill="currentColor" font-size="11.5" opacity="0.6">the copper an 8-pin part solders onto, seen from above</text></svg>

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

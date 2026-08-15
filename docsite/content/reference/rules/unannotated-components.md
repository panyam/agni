---
title: "unannotated-components"
description: "Parts still carry a placeholder designator instead of an assigned one."
---

### What it means

Parts whose reference designator is still a placeholder — `R?`, `C?`, `REF**`, or a partly-assigned
`C?1845` — rather than an assigned name. Annotation is the step where a tool walks the sheets and
turns every `R?` into `R1`, `R2`, `R3`. Until it runs, or for parts added after it last ran, the
placeholder is what the file carries.

The parts are drawn, placed and connected. Only their names are missing.

### Why engineers want it

A designator is the join key between three different things: the symbol on the sheet, the line on
the BOM, and the footprint on the board. A part with a placeholder has none of them. You cannot
order it, you cannot find it on the board, and a diff cannot follow it from one revision to the next.

The reason this needs a rule rather than being obvious is that an unannotated design does not look
broken. Everything is drawn. The netlist is complete. Nothing errors. The design reads as finished
because, electrically, it nearly is.

### Impact

Beyond the missing BOM line, a placeholder actively degrades other checks, because it is shared by
every unannotated part of its kind. On one export 176 distinct resistors were all called `R?`, so
any rule keying on the designator saw one impossible part rather than 176 real ones.

`pin-net-conflict` declines to judge them for exactly this reason (see its second suppression):
`(R?, 1)` does not name a pin, so "a pin belongs to exactly one net" has nothing to say about it.
That is the right call, but it means those parts would be covered by nothing at all if this rule did
not exist — and no findings would read as "annotated" rather than "not looked at".

### What to do about it

Run the annotation step in the authoring tool, then re-export. This is expected mid-design and is
worth failing on before release.

### Query structure

report each placeholder designator the reader collected.

    select U in unannotated_components

One finding per PLACEHOLDER, not per part: "176 parts are still called R?" is the reviewable fact,
where 176 findings would be the same sentence 176 times. The count rides in the message because the
placeholder alone does not say how much of the design is unnamed.

Reads: unannotated_component. Tier P.

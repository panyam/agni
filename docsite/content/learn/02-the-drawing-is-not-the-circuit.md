---
title: "2. The drawing is not the circuit"
description: "A schematic is a rendering of a netlist, and the two can disagree. The ways they do, and which ones a tool can catch."
---

Here is a question that sounds trivial and is not: **are these two pins connected?**

You would think you could tell by looking. Two wires meet on the page, so they are joined. A wire runs from a pin to somewhere, so that pin is on that net. Neither of those is reliably true, and the gap between what a schematic *looks like* and what it *means* is where a particular class of expensive bug lives.

**Prerequisites:** EE1, and [chapter 1](../01-what-a-board-is-made-of/) for what the parts are doing.

## The drawing is a rendering (EE2)

The thing a CAD tool actually exports, and the thing a board gets built from, is a **netlist**: a list of nets, each naming the pins that share it. The picture on your screen is one rendering of that netlist, drawn by a human for other humans.

Most of the time the two agree, which is what makes the disagreements dangerous. When the drawing shows a connection the netlist does not have, every human in the review chain sees the connection, because they are all reading the same picture. The board comes back and the connection is absent.

This chapter is about those disagreements. A tool that reads the netlist looks at the thing that gets built rather than the thing that got drawn, and that alone justifies having one.

## One dot changes the circuit (EE2)

The clearest case. Two schematics, three components each, the same wires in the same places. They differ by a single construct, a junction dot placed where a tap wire meets a horizontal one. (Their title blocks differ too, so you can tell which file you are looking at.)

{{ agniRun "content/learn/runs/tjunc-nets.yaml" }}

Same parts, different circuit. One file has five nets and the other has four.

The reason is a KiCad convention that trips up nearly everyone the first time. **A wire crossing or touching another wire is not a connection.** Only a junction dot makes it one. That rule exists so you can route a wire across a busy sheet without accidentally shorting it to everything it passes over, which is genuinely useful, and it means a drawing that looks connected may not be.

Ask what actually changed:

{{ agniRun "content/learn/runs/tjunc-diff.yaml" }}

The net `TAP` is gone, because it merged into `BUS`. And `BUS` gained `R3.1`, a pin it did not have before. In the version without the dot, R3 sits on a net of its own that reaches nothing else on the board. In the version with it, R3 is on the bus.

That is the failure mode in full. The author drew a T, meaning "connect these". The reviewer saw a T and read it the same way. The netlist has two separate nets, and R3 does nothing.

Here is what the rule says about each:

{{ agniRun "content/learn/runs/tjunc-verdicts.yaml" }}

Worth noticing that the second run reports a **pass** rather than saying nothing. That is recent (agni issue 420), and it was not free: the reader was computing which taps were joined and throwing that half away, because the step that makes a dot electrically real also erases the evidence that a tap was ever there. Before that, a sheet whose taps all had their dots produced exactly what a sheet with no taps produced, which is silence. A pass you can count is worth more here than almost anywhere else in the catalog, because this defect is invisible in review by construction.

## Names are wires too (EE2)

The opposite problem. A schematic can also **hide** a connection that is genuinely there.

Two pins on opposite corners of a sheet, with no wire drawn between them, are connected if they carry the same label. Labels are how anyone keeps a dense sheet readable, and every power symbol is the same mechanism: every `GND` symbol on every sheet is one net, and the drawing shows twenty separate little triangles.

So a name is a connection, which has a consequence people find surprising. **Renaming a net can rewire the board.** Give two unrelated nets the same label and they merge. Change one label of a pair and they split.

That makes names checkable, and gives two rules that are mirror images:

{{ agniRun "content/learn/runs/name-conflicts.yaml" }}

Both of those are one net with two claims on its identity. `label-alias-conflict` is two ordinary labels in the same scope; `power-tap-conflict` is two design-wide names, `+3.3V` and `+3V3`, which a human reads as obviously the same rail and a tool must not, because deciding they are the same is exactly the kind of guess that merges two nets nobody meant to merge.

And the mirror:

{{ agniRun "content/learn/runs/dupname.yaml" }}

One name, two nets, not connected. Somebody has two things called `VCC` and believes they are one thing. Whether that is a typo or a deliberate reuse across scopes, it is worth knowing before the board is built.

The third variant is the tap fixture again with a label instead of a dot. A label sitting where the tap meets the through-wire joins them just as firmly as a junction dot does, because naming a point puts it on that net. That is correct KiCad behaviour and it is much easier to delete by accident, which is why `wire-no-junction`'s pass says *which* construct is holding the tap together rather than only that something is.

## Wires that reach nothing (EE2)

The simplest case, and still worth a rule. A wire drawn from a pin out into empty space, ending on no pin, no junction and no label.

Nothing electrical happens. The wire contributes nothing, and whatever it was meant to reach is unconnected. Usually somebody started drawing it and got interrupted, or moved the part it pointed at and left the stub behind (you will do this yourself, and the check is how you find out).

`dangling-endpoint` reports these, and the subject is a coordinate rather than a component, because a wire end that reaches nothing has nothing else to be named after. It shares that with `wire-no-junction` above, and those two are the only rules in the catalog whose findings name a location on a sheet rather than a thing on the board. Both are checks about the drawing rather than about the board, which is where the difference comes from.

## What the reader can and cannot see (EE2)

Everything in this chapter has something in common that is worth naming, because it explains a whole corner of the tool.

**None of it survives into the netlist.** A junction dot, a wire that ends in space, a label sitting mid-span: all of these are facts about the *drawing*. By the time the design is a list of nets and pins, the dot has done its work and disappeared, and the dangling wire has left no trace at all. The netlist cannot tell you a dot was missing, because a missing dot just looks like two nets that were always separate.

So these checks have to happen in the **reader**, at the moment the file is parsed, and the answers are carried forward as diagnostics attached to the design. That is why they sit in their own category, and why they behave differently from the rest of the catalog. `wire-no-junction` reports not-applicable on a netlist format like EDIF rather than passing, because EDIF has no wires in it to examine and a clean result there would mean nothing.

This is also the honest limit of the whole approach. A tool reading a netlist can tell you what the netlist says. Whether the netlist matches what the designer meant is a question about intent, and the only reason any of this chapter's defects is catchable is that the drawing keeps enough evidence of intent for a reader to notice the mismatch.

## What you can now answer

- Why "are these two pins connected" cannot be answered by looking at the picture. *(EE2)*
- Why a junction dot is not decoration, and what happens to the netlist without one. *(EE2)*
- Why renaming a net can rewire a board, and why two spellings of one rail is a finding rather than a tidy-up. *(EE2)*
- Why these particular checks live in the reader and report not-applicable on some formats. *(EE2)*

## The rules this page explains

| Rule | What it catches |
|---|---|
| [`wire-no-junction`](../../reference/rules/wire-no-junction/) | a tap that looks connected and is not |
| [`dangling-endpoint`](../../reference/rules/dangling-endpoint/) | a wire ending on nothing |
| [`label-alias-conflict`](../../reference/rules/label-alias-conflict/) | one net, two rival labels |
| [`power-tap-conflict`](../../reference/rules/power-tap-conflict/) | one net, two rival design-wide names |
| [`duplicate-net-name`](../../reference/rules/duplicate-net-name/) | one name, two unconnected nets |
| [`single-pin-net`](../../reference/rules/single-pin-net/) | a net reaching only one pin, which is often the downstream symptom of the above |

Next: [chapter 3 on capacitors](../03-why-every-chip-needs-capacitors/), which moves from "is it connected" to "why is that part there at all".

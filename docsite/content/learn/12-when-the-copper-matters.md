---
title: "12. When the copper matters"
description: "The last chapter. Everything so far treated a design as a graph; the board is a physical object, and a whole class of defect lives only there."
---

Every chapter so far has treated a design as a **graph**. Parts, nets, what connects to what. That model got us a long way, and it has been quietly deferring one thing since [chapter 3](../03-why-every-chip-needs-capacitors/#the-copper-ee7).

A board is not a graph. It is a physical object with copper on it, and the copper has width, position, and neighbours.

**Prerequisites:** the course to here. This chapter is mostly about why the earlier ones stopped where they did.

**Levels on this page:** [EE7](../levels/#layout-ee7). It links to [what that level means](../levels/).

## The promises this chapter is here to keep (EE7)

Three earlier chapters ended by pointing at this one.

[Chapter 3](../03-why-every-chip-needs-capacitors/#the-copper-ee7) said a decoupling capacitor on the correct net but placed 20 mm from the pin does not decouple it, because the loop it forms with the chip has enough inductance to defeat it at the frequencies it was fitted for. Same netlist, working or not working depending on where it sits.

[Chapter 11](../11-crystals-and-oscillators/) said the stray capacitance of the tracks counts toward a crystal's load, so the same oscillator schematic runs at two frequencies on two layouts.

And [chapter 1](../01-what-a-board-is-made-of/) put termination on its list of jobs, which only means anything because a track is a transmission line, which is a fact about its physical length relative to the signal on it.

None of those is checkable from a netlist. All of them are ordinary.

## What geometry can answer (EE7)

Give the tool the copper and a different class of question opens up:

{{ agniRun "content/learn/runs/copper-geometry.yaml" }}

Three rules, and each has a different shape from anything earlier in the course.

**`copper-clearance` names a pair and a place.** `CLOSE_A + CLOSE_B`, with `worst gap 0.040mm near (10.00, -12.00)mm`. Two nets that should be electrically separate have copper running closer than the process can reliably keep apart, and the verdict carries a coordinate because "these two are too close" is unactionable without knowing where. This is the same pair-shaped subject [chapter 9](../09-sequencing-and-straps/) used for sequencing, for the same reason: the defect is a relation, not a property.

**`track-width` and `annular-width` name a net and a count.** One track segment below `0.127mm`, one via whose ring of copper is below `0.075mm`. Neither is a connectivity question at all. The net is connected; it is connected by metal too thin to be made reliably.

## A fourth kind of authority (EE7)

Look at where those numbers come from, because it is not any of the sources the course has met so far.

`0.127mm` is not a physical law and it is not in a component's datasheet. It is what a **fabricator** can hold. A different process, or more money, buys a smaller number. A cheaper one may not manage this one.

So the course has now seen four different answers to "who says what is correct":

- **Physics**, in chapters 1 through 6. A floating input drifts and a reversed LED does not conduct whatever anyone thinks.
- **A vendor**, in chapter 7. The part's own datasheet, with the trust question that came with it.
- **Your own declaration**, in chapters 8 through 10. Nothing states what a rail was supposed to be until you do.
- **A manufacturing process**, here. What can actually be built, which is a business relationship as much as an engineering fact.

That is worth noticing as a shape rather than a list. A check is only as meaningful as the authority behind its threshold, and knowing which of the four you are looking at tells you what to do when one fires. You argue with a fab about clearance. You do not argue with a floating input.

The `netclass-track-width` and `netclass-via-drill` rules sit at the same tier and answer to a fifth: a **net class**, which is your own declaration about how a particular group of nets should be routed. A power net wants more copper than a signal net because it carries more current, and nothing in the geometry says which is which.

## The netlist did not change (EE7)

Here is the point the whole chapter is for.

{{ agniRun "content/learn/runs/clearance-unchanged.yaml" }}

`CLOSE_A` and `CLOSE_B` are two separate nets, correctly two, and the rules that could object to their connectivity do not. `duplicate-net-name` passes because each name is claimed once. `output-output-conflict` passes because neither has two things driving it. Both would go on passing if the gap were 0.004 mm instead of 0.04.

`single-pin-net` does fail, and it is worth saying why rather than quietly leaving it out of the run: a bare `.kicad_pcb` is a board fixture with one pad per net, so every net on it is a stub. That has nothing to do with the clearance and would fire identically on a board whose copper was a mile apart.

Which is the point. There is no netlist you could inspect, no query you could write, and no amount of care with the schematic that would find the real defect here. The connectivity is right. The board is wrong.

A geometry pass says so with the same discipline:

{{ agniRun "content/learn/runs/clearance-pass.yaml" }}

It measured the closest approach, states that the two nets never came within the floor, and says on how many places it looked. That is the considered set from [chapter 5](../05-who-drives-this-net/#two-nets-opposite-problems-ee3) arriving at the board tier.

That is the honest edge of everything before this chapter, and it is why the tool has a board tier rather than treating the schematic as the design. A netlist is a model, models leave things out, and the things this one leaves out are exactly the ones that get decided after the schematic is signed off.

## What you can now answer

- Why a decoupling capacitor's position changes whether it works, and why chapter 3 could not check it. *(EE7)*
- Why a clearance finding carries a coordinate when almost nothing else in the catalog does. *(EE7)*
- Where a fabrication limit comes from, and how that differs from a datasheet limit. *(EE7)*
- Why two nets can pass every connectivity rule in the catalog and still be a defective board. *(EE7)*

## The rules this page explains

| Rule | Severity | What it catches |
|---|---|---|
| [`copper-clearance`](../../reference/rules/copper-clearance/) | error | copper of two nets closer than the process floor |
| [`track-width`](../../reference/rules/track-width/) | warning | a track segment below the fabrication minimum |
| [`annular-width`](../../reference/rules/annular-width/) | warning | a via whose ring of copper is too thin to drill reliably |
| [`netclass-track-width`](../../reference/rules/netclass-track-width/) | warning | a net routed narrower than its declared class requires |
| [`netclass-via-drill`](../../reference/rules/netclass-via-drill/) | warning | a via drilled smaller than its declared class requires |

## The end of the course

Twelve chapters, and the shape they make is worth stating once.

You started able to name components. You can now look at an unfamiliar board, say what most of its parts are for, predict how each would fail and what the bench would see, check a part against its datasheet and cite the row, read its power tree and say what has to come up first, and know which questions the copper answers that the schematic cannot.

More usefully, you can tell **what a report did not look at**. That has been the through-line since chapter 5's pass on a broken net, through the `not-considered` verdicts, to [chapter 10's](../10-interfaces-and-what-they-require/#the-silence-at-the-end-ee6) satisfied requirement that produced no output at all. A tool that tells you what it checked is worth more than one that tells you it found nothing, and knowing the difference is most of what separates using a checker from trusting one.

Where to go next depends on what you want. The [tutorials](../../tutorials/) take one board from a first read to a checklist gating CI, which is the tool-shaped version of this course. The [rules catalog](../../reference/rules/) now reads as a list of things you understand rather than a glossary. And [building a check rule](../../build/check-rule/) is the other side of it, where the reasoning in these chapters gets written down as something a machine runs.

---
title: "11. Crystals and oscillators"
description: "A crystal does not produce a signal. Two capacitors decide what frequency it runs at, and adding more of them is a defect."
---

A short chapter about one component, chosen because it is the clearest case in the course of a part whose *value* matters as much as its presence, and where doing more of the obviously-good thing makes it worse.

**Prerequisites:** [Chapter 3](../03-why-every-chip-needs-capacitors/) for capacitors, [chapter 7](../07-reading-a-datasheet/) for ratings.

**Levels on this page:** [EE3](../levels/#roles-ee3), [EE5](../levels/#numbers-ee5). Each links to [what that level means](../levels/).

## A crystal is not an oscillator (EE3)

The first surprise is that a quartz crystal does not produce anything. Put one on a bench supply and nothing happens. It is a **mechanical resonator**: a sliver of quartz that flexes when you apply a voltage and generates a voltage when it flexes, and which does both very efficiently at one particular frequency determined by how it was cut.

The oscillator is the chip. Inside is an amplifier, and the crystal is wired across it so that its output feeds back to its input through the crystal. Noise gets amplified, the crystal passes back only the component at its own frequency, that gets amplified again, and within a few milliseconds the whole thing is ringing steadily at the crystal's frequency.

The two capacitors are part of that feedback path. Each terminal gets one, to ground.

{{ includeFile "figures/crystal-load-caps.svg" }}

## What the capacitors decide (EE3)

They are not decoupling. Nothing here is being smoothed.

A crystal's exact frequency depends on the **capacitance it sees** across its terminals. Load it differently and it runs at a slightly different frequency, which is a property of the quartz rather than a flaw. So a crystal's datasheet states a **load capacitance**, and that figure is a condition on the promise: run me with 12 pF and I will give you the frequency on the label.

The two capacitors are how a board supplies it. They sit in series as far as the crystal is concerned, so two 22 pF parts present about 11 pF, plus whatever the pins and the traces contribute.

That last term is why this chapter reaches into [EE7](../levels/#layout-ee7). Stray capacitance from the copper is part of the load, so the same schematic on two different layouts presents two different loads, and the usual practice is to keep those tracks short and away from everything for exactly that reason.

## Presence is checkable, value is not (EE3)

{{ agniRun "content/learn/runs/crystal-caps.yaml" }}

The first board is missing a load capacitor on one terminal. Note the subject: `Y1.2`, a **terminal** rather than the part, because each side needs its own capacitor and having one is not half a solution. The second board has both and passes twice.

What the rule does not check is the **value**, and by now the reason should be familiar. Presence is a fact about the netlist. The right value needs the crystal's specified load from its datasheet, the chip's own pin capacitance from another datasheet, and the stray capacitance of a layout that may not exist yet. That is the same boundary [chapter 3](../03-why-every-chip-needs-capacitors/) drew around decoupling and [chapter 4](../04-pull-ups-and-undefined-states/) drew around pull-ups, and it lands in the same place: the check that can be made from a netlist is made, and the rest is stated rather than pretended.

Getting the value wrong does not stop the clock. It shifts it, by a small percentage, which is enough to matter. A UART running a few percent off will still frame most characters and corrupt some. A CAN controller off frequency fails at the far end of the bus first. These are the bugs that get blamed on cables.

## More is worse (EE5)

The counterintuitive one, and the reason this chapter exists at all.

Some parts are **ceramic resonators with the load capacitors built in**. Cheaper, less accurate, and physically one three-terminal part where a crystal plus two caps is three parts. They present their specified load internally, so they need nothing external.

Fit external caps to one anyway and the load roughly doubles. The oscillator now sees far more capacitance than either part was specified for, so it starts slowly, starts off-frequency, or over temperature and part spread does not start at all.

The rule's own `Impact` describes how that arrives: *"The board usually oscillates on the bench and drifts or drops out in the field, and every timed peripheral clocked from it (UART baud, CAN bit timing) goes with it. It is a silent BOM/layout carry-over from a crystal design."*

That last phrase is the mechanism. Nobody adds redundant capacitors on purpose. Somebody copies an oscillator block from a previous design, swaps the crystal for a cheaper resonator, and leaves the two capacitors in place because they were already there and removing parts feels like the risky edit.

## What you can now answer

- Why a crystal produces nothing on its own, and where the oscillator actually is. *(EE3)*
- What the two capacitors are for, given that nothing is being smoothed. *(EE3)*
- Why the same schematic can run at two frequencies on two layouts. *(EE3)*
- Why a rule checks that the capacitors are present and says nothing about their value. *(EE3)*
- Why adding load capacitors to a part that already has them is a defect rather than margin. *(EE5)*

## The rules this page explains

| Rule | Severity | What it catches |
|---|---|---|
| [`crystal-load-caps`](../../reference/rules/crystal-load-caps/) | warning | a crystal terminal with no load capacitor to ground |
| [`resonator-redundant-load-caps`](../../reference/rules/resonator-redundant-load-caps/) | warning | a resonator with integrated caps carrying external ones too |

The second has no fixture in this course, so it is described from its own catalog entry rather than run. The pair is worth reading together: one fires when a capacitor is missing and the other when one is present, on parts that look nearly identical on a schematic.

Next: [when the copper matters](../12-when-the-copper-matters/), the last chapter, and the level everything so far has deferred to.

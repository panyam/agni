---
title: "7. Reading a datasheet like a type signature"
description: "A datasheet is a contract with two very different kinds of number in it. Which one you are reading changes what a violation means."
---

Everything so far has been answerable from the netlist. Whether a wire connects, what a part is for, which pin drives. This chapter crosses a line, into questions the design cannot answer at all because the answers live in a document it does not contain.

Read [chapter 1](../01-what-a-board-is-made-of/) first, and [chapter 3](../03-why-every-chip-needs-capacitors/#the-numbers-ee5) for a first look at a rating.

This page teaches at [EE5](../levels/#numbers-ee5), and the [levels page](../levels/) says what each level means.

## A part is a contract (EE5)

If you write software, you already have the right model. A datasheet is a **type signature** for a part, stating what the part promises and what it requires of you in return. Feed it what it requires and the promises hold. Go outside, and the vendor makes no claim at all about what happens.

Where the analogy pays off is in what a violation *means*. A type error is a compile-time refusal. A datasheet violation refuses nothing. The board gets built, and the part behaves in some way the vendor never characterised, which may be "fine on this unit today".

## Two numbers that look alike (EE5)

Here is the seeded data for the two regulators on the tutorial board:

{{ agniRun "content/learn/runs/datasheet-rows.yaml" }}

Look at `U1`. It has **two** VIN numbers, and they come from different pages of the same document.

**36 V is the absolute maximum**, from page 3. That is a damage threshold. It says nothing about the part working; it says that beyond this you may destroy it, and that the vendor's other promises were never evaluated up there. It is not a design target and operating at it is not "using the full range".

**32 V is the recommended operating maximum**, from page 4. That is the actual contract. Stay inside it and every other number in the datasheet applies: the efficiency curve, the output accuracy, the thermal figures.

The gap between them is deliberate margin, and treating the bigger number as the usable one is the classic way to build something that works on the bench and fails in the field. Confusing the two is probably the single most common datasheet mistake, and it is why the parameter layer records `limit_kind` on every row rather than storing "the VIN limit".

{{ includeFile "figures/absolute-maximum-rating.svg" }}

The seeded rows also carry **conditions**, `TA = 25C` here, though the query above does not print them. A number is only true under the conditions it was measured at, and a part characterised at 25°C tells you comparatively little about the same part at 85°C in a sealed enclosure.

## A third number, which promises nothing (EE5)

The two numbers above are both promises. An absolute maximum promises damage beyond it; a recommended
operating range promises the rest of the document holds inside it. A datasheet prints a third kind
that promises nothing at all, and it is the one most likely to be mistaken for a fact.

Both regulators on this board state an output voltage:

{{ agniRun "content/learn/runs/typical-values.yaml" }}

`U1` outputs 3.3 V. That is the number the schematic calls `+3V3`, the number on the rail label, the
number you would put in a power budget. It is a **typical** value, which means it is roughly what a
part from the middle of the production run does at room temperature with a modest load. The part on
your bench is a sample from that distribution, and it is within spec anywhere the datasheet's
tolerance allows, which this row does not even state.

So a typical is useful for the things averages are useful for, such as estimating what the board draws,
sizing a heatsink, or sanity-checking a rail label. It is the wrong number to design a threshold
against, because the part that trips your comparator will be the one at the edge of the distribution,
and it was in spec the whole time.

Ask for it the way you would ask for a limit and you can see the layer refusing to answer:

{{ agniRun "content/learn/runs/typical-not-a-limit.yaml" }}

Two rows, and no number in either. `param` reports ceilings, a typical is not one, and the row stays
with its number missing rather than quietly reporting zero. That absence is deliberate, because
a threshold written against a missing number then cannot silently pass, since ordering
refuses to compare an absent value against a present one.

Note the second row's citation while you are here. `U2`'s 1.8 V comes from a placeholder at
confidence 0.3, so it is a typical value that nobody has even transcribed from a real document. Two
different reasons to distrust one number, and the section on provenance below takes up the second.

## The comparison (EE5)

With ratings available, `supply-exceeds-abs-max` can compare them against the rails:

{{ agniRun "content/learn/runs/abs-max-verdicts.yaml" }}

`U1` sits on 12 V against a 36 V maximum and passes. `U2` sits on the 3.3 V rail against a 3 V maximum and fails.

Both verdicts state both numbers, which is the EE5 habit in miniature. "Exceeds its rating" is unactionable. "3.3 V exceeds the absolute maximum of 3 V" can be checked against the document by anyone.

## Where did the number come from? (EE5)

Now the part that separates this layer from a spreadsheet of limits.

Every parameter carries **provenance**: which document, which page, which table, how it got there, and how much anyone should trust it. The query above printed it. `U1`'s rows say *page 3, "Absolute Maximum Ratings" (hand, confidence 1)*. `U2`'s say *page 0, "" (mock, confidence 0.3)*.

`U2`'s rating is a placeholder somebody typed to stand in for a datasheet nobody has transcribed yet. It might be right. Nothing has checked it.

So the same finding reads differently depending on what is asking:

{{ agniRun "content/learn/runs/datasheet-trust.yaml" }}

`agni check` reported that as an `error`. A **review** reports it as `provisional`, because the evidence sits below the trust floor. The engine is declining to call a defect on a number nobody has verified, while still refusing to hide it.

A parameter corpus starts empty and fills up over months, mostly with rows somebody typed in a hurry. A tool that treated every seeded number as gospel would produce confident accusations from placeholder data, and the first time that happens to an engineer they stop believing the tool. A tool that ignored unverified rows would go quiet instead. Provisional is the third answer, which reports the finding and says its evidence is unverified.

## What this layer does not cover (EE5)

Count the verdicts above: **two**, on a board with nineteen parts.

That is not a bug, and the rule states why where it happens. A part with no seeded datasheet is *not a subject*, because there is no stated rating to compare anything against. Only the two regulators have parameter files, so only their supply pins were judged. Every other part on the board went unexamined by this rule, and no output claims otherwise.

Coverage at EE5 is therefore bounded by your parameter corpus rather than by your design, which is a different shape from every earlier chapter. A connectivity rule sees the whole netlist for free. A datasheet rule sees exactly as much as somebody has typed in, and the work of extending it is transcription rather than cleverness.

Remember [chapter 3's](../03-why-every-chip-needs-capacitors/#the-numbers-ee5) closing point here too, because it is the limit beyond this one. A number can be correctly transcribed, correctly compared, and still not be the number in your circuit, because a ceramic capacitor's marked value falls with applied voltage, so a part that satisfies every check on paper can be short of capacitance on the bench.

## What you can now answer

- Why a datasheet has two maximum voltages and what each one licenses. *(EE5)*
- Why a rating is meaningless without its conditions. *(EE5)*
- Why a typical value is not a promise, and what it is still good for. *(EE5)*
- Why the same defect reads as an error to one command and as provisional to another. *(EE5)*
- Why a datasheet rule judged two subjects on a nineteen-part board, and why that is the correct count. *(EE5)*

## The rules this page explains

| Rule | Severity | What it catches |
|---|---|---|
| [`supply-exceeds-abs-max`](../../reference/rules/supply-exceeds-abs-max/) | error | a supply pin above the part's absolute-maximum input |
| [`cap-voltage`](../../reference/rules/cap-voltage/) | error | a capacitor's rated voltage below its rail, with derating |
| [`fet-vdss-below-switched-rail`](../../reference/rules/fet-vdss-below-switched-rail/) | error | a FET on a rail at or above its drain-source breakdown voltage |
| [`regulator-output-exceeds-abs-max`](../../reference/rules/regulator-output-exceeds-abs-max/) | error | a regulator driving a rail above what a part it feeds can survive |
| [`load-switch-trip-above-fet-rating`](../../reference/rules/load-switch-trip-above-fet-rating/) | error | a load switch that trips above its pass FET's continuous rating |

In the next chapter, [the power tree](../08-the-power-tree/), the question stops being about one part and becomes about how the whole board is fed.

---
title: "7. Reading a datasheet like a type signature"
description: "A datasheet is a contract with two very different kinds of number in it. Which one you are reading changes what a violation means."
---

Everything so far has been answerable from the netlist. Whether a wire connects, what a part is for, which pin drives. This chapter crosses a line: the questions here cannot be answered from the design at all, because the answers live in a document the design does not contain.

**Prerequisites:** [Chapter 1](../01-what-a-board-is-made-of/), and [chapter 3](../03-why-every-chip-needs-capacitors/#the-numbers-ee5) for a first look at a rating.

**Levels on this page:** [EE5](../levels/#numbers-ee5). It links to [what that level means](../levels/).

## A part is a contract (EE5)

If you write software, you already have the right model. A datasheet is a **type signature** for a part: it states what the part promises, and what it requires of you in return. Feed it what it requires and the promises hold. Go outside, and the vendor makes no claim at all about what happens.

Where the analogy pays off is in what a violation *means*. A type error is a compile-time refusal. A datasheet violation is nothing so tidy: the board gets built, and the part behaves in some way the vendor never characterised, which may be "fine on this unit today".

## Two numbers that look alike (EE5)

Here is the seeded data for the two regulators on the tutorial board:

{{ agniRun "content/learn/runs/datasheet-rows.yaml" }}

Look at `U1`. It has **two** VIN numbers, and they come from different pages of the same document.

**36 V is the absolute maximum**, from page 3. That is a damage threshold. It says nothing about the part working; it says that beyond this you may destroy it, and that the vendor's other promises were never evaluated up there. It is not a design target and operating at it is not "using the full range".

**32 V is the recommended operating maximum**, from page 4. That is the actual contract. Stay inside it and every other number in the datasheet applies: the efficiency curve, the output accuracy, the thermal figures.

The gap between them is deliberate margin, and treating the bigger number as the usable one is the classic way to build something that works on the bench and fails in the field. Confusing the two is probably the single most common datasheet mistake, and it is why the parameter layer records `limit_kind` on every row rather than storing "the VIN limit".

There is a third thing on each row worth noticing: **conditions**. The seeded rows here carry `TA = 25C`. A number is only true under the conditions it was measured at, and a part characterised at 25°C tells you comparatively little about the same part at 85°C in a sealed enclosure.

## The comparison (EE5)

With ratings available, the rule can do what it could not before:

{{ agniRun "content/learn/runs/abs-max-verdicts.yaml" }}

`U1` sits on 12 V against a 36 V maximum and passes. `U2` sits on the 3.3 V rail against a 3 V maximum and fails.

Both verdicts state both numbers, which is the EE5 habit in miniature. "Exceeds its rating" is unactionable. "3.3 V exceeds the absolute maximum of 3 V" can be checked against the document by anyone.

## Where did the number come from? (EE5)

Now the part that separates this layer from a spreadsheet of limits.

Every parameter carries **provenance**: which document, which page, which table, how it got there, and how much anyone should trust it. The query above printed it. `U1`'s rows say *page 3, "Absolute Maximum Ratings" (hand, confidence 1)*. `U2`'s say *page 0, "" (mock, confidence 0.3)*.

That difference is not cosmetic. `U2`'s rating is a placeholder somebody typed to stand in for a datasheet nobody has transcribed yet. It might be right. Nothing has checked it.

So the same finding reads differently depending on what is asking:

{{ agniRun "content/learn/runs/datasheet-trust.yaml" }}

`agni check` reported that as an `error`. A **review** reports it as `provisional`, because the evidence sits below the trust floor. The engine is declining to call a defect on a number nobody has verified, while still refusing to hide it.

This matters more than it first appears. A parameter corpus starts empty and fills up over months, mostly with rows somebody typed in a hurry. A tool that treated every seeded number as gospel would produce confident accusations from placeholder data, and the first time that happens to an engineer they stop believing the tool. A tool that ignored unverified rows would go quiet instead. Provisional is the third answer, and it is the honest one.

## What this layer does not cover (EE5)

Count the verdicts above: **two**, on a board with nineteen parts.

That is not a bug and the rule says so where it happens: a part with no seeded datasheet is *not a subject*, because there is no stated rating to compare anything against. Only the two regulators have parameter files, so only their supply pins were judged. Every other part on the board went unexamined by this rule, and no output claims otherwise.

Coverage at EE5 is therefore bounded by your parameter corpus rather than by your design, which is a different shape from every earlier chapter. A connectivity rule sees the whole netlist for free. A datasheet rule sees exactly as much as somebody has typed in, and the work of extending it is transcription rather than cleverness.

Worth remembering [chapter 3's](../03-why-every-chip-needs-capacitors/#the-numbers-ee5) closing point here too, because it is the limit beyond this one. A number can be correctly transcribed, correctly compared, and still not be the number in your circuit: a ceramic capacitor's marked value falls with applied voltage, so a part that satisfies every check on paper can be short of capacitance on the bench.

## What you can now answer

- Why a datasheet has two maximum voltages and what each one licenses. *(EE5)*
- Why a rating is meaningless without its conditions. *(EE5)*
- Why the same defect reads as an error to one command and as provisional to another. *(EE5)*
- Why a datasheet rule judged two subjects on a nineteen-part board, and why that is honest. *(EE5)*

## The rules this page explains

| Rule | Severity | What it catches |
|---|---|---|
| [`supply-exceeds-abs-max`](../../reference/rules/supply-exceeds-abs-max/) | error | a supply pin above the part's absolute-maximum input |
| [`cap-voltage`](../../reference/rules/cap-voltage/) | error | a capacitor's rated voltage below its rail, with derating |
| [`fet-vdss-below-switched-rail`](../../reference/rules/fet-vdss-below-switched-rail/) | error | a FET on a rail at or above its drain-source breakdown voltage |
| [`regulator-output-exceeds-abs-max`](../../reference/rules/regulator-output-exceeds-abs-max/) | error | a regulator driving a rail above what a part it feeds can survive |
| [`load-switch-trip-above-fet-rating`](../../reference/rules/load-switch-trip-above-fet-rating/) | error | a load switch that trips above its pass FET's continuous rating |

Next: the power tree, where the question stops being about one part and becomes about how the whole board is fed.

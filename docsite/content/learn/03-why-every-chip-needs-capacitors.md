---
title: "3. Why every chip needs capacitors"
description: "Decoupling and bulk, explained at four levels, with the rules that check each one."
---

Open any board and you will find capacitors scattered around every chip, usually more of them than anything else. This page is about why, and it is the best single example of the [level ladder](../), because the same capacitor is a different question at four different levels and the tool checks it at a different tier each time.

**Prerequisites:** [EE1](../levels/#parts-ee1) (you know a capacitor stores charge) and [EE2](../levels/#nets-ee2) (you can read a schematic as a graph of nets).

**Levels on this page:** [EE3](../levels/#roles-ee3), [EE4](../levels/#failure-modes-ee4), [EE5](../levels/#numbers-ee5), [EE7](../levels/#layout-ee7). Each links to [what that level means](../levels/).

## The role (EE3)

> *What job is this part doing?*

A chip's supply pin needs a **local charge reservoir**.

Everything else on this page follows from that sentence, and the load-bearing word is *local*. The obvious mental model is that the voltage regulator supplies the chip, so as long as the wire gets there, the chip is powered. That model is fine for a torch and wrong for a digital chip, and the reason is time.

When a chip's output drivers switch, they all flip within a nanosecond or two of each other, and at that instant the chip demands a gulp of current it did not need a moment earlier. The current has to come down the trace from the regulator. But a trace has **inductance**, roughly 1nH per millimetre, and inductance is the property that resists a *change* in current:

```
V = L · di/dt
```

Put numbers on it. A 50mm trace is about 50nH. Suppose the chip's demand rises by 100mA over 10ns, so `di/dt` is 10⁷ amps per second. Then `V = 50×10⁻⁹ × 10⁷ = 0.5V`. Half a volt is dropped across a piece of copper, at exactly the moment the chip is busiest. On a 3.3V part that is a 15% sag on the supply, appearing and vanishing in nanoseconds.

A capacitor sitting at the supply pin already holds charge and has almost no inductance between it and the pin. It supplies that gulp locally and recharges slowly from the regulator afterwards. The regulator handles the average, the capacitor handles the transient.

Two jobs, two rules:

- A **decoupling capacitor** sits at each supply pin and handles the fast transients. → [`decoupling-present`](../../reference/rules/decoupling-present/)
- A **bulk capacitor** is larger, one per rail rather than one per pin, and handles slower swings such as a whole subsystem waking up. → [`bulk-cap`](../../reference/rules/bulk-cap/)

Run the first of those on the tutorial board and read what it concluded about each rail:

{{ agniRun "content/learn/runs/decoupling-verdicts.yaml" }}

The passes are the interesting part. They name the capacitor, so you can open the schematic and confirm that `C1` really is on `PMIC_CORE_3V3`. That is the difference between a check that ran and a check that found nothing.

`bulk-cap` is deliberately absent from that run. It scopes on rails the design NAMES, using a net attribute that only the schematic-geometry readers set, so on this EDIF board it has no subject at all and reports nothing. Printing that beside the decoupling verdicts would read as "the bulk capacitors are fine", which is the confusion this whole tool exists to remove. It is filed as agni issue 426.

## The failure mode (EE4)

> *How does this break, and what does the bench see?*

Here is the finding on its own:

{{ agniRun "content/learn/runs/decoupling-finding.yaml" }}

Note the severity: **warning**, not error. That choice carries the EE4 content of this rule and is worth sitting with, because the naive reading says a missing decoupling cap is obviously serious and should be an error.

It is serious. It stays a warning because **the board works**, and that is what makes it expensive.

A board with no decoupling on a rail powers up, runs, and passes bring-up. It fails later and intermittently: when many outputs happen to switch at once, when the enclosure is warm, on three units out of ten and not the other seven. What the bench sees is a spontaneous reset, a corrupted register read, an ADC that is noisier than it should be, a link that drops once an hour. None of those symptoms points at a capacitor. Engineers lose weeks to this class of fault, and they lose them at the end of a project rather than the beginning.

The rule's own `Impact` field says it in one line: *"Rarely visible at first power-on, expensive to find later."*

This is the general shape of EE4 and it is why severity in this catalog is a **policy** signal rather than a confidence signal. An `error` is something that will not work at all, like an I2C bus with no pull-up, which cannot signal even once. A `warning` is something that usually indicates a defect but where the board still runs. Missing decoupling is the archetype of the second: maximum eventual cost, minimum immediate symptom.

## The numbers (EE5)

> *Is this within spec, and which datasheet row says so?*

At EE3 the answer was "a capacitor". At EE5 you have to say *which* capacitor, and now two numbers matter.

**Capacitance.** The vendor states what the part needs, typically 100nF at each supply pin plus a shared bulk capacitor of a few µF to tens of µF. This is in the datasheet, not deducible from the schematic, which is why it lives in the parameter layer rather than in a netlist rule.

**Voltage rating.** A capacitor has a maximum working voltage, and putting it on a rail above that rating is a failure with a mechanism rather than a guideline: ceramic capacitors fail short, so an overstressed decoupling cap becomes a dead short from the rail to ground. [`cap-voltage`](../../reference/rules/cap-voltage/) checks it, with a derating factor, because running a part at exactly its rating leaves nothing for tolerance or transients:

{{ agniRun "content/learn/runs/cap-voltage.yaml" }}

Read the finding closely, because its structure is the EE5 skill in miniature. It states the observed value (6.3V rated), the required value (12.5V), how the requirement was computed (`rail 10V × derate 1.25`), and **the datasheet page and table it came from**. An EE5 claim you cannot trace to a row in a document is an opinion.

There is a third number that nothing in this catalog checks yet, and it is the one that catches people. **A ceramic capacitor's capacitance falls as you apply DC voltage to it.** Class II dielectrics (X5R, X7R) can lose more than half their marked value at their rated voltage. A 100nF 6.3V X7R on a 5V rail may deliver under 50nF in circuit, so a design that looked correct on the schematic is short of decoupling on the bench. Class I dielectrics (C0G/NP0) do not do this, and cost more for the same capacitance. The usual working habit is to specify a voltage rating well above the rail, often 2× or more, for capacitance stability rather than for breakdown margin.

## The copper (EE7)

> *Why must that capacitor be at the pin?*

Everything above is netlist-level: the capacitor is on the right net, with the right value and rating. A board can satisfy all of it and still have no working decoupling.

The reason is the same inductance from EE3, now applied to the capacitor's own path. A decoupling cap does its job through a loop: out of the cap, into the supply pin, through the chip, out of the ground pin, back to the cap. That loop has inductance proportional to its physical area. Place the cap 20mm away, or route its ground back through a long trace instead of straight down into a ground plane, and the loop inductance swamps the capacitor at exactly the frequencies it was there for.

The rule says so itself, in its remedy: *"Add a decoupling capacitor from the rail to ground at each supply pin, and place it at the pin in layout. A capacitor drawn on the rail but placed across the board does not decouple it."*

This is a genuine limit of a netlist check rather than a gap someone forgot to fill, and the rule's scope note is explicit that placement is a board-tier notion. It is also why the tool has a board tier at all. Reading a netlist can prove a capacitor is *present*; only geometry can say whether it is *close*.

## What you can now answer

- Why a chip needs a capacitor a few millimetres away when a regulator is already supplying it. *(EE3)*
- Why missing decoupling is a warning rather than an error, and what it looks like on the bench. *(EE4)*
- What a voltage rating means, why a derate factor exists, and why "100nF" on a schematic may not be 100nF in the circuit. *(EE5)*
- Why the netlist check stops where it does, and what a board-tier check would have to add. *(EE7)*

## The rules this page explains

| Rule | Level | What it checks |
|---|---|---|
| [`decoupling-present`](../../reference/rules/decoupling-present/) | EE3 | a rail feeding supply pins carries at least one capacitor |
| [`bulk-cap`](../../reference/rules/bulk-cap/) | EE3 | a named rail carries any capacitor at all |
| [`cap-voltage`](../../reference/rules/cap-voltage/) | EE5 | a capacitor's rated voltage clears its rail, with derating, cited to a datasheet |

Next: [pull-ups and undefined states](../04-pull-ups-and-undefined-states/), which runs the same shape of argument over a resistor.

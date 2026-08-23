---
title: "8. The power tree"
description: "A board is fed by a cascade, not a supply. The first level where correctness comes from a declaration rather than from physics."
---

Every chapter so far asked about one thing: this capacitor, this net, this pin, this part's rating. This one asks about the board.

**Prerequisites:** [Chapter 3](../03-why-every-chip-needs-capacitors/) for rails, [chapter 7](../07-reading-a-datasheet/) for ratings.

**Levels on this page:** [EE6](../levels/#systems-ee6). It links to [what that level means](../levels/).

## A board is fed by a cascade (EE6)

Power arrives at a connector as one voltage and has to become several, because the parts on a board do not agree about what they want. A processor core might run at 1.8 V, its I/O at 3.3 V, and a motor driver straight off a 12 V input.

So a board carries a chain of regulators, and the shape of that chain is the **power tree**. (Ask an engineer to sketch a board they own and this is usually the first thing they draw.) Read the tutorial board's off its regulators:

{{ agniRun "content/learn/runs/power-tree.yaml" }}

`U1` touches `PMIC_MAIN_12V0` and `PMIC_CORE_3V3`. `U2` touches `PMIC_CORE_3V3` and `PMIC_IO_1V8`. So 12 V feeds U1, which makes 3.3 V, which feeds U2, which makes 1.8 V. A cascade, each stage taking the previous one's output as its input.

Cascading rather than running three regulators from the input is a deliberate trade. A switching regulator is efficient over a big step, which is what you want taking 12 V down, and it is electrically noisy. A linear regulator wastes the voltage it drops as heat, which is fine over a small step, and it is quiet. So the usual shape is a switcher doing the heavy lifting and a linear part cleaning up a rail that feeds something sensitive.

Two other nets in that output are worth naming, because [chapter 1](../01-what-a-board-is-made-of/) met them already. `PMIC_EN` is the enable that R2 pulls high, and `PMIC_PG` is the power-good output that R3 feeds to the processor's reset. Neither carries power. They are how the tree gets turned on and how the rest of the board learns that it came up, which is [chapter 9](../).

## Nothing here is a fact about the world (EE6)

Now the shift, and it is the reason this level feels different.

Everything checkable so far was checkable against something outside the design. Whether a wire connects is a fact about the netlist. Whether a capacitor's rating clears its rail is a fact in a datasheet. The tool did not need to be told what "correct" meant, because physics and the vendor had already said.

A power tree has no such external referent. **Nothing anywhere states that `PMIC_CORE_3V3` was supposed to be 3.3 V.** The name says so, and a name is a convention somebody followed, not a measurement. A board where that rail is actually 5 V is perfectly self-consistent; it is just not the board anyone meant to build.

So at this level the tool has to be *told* what was intended, in a declaration that sits beside the design:

```yaml
voltage_domains:
  - {name: main, nominal: 12.0, rails: [PMIC_MAIN_12V0]}
  - {name: io,   nominal: 3.3,  rails: [PMIC_CORE_3V3]}
  - {name: core, nominal: 3.3,  rails: [PMIC_IO_1V8]}
```

and then it can check the board against it:

{{ agniRun "content/learn/runs/voltage-domains.yaml" }}

The third declaration is wrong on purpose. It puts `PMIC_IO_1V8`, whose name declares 1.8 V, in a domain declared at 3.3 V. The rule reports the disagreement without deciding which side is mistaken, because it cannot: either somebody declared the domain wrongly, or the rail is named wrongly, or the rail really is at the wrong voltage. All three are worth knowing about and they are the same finding until a human looks.

This is what an intent file buys and what it costs. It buys checks that no amount of cleverness could derive from the netlist. It costs you writing down what you meant and keeping that true as the design moves, which nobody enjoys and which is why the layer stays optional.

## Enough current to go round (EE6)

The other system-level question. A rail has to supply everything hanging off it, and once again the netlist is silent: nothing in a connectivity graph says how much a part draws.

That is two separate questions, and the catalog keeps them as two rules:

{{ agniRun "content/learn/runs/rail-budgets.yaml" }}

**Capacity** asks whether the supply meets the declared peak at all. `3V3` is fed by a part rated 0.5 A against a declared 0.8 A, so it fails outright.

**Margin** asks whether it meets the peak with headroom. `1V8`'s supply is rated 0.9 A, which clears the 0.8 A peak, and misses the 0.96 A that a 1.2× margin factor asks for. Designing a rail to exactly its worst-case draw leaves nothing for tolerance, temperature, or the load somebody adds next revision.

Look at what the margin rule says about `3V3`, though: **not-considered**, with the reason *"the supply is rated below the 0.8A peak itself, which rail-current-capacity reports rather than this rule"*.

That is one defect being reported once. A rail that cannot meet its peak also cannot meet its peak with margin, so a naive pair of rules would report the same underlying problem twice, at two severities, and a reviewer would spend time working out whether they were looking at one problem or two. The margin rule declines instead, and says which rule owns the answer.

## What you can now answer

- Why a board has a chain of regulators rather than one supply, and why the chain usually starts with a switcher. *(EE6)*
- Why nothing in a netlist can tell you a rail is at the wrong voltage. *(EE6)*
- What a voltage-domain finding does *not* tell you, and why it cannot. *(EE6)*
- Why capacity and margin are two questions, and what a not-considered verdict is doing between them. *(EE6)*

## The rules this page explains

| Rule | Severity | What it checks against |
|---|---|---|
| [`intent/voltage-domain-mismatch`](../../reference/rules/intent-voltage-domain-mismatch/) | warning | a declared domain's nominal voltage against its rails |
| [`intent/rail-current-capacity`](../../reference/rules/intent-rail-current-capacity/) | error | a rail's supply against the declared peak draw |
| [`intent/rail-current-margin`](../../reference/rules/intent-rail-current-margin/) | warning | the same, with a declared margin factor |
| [`rail-not-classified`](../../reference/rules/rail-not-classified/) | info | a rail the naming vocabulary could not classify at all |

Every one of these needs something declared. That is the defining property of this level, and it is why a run against a design with no intent file reports these as needing design intent rather than as passing.

Next: sequencing and straps, the other half of the system question. Not what the rails are, but what order they come up in and what the parts read at the moment they do.

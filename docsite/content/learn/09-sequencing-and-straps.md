---
title: "9. Sequencing and straps"
description: "Rails have to come up in an order, and parts read their configuration at the instant they do. Both are declared, and both are invisible in a schematic review."
---

[Chapter 8](../08-the-power-tree/) was about what the rails are. This one is about **when** they arrive and **what the parts read at the moment they do**, which are two different questions that both turn out to be invisible on a schematic.

**Prerequisites:** [Chapter 8](../08-the-power-tree/), particularly the part about declarations.

**Levels on this page:** [EE6](../levels/#systems-ee6). It links to [what that level means](../levels/).

## Order matters, and the datasheet says so (EE6)

A chip with more than one supply usually cares which arrives first. It is not a preference. Bring an I/O rail up while the core is still at zero and current finds its way in through the chip's internal protection structures, which are diodes that were never meant to carry it. At best the part draws current nobody budgeted for and fails to start. At worst it latches up, which is a self-sustaining short inside the die that persists until power is removed and sometimes destroys the part.

So a datasheet states a required order, and a board has to enforce it. The usual mechanism is a chain: each regulator has a **power-good** output that goes true once its rail is up, and the next regulator has an **enable** input. Wire one to the other and the second rail physically cannot come up before the first.

You have already seen the handles. [Chapter 8](../08-the-power-tree/) met `PMIC_EN` and `PMIC_PG` on the tutorial board and set them aside as the nets that carry no power.

## Declared, then checked (EE6)

Like everything at this level, the intended order has to be written down before anything can check it:

```yaml
sequences:
  - name: SoC power tree
    relation: enable-gated
    order:
      - {rail: VDD_CORE, good: CORE_PG}
      - {rail: VDD_IO, enable: IO_EN}
```

{{ agniRun "content/learn/runs/sequencing.yaml" }}

The pass is worth reading in full, because it states the mechanism rather than the conclusion: *"CORE_PG (the power-good of VDD_CORE) reaches IO_EN (the enable of VDD_IO), so VDD_IO is held off until VDD_CORE is good."* Somebody can confirm that on the schematic without knowing what the rule does.

The subject is a **pair**, `CORE_PG + IO_EN`. A sequencing requirement is a relation between two things rather than a property of one, so the verdict names both rather than picking one and mentioning the other in prose.

Now look at the modem item, which fails: *"both handles are on the design and nothing connects MODEM_PG to MODEM_EN."*

Read that carefully, because it is not quite an accusation. Both handles exist and both land on the MCU, which means that sequence is enforced in **firmware** rather than in copper. The netlist genuinely does not connect them, so the finding is true. Whether it is a *defect* depends on something no netlist contains: whether the software is trusted to do it, and whether it does it before the rails are enabled. The rule reports what it can see and stops.

## The same board, declared differently (EE6)

Here is the sharpest version of [chapter 8's](../08-the-power-tree/#nothing-here-is-a-fact-about-the-world-ee6) point. Same design file, same wiring, the order declared the other way round:

{{ agniRun "content/learn/runs/sequencing-reversed.yaml" }}

*"CORE_PG reaches IO_EN, which is the declared order inverted."*

Nothing about the board changed. The chain that passed a moment ago now fails, and the rule can tell the difference between a chain that is missing and a chain that runs backwards. At this level a design is not correct or incorrect on its own. It is correct *with respect to* something somebody declared, and the declaration is as capable of being wrong as the board is.

## What a part reads on the way up (EE6)

The second half, and it happens in the same instant.

Many chips have pins that are read **once**, at the moment reset releases, to configure something that cannot be changed afterwards: which address the part answers to on a shared bus, which source it boots from, how wide its memory interface is. These are **straps**, and they are set by a resistor tying the pin high or low.

Two things make them a distinctive class of bug. The value is latched at power-up and never re-read, so nothing observable later says what was read. And a strap is just a resistor to a rail or to ground, so nothing about it looks different from the pull-ups in [chapter 4](../04-pull-ups-and-undefined-states/).

The number a group of straps encodes is spread across several pins, and the interesting failure is across two parts rather than within one:

{{ agniRun "content/learn/runs/straps.yaml" }}

Two PHYs on one MDIO bus, and in the first declaration both strap to address 1. Each is individually correct. Every connectivity check passes, every resistor is present and correctly valued, and the two parts fight over one address the moment either is spoken to. The bus goes unreliable in a way that reads as noise or marginal timing rather than as a wiring fault.

That is the case that most needs a rule rather than a reviewer, because it is invisible on any single schematic page. Nothing on the sheet showing the first PHY says anything about the second.

The second run is the same `.edn` again, declared for a part whose address pins are numbered the other way round, which is a real thing datasheets differ on. The declared net *order* is what states which pin is the high bit, so reading the same three resistors MSB-first instead of LSB-first gives 4 rather than 1, and the two devices no longer clash.

Both verdicts name a **pair** of devices for the same reason the sequencing one did. "Do these two collide" is a question about two things.

## What you can now answer

- Why a chip cares which of its rails arrives first, and what latch-up is. *(EE6)*
- How a board enforces an order in hardware, and what the two handles are called. *(EE6)*
- Why the same board can pass one declaration and fail another, and what that says about correctness at this level. *(EE6)*
- Why an address collision is invisible in a schematic review. *(EE6)*
- Why "nothing connects these" is a true statement that may not be a defect. *(EE6)*

## The rules this page explains

| Rule | Severity | What it checks against |
|---|---|---|
| [`intent/sequence-*`](../../reference/rules/) | warning | a declared power-up order against the enable chain in copper |
| [`intent/strap-address-collision`](../../reference/rules/intent-strap-address-collision/) | error | two devices on one declared bus strapping to the same address |
| [`intent/property-strap`](../../reference/rules/intent-property-strap/) | warning | a single strap pin against the level the declaration states |

The sequence rules are named after the sequence you declare, so a design with a "SoC power tree" sequence compiles a rule called `intent/sequence-soc-power-tree`. There is no fixed catalog entry to link, which is why that row points at the catalog root.

Next: [interfaces and what they require](../10-interfaces-and-what-they-require/), the last of the three system chapters.

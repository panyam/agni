---
title: "7. Your architecture"
description: "Declare what the board is supposed to be, and detect when it stops being that."
---

The three tiers so far describe your team. Naming, buses, and parts are the same across every board
you build. This one is different: it describes one board, and it is the only tier that can catch a
design drifting from what it was meant to be.

The engine has no built-in opinion about your architecture. It cannot know that a rail was supposed
to be 1.8 V, because a netlist records what is connected and never records what anyone intended. So
you declare it, and the declaration becomes checkable.

The file lives beside the design, not at the project root.

## The declaration

`designs/gateway/intent.yaml`:

```yaml
name: gateway intent
modules:
  - {name: regulators, class: regulator, count: 2}
  - {name: connectors, class: connector, count: 1}
voltage_domains:
  - {name: main, nominal: 12.0, rails: [PMIC_MAIN_12V0]}
  - {name: io, nominal: 3.3, rails: [PMIC_CORE_3V3]}
  - {name: core, nominal: 3.3, rails: [PMIC_IO_1V8]}
subsystems:
  - {name: power tree, nets: [PMIC_MAIN_12V0, PMIC_CORE_3V3, PMIC_IO_1V8]}
  - {name: can, nets: [CAN1_CANH, CAN1_CANL, CAN1_TXD, CAN1_RXD]}
```

Read it as the sentence you would say describing the board to a colleague. Two regulators and one
connector. Three voltage domains at these voltages. A power tree and a CAN subsystem made of these
nets.

## Running it

{{ agniRun "content/tutorials/runs/07-check-intent-params.yaml" }}

The declaration says the core domain runs at 3.3 V. The rail assigned to it is a 1.8 V rail. Nothing
structural is wrong with the board, and no rule from any other tier has anything to say. The only
reason this is catchable is that somebody wrote down what was intended and the two disagree.

That is the whole value of the tier. It does not find defects in the usual sense. It finds
divergence between the board and the description of the board, and that divergence creeps in over
months as a design is edited by people who did not write the original plan.

## A tier can depend on another tier

Run the same thing without `--params`:

{{ agniRun "content/tutorials/runs/07-check-intent.yaml" }}

Two extra findings, and both are false. The board plainly has two regulators.

The declaration says `class: regulator`. Without a datasheet corpus, the classifier can tell U1 and
U2 are integrated circuits from their reference designators, but not what kind. "This is a
regulator" comes off the part's datasheet. Attach `--params` and the class resolves, and both
findings disappear.

Worth internalizing, because it generalizes: a module declaration written in terms of device class
is only as good as the parameter tier underneath it. If you plan to declare modules by class, seed
those parts first. Otherwise the intent tier reports absences that are really gaps in a different
tier.

## Without the declaration

```
| A1 | each rail sits at its declared voltage | needs-design-intent | needs a design-intent declaration (--intent-path) |
| A2 | the declared modules are all present | needs-design-intent | needs a design-intent declaration (--intent-path) |
```

`needs-design-intent`, not `pass`. A question about intent cannot be answered by a design that never
stated its intent, and reporting that honestly is the difference between a checklist you can trust
and one you cannot.

## All four tiers

That is the last of them. Running the full checklist with everything attached:

```
make review
```

```
**3 pass, 8 fail, 1 n/a, 2 not-automated, 1 provisional (of 15)**
```

Which raises the question the next rungs answer: what is that checklist, and how should those
numbers be read?

## Next

Rung 8, writing your checklist, is being written. Until it lands, `examples/tutorial-project/review.yaml`
is a worked example with all four binding kinds in it, and [Checks and reports](../../guide/checks-and-reports/)
covers the underlying report.

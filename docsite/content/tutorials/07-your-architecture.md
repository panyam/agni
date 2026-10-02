---
title: "7. Your architecture"
description: "Declare what the board is supposed to be, and detect when it stops being that."
---

The three tiers so far describe your team. Naming, buses, and parts are the same across every board
you build. This one describes one board, and it is the only tier that can catch a
design drifting from what it was meant to be.

The engine has no built-in opinion about your architecture. It cannot know that a rail was supposed
to be 1.8 V, because a netlist records what is connected and never records what anyone intended. So
you declare it, and the declaration becomes checkable.

The declaration lives in the design's own descriptor, under `intent:`, rather than at the project
root.

## The declaration

The `intent:` section of `designs/gateway/design.yaml`:

```yaml
intent:
  modules:
    - {name: regulators, class: regulator, count: 2}
    - {name: connectors, class: connector, count: 1}
    - {name: power tree, nets: [PMIC_MAIN_12V0, PMIC_CORE_3V3, PMIC_IO_1V8]}
    - {name: can, nets: [CAN1_CANH, CAN1_CANL, CAN1_TXD, CAN1_RXD]}
  nets:
    PMIC_MAIN_12V0: {nominal: 12.0, domain: main}
    PMIC_CORE_3V3: {nominal: 3.3, domain: io}
    PMIC_IO_1V8: {nominal: 3.3, domain: core}
```

Read it as the sentence you would say describing the board to a colleague. Two regulators and one
connector. A {{ explainable "power-tree" }} and a CAN block made of these nets. Three rails at these
voltages, each in a named domain.

## Running it

{{ agniRun "content/tutorials/runs/07-check-intent-params.yaml" }}

The declaration says the core domain runs at 3.3 V. The rail assigned to it is a 1.8 V rail. Nothing
structural is wrong with the board, and no rule from any other tier has anything to say. The only
reason this is catchable is that somebody wrote down what was intended and the two disagree.

This tier finds divergence between the board and the description of the
board rather than defects in the usual sense, and that divergence creeps in over
months as a design is edited by people who did not write the original plan.

## A tier can depend on another tier

Run the same thing without `--params`:

{{ agniRun "content/tutorials/runs/07-check-intent.yaml" }}

Two extra findings, and both are false. The board plainly has two regulators.

The declaration says `class: regulator`. Without a datasheet corpus, the classifier can tell U1 and
U2 are integrated circuits from their {{ explainable "reference-designator" "reference designators" }}, but not what kind. "This is a
regulator" comes off the part's datasheet. Attach `--params` and the class resolves, and both
findings disappear.

A module declaration written in terms of device class
is only as good as the parameter tier underneath it. If you plan to declare modules by class, seed
those parts first. Otherwise the intent tier reports absences that are really gaps in a different
tier.

## Without the declaration

```
| A1 | each rail sits at its declared voltage | needs-design-intent | needs a design-intent declaration (--intent-path) |
| A2 | the declared modules are all present | needs-design-intent | needs a design-intent declaration (--intent-path) |
```

`needs-design-intent`, not `pass`. A question about intent cannot be answered by a design that never
stated its intent, and a checklist that reported `pass` here instead
could not be trusted.

## All four tiers

That is the last of them. Running the full checklist with everything attached:

```
make review
```

```
**3 pass, 8 fail, 1 n/a, 2 not-automated, 1 provisional (of 15)**
```

The next rungs answer what that checklist is and how to read those numbers.

## Next

[Write your checklist](../08-write-your-checklist/), which binds the team's review questions to the
engine so the mechanical ones answer themselves.

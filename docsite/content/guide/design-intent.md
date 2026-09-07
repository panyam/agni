---
title: "Design intent"
description: "Declare what a board is supposed to contain, and have every revision checked against the architecture you agreed rather than against itself."
---

A {{ explainable "netlist" }} says what *is* wired. It cannot say what was *meant*. "The board has two
regulators" is a fact you can read off the file; "the board should have two regulators" is a decision
somebody made in a meeting, and nothing in the export records it.

A design-intent declaration writes that decision down as YAML beside the design, and `check` compares
the board against it. It is the tier that catches a rail quietly moved to the wrong domain, a module
dropped during a cost reduction, or a power-up order that got reversed when the {{ explainable "power-tree" "power tree" }}
was redrawn. None of those look wrong in the schematic. They look wrong against what you said you
were building.

## Write a declaration

```yaml
# designs/gateway/intent.yaml
name: gateway intent
modules:
  - {name: regulators, class: regulator, count: 2}
  - {name: connectors, class: connector, count: 1}
voltage_domains:
  - {name: main, nominal: 12.0, rails: [PMIC_MAIN_12V0]}
  - {name: io,   nominal: 3.3,  rails: [PMIC_CORE_3V3]}
  - {name: core, nominal: 3.3,  rails: [PMIC_IO_1V8]}
subsystems:
  - {name: power tree, nets: [PMIC_MAIN_12V0, PMIC_CORE_3V3, PMIC_IO_1V8]}
  - {name: can, nets: [CAN1_CANH, CAN1_CANL, CAN1_TXD, CAN1_RXD]}
```

That last voltage domain is wrong on purpose, and it is the whole point of the file. `core` is
declared at 3.3 V while the rail it names is `PMIC_IO_1V8`, an actual 1.8 V rail. Nothing about the
schematic is malformed. The board simply stopped matching the architecture, and only a declaration
can notice.

## Where the file goes

Beside the design, named `intent.yaml`. A design that belongs to a project needs no flag, because the
descriptor defaults that name and the project finds it:

{{ agniRun "content/guide/runs/intent-domain-mismatch.yaml" }}

Intent is per-**design**, unlike naming conventions, interface profiles and seeded parameters, which
are per-project. Each board has its own intended architecture, so the file lives with the board
rather than with the team.

`--intent-path` exists for a design that belongs to no project. Reaching for it on a design whose
project already declares intent is an error rather than a silent double-load, so you find out
immediately instead of reading every finding twice.

## The eight forms

Each form answers a question the netlist cannot, and each compiles to its own rule so a reviewer
signing them off separately gets separate verdicts.

| Form | Declares | Fails when |
|---|---|---|
| `modules` | the functional blocks the board must contain, by class or MPN, optionally with a count | a declared module is absent, or the count is short |
| `voltage_domains` | named rails pinned to a nominal voltage | a declared rail is missing, or sits on the wrong domain |
| `subsystems` | a named architectural block and the nets it must instantiate | its source component is absent, or any declared net is missing |
| `protections` | a rail that must carry a protection device, by kind | the declared rail carries no device of that kind |
| `net_properties` | what a net *is*, rather than that it exists (a reset is active-low) | the design's structure contradicts the declaration |
| `rail_budgets` | the peak current a rail draws, with an optional `margin_factor` | the supply reaching it is rated below the peak, or below the margin |
| `sequences` | the power-up order of groups of rails | the gating chain is absent, or runs the other way round |
| `strap_groups` | several strap nets read together as one binary number, and the value it encodes | the group does not encode the declared value, or two devices collide |

`rail_budgets` is the one that joins two tiers. The declaration supplies the demand, which no design
artifact carries, and a seeded {{ explainable "absolute-maximum-rating" "datasheet parameter" }}
supplies the regulator's capacity. Both halves have to be present or the rule stays quiet rather than
guessing.

## With no declaration, nothing passes

There is no built-in intent, and that absence is deliberate. A generic statement of what a board
should contain says nothing, and a rule that enumerated its expectations *from the design* would
always agree with the design. So every intent rule iterates the declaration and probes the netlist,
never the reverse.

The consequence is worth knowing before you read a report. A design run with no declaration leaves
its intent-bound review items reading **needs-design-intent**, never **pass**. The mechanism exists
and is blocked on an input you have not supplied yet, which is a different thing from a board that
was checked and found clean. [Checks and reports](../checks-and-reports/) covers the full outcome
vocabulary.

## What intent cannot do

It checks presence, property and order, over the netlist. It does not simulate. A declared sequence
is verified as a gating chain in the connectivity, not as timing on a scope, so a board that wires
the enables correctly and still browns out at power-up is outside what a declaration can see.

It is also only as good as the names. A rail the declaration calls `PMIC_CORE_3V3` has to be called
that on the board, which is the same dependency [naming conventions](../naming-conventions/) exist to
make explicit rather than accidental.

## Where to go next

- [Checks and reports](../checks-and-reports/): what `needs-design-intent` means beside the other
  outcomes, and how `--fail-on` treats them.
- [Interface profiles](../interface-profiles/): the other declarative tier, per-project rather than
  per-design, for the shape of a bus rather than the shape of a board.
- [How a rule gets written](../../architecture/rules-and-checks/#how-a-rule-gets-written): where an
  intent declaration sits among the four ways to author a rule, and why the engine ships none of its
  own.

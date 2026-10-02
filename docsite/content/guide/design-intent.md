---
title: "Design intent"
description: "Declare what a board is supposed to contain, and have every revision checked against the architecture you agreed rather than against itself."
---

A {{ explainable "netlist" }} says what *is* wired. It cannot say what was *meant*. "The board has two
regulators" is a fact you can read off the file; "the board should have two regulators" is a decision
somebody made in a meeting, and nothing in the export records it.

A design-intent declaration writes that decision down as YAML beside the design, and `check` compares
the board against it. This tier catches a rail quietly moved to the wrong domain, a module
dropped during a cost reduction, or a power-up order that got reversed when the {{ explainable "power-tree" "power tree" }}
was redrawn. None of those look wrong in the schematic. They look wrong against what you said you
were building.

## Write a declaration

```yaml
# designs/gateway/design.yaml
name: gateway
entry: gateway.edn
intent:
  modules:
    - {name: regulators, class: regulator, count: 2}
    - {name: connectors, class: connector, count: 1}
    - {name: power tree, nets: [PMIC_MAIN_12V0, PMIC_CORE_3V3, PMIC_IO_1V8]}
    - {name: can, nets: [CAN1_CANH, CAN1_CANL, CAN1_TXD, CAN1_RXD]}
  nets:
    PMIC_MAIN_12V0: {nominal: 12.0, domain: main}
    PMIC_CORE_3V3:  {nominal: 3.3,  domain: io}
    PMIC_IO_1V8:    {nominal: 3.3,  domain: core}
```

That last rail is wrong on purpose, so the run below has something to catch. `PMIC_IO_1V8` is
declared at 3.3 V in the `core` domain, and it is an actual 1.8 V rail. Nothing about the schematic is
malformed. The board simply stopped matching the architecture, and only a declaration can notice.

## Where it goes

In the design's own `design.yaml`, under `intent:`. A design that belongs to a project needs no flag,
because the project reads its descriptor and finds the section:

{{ agniRun "content/guide/runs/intent-domain-mismatch.yaml" }}

Intent is per-**design**, unlike naming conventions, interface profiles and seeded parameters, which
are per-project. Each board has its own intended architecture, so the declaration lives with the
board rather than with the team, in the one file that already says which files the board is.

`--intent-path` on `check` and `review` names a file in the same shape, a `name` and an `intent:`
section, for a design that belongs to no project or to try a declaration before committing it. It
rides the request as the declaration's value, so on a design that already declares intent it REPLACES
that declaration for the run rather than being added to it, the way a request's naming convention
replaces the project's. A server takes no intent flag, since intent is per design and a flag would
apply one board's declaration to every design it serves (agni issue 831).

The section's schema is the `DesignIntent` message in `protos/agni/v1/config/intent.proto`, so a
declaration has the same shape in YAML, in JSON and on the wire, and a key the message does not have
fails the load with the line it sits on.

A separate `intent.yaml` beside the design is no longer read, and a project holding one fails to load
with a message saying where its declarations go (agni issue 824). So does a declaration written in
the earlier nine-form vocabulary, each refused key naming its replacement.

## The seven forms

Each form answers a question the netlist cannot. Three are keyed by what they describe (a block, a
net, a component), and the other four describe relationships between several nets.

| Form | Declares | Fails when |
|---|---|---|
| `modules` | a functional block the board must contain, by class or MPN with an optional count, or by the nets it must instantiate | a declared module is absent, the count is short, or a declared net is missing |
| `nets` | what each named net is, keyed by name (its voltage and domain, its peak draw, the protection it carries, its reset polarity, strap level or AC coupling) | the design contradicts any fact declared for it |
| `sequences` | the power-up order of groups of rails | the gating chain is absent, or runs the other way round |
| `strap_groups` | several strap nets read together as one binary number, and the value it encodes | the group does not encode the declared value, or two devices collide |
| `io_map` | which net lands on which pin of which device, and optionally what sits at the far end | the net is on a different pin, the declared net is absent, or the far end is wrong |
| `margin_factor` | the headroom every supply must have over its rails' declared peaks | a supply is rated below peak times the factor |
| `components` | what each named component is, keyed by ref-des (a connector's exposure) | a declared component is absent, or is not a connector |

A `nets` entry carries any of these facts, and each compiles to the same rule it always did.

```yaml
intent:
  nets:
    VDD_3V3:   {nominal: 3.3, domain: io, peak: 0.8, protect: [ovp, discharge]}
    MCU_NRST:  {reset: low}
    BOOT0:     {strap: low, min_ohms: 4700, max_ohms: 47000}
    PCIE_TX0_P: {ac_coupled: true}
  margin_factor: 1.2
```

A rail with a `nominal` and no `domain` is named by its voltage (`3.3V`), and rails sharing a domain
must share a voltage. `protect` takes `ovp` (a TVS or zener clamping the rail) and `discharge` (a
bleeder to ground). A resistance band belongs to a strap, so `min_ohms` without `strap` is a load
error.

A `components` entry says whether a connector faces the outside of the product. The exposure rules
(`esd-protection`, `esd-clamp-not-tvs`, `input-protection` and `reverse-blocking-absent`) treat every
connector as a way into the board, which is right for a cable entry and wrong for a module socket or
a mezzanine that joins two boards inside one enclosure. Whether a connector is which depends on the
product rather than the part. The same board-to-board connector is the module socket on one carrier
and the cable entry on another, and its part number and footprint rarely say either way, so the
engine does not guess from them.

```yaml
intent:
  components:
    J1:  {exposure: internal}   # the module connector
    J18: {exposure: internal}   # the expansion mezzanine
```

A connector declared `internal` leaves the exposure rules, so a net reaching only it is not checked
for ESD or input protection. A net that also reaches an undeclared connector still is. An undeclared
connector is external, and `external` can be written to say so. The declarations themselves compile
to `intent/exposure-declared`, which fails a ref-des the design does not have and a part that is not a
connector, because either would change nothing while reading as a declaration that took effect.

`io_map` is the largest of them in practice and the one most boards already have, usually as a
spreadsheet. On any board carrying a big MCU or SoC, someone decides which peripheral lands on which
pin long before the schematic exists, firmware is written against that decision, and the schematic is
drawn from it. Usually an assignment moves late, the map is updated, and one net
does not get redrawn. Nothing about the resulting board is electrically wrong, so every other rule
passes, and it surfaces at bring-up as a peripheral that does not respond.

```yaml
io_map:
  - net: I2C_SDA
    device: U3
    pin: '9'
  - net: MCU_NRST
    device: U3
    pin: PTC11                     # the datasheet's name works as well as the designator
    to: {device: U1, pin: '5'}     # optional far end
```

Write the pin either way. A map is authored in the vocabulary a datasheet and a firmware header use,
and a netlist answers in package designators, so both are resolved: `PTE7`, `PTE07`, `pte7` and a
name carrying a zero-width space pasted out of a spreadsheet all reach the same pin. A match that
needed any of that says so in the verdict, so a note always means something was inferred.

It compiles to four rules rather than one. Three of them ask whether the design kept the promises the
map made, and the fourth inverts the question to ask which nets the map never mentioned. That number
is usually the important one. A design with sixteen hundred nets and a map declaring two hundred has
two hundred checked and fourteen hundred unexamined, which is not the same as clean, and nothing else
in a run tells those apart. Nets it does not name report as `not-considered` rather than as failures,
because an undeclared net is a question nobody asked and not a fault in the board.

The three that check the promises are separated because a reviewer acts on them differently, and
because two of them catch opposite defects (a net the map declares and the netlist does not have is
usually a real disconnection, where a net the netlist has and the map does not declare is an
incomplete map). The far-end columns are the sparse ones. In the one real map we measured, roughly a
third of rows declared a far end, so `io-map-far-end` gives EVERY row a verdict and a row with no far
end reads `not-considered`. A rule that reported only the rows carrying one would show a clean result
over a third of the map and say nothing about the rest. `function` is accepted and NOT yet
evaluated, since deciding whether a function is legal on a pin needs the part's alternate-function table; every verdict on a row carrying one says so
outright.

A `peak` joins two tiers. The declaration supplies the demand, which no design
artifact carries, and a seeded {{ explainable "absolute-maximum-rating" "datasheet parameter" }}
supplies the regulator's capacity. Both halves have to be present or the rule stays quiet rather than
guessing.

### One rule per declared thing

Two review items bound to one rule name share its verdict. So anything a reviewer signs off
separately gets its own rule (WS3-058), and the naming follows what the sign-off is about.

| Form | Rules it compiles to | Why that grain |
|---|---|---|
| `modules` with nets, `sequences`, `strap_groups` | one per declared entry, named from a slug of it (`subsystem-<name>`, `sequence-<name>`, `strap-group-<name>`) | each entry is its own checklist item |
| `modules` without nets | fixed names (`module-missing`, `module-count`) | the review item is the mechanism rather than any one entry |
| `nets` | one per FACT (`voltage-domain-mismatch`, `protection-<kind>`, `property-<kind>`, `rail-current-capacity`, `rail-current-margin`, `load-switch-trip-below-budget`) | a reviewer signs off "every rail has its OVP clamp", not each rail |
| `io_map` | four fixed names, whatever the map's length | the exception, since nobody signs off "net 137 is on the right pin" as its own item |

`strap_groups` also compiles one `strap-address-collision` rule across all groups, because a
collision is between two groups and belongs to neither. It compiles only when two groups share a bus,
since over fewer it could only pass. `rail-current-margin` compiles only when the
declaration states a `margin_factor`, so an item bound to it reads needs-design-intent rather than
passing against a number nobody declared.

## With no declaration, nothing passes

There is no built-in intent, and that absence is deliberate. A generic statement of what a board
should contain says nothing, and a rule that enumerated its expectations *from the design* would
always agree with the design. So every intent rule iterates the declaration and probes the netlist,
never the reverse.

A design run with no declaration leaves
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

- [Checks and reports](../checks-and-reports/) explains what `needs-design-intent` means beside the
  other outcomes, and how `--fail-on` treats them.
- [Interface profiles](../interface-profiles/) is the other declarative tier, per-project rather
  than per-design, for how a bus is wired rather than what a board contains.
- [How a rule gets written](../../architecture/rules-and-checks/#how-a-rule-gets-written) shows
  where an intent declaration sits among the four ways to author a rule, and why the engine ships
  none of its own.

---
title: "intent/module-missing"
description: "A functional block the design intent declares required is absent from the design."
---

### Remedy

Add the missing block to the schematic, or amend the intent declaration if the architecture has moved on. One of the two is out of date, and only the author knows which.

### What it means

The design intent declares which functional blocks the schematic is required to contain (a SoC,
a CAN transceiver, a regulator). This rule fails once per declared module that no design component
satisfies. A module matches when any component carries its declared device class, or its exact MPN
(the MPN path resolves only on a model loaded with `--params`).

### Why engineers want it

"All required modules present" is a design-review question the netlist cannot answer on its own:
the schematic says what IS wired, not what was SUPPOSED to be there. A dropped block (a forgotten
transceiver, a regulator left off a respin) reads as a perfectly valid netlist. The declared
architecture is the external reference the design is checked against.

### Impact

A required functional block is absent, so the board is missing a capability its architecture called
for. Caught at review it is a one-line respin note; missed, it is a bring-up blocker or a field
recall.

![A declared module absent from the design is flagged; the module present is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/module-missing.svg)

### Scope note

The expectation set comes from the declaration, never from the netlist: the rule iterates the
declared modules and probes the design, so a missing module fails. A rule that enumerated modules
from the design would always pass (circular), the exact silent false-pass the honest-guard discipline
exists to prevent. There is no built-in intent; the declaration is loaded per design via
`--intent-path`, so a design run with none leaves the item not-automated rather than silently passing.

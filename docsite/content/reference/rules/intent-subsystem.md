---
title: "intent/subsystem"
description: "An architectural subsystem the design intent declares is missing a required part or net."
---

### Remedy

Add the missing part or net to the subsystem, or amend the declaration if the architecture changed and the intent document did not.

### What it means

The design intent declares named architectural subsystems (a clock tree, a reset scheme, the power
tree), each evidenced by a required source component and/or a set of nets that must all exist. This is
the family doc for every `intent/subsystem-<name>` rule: each declared subsystem compiles to its own
rule (so "clock architecture" and "reset architecture" bind and report independently), and each fails
when its source component is absent or any of its required nets is missing.

### Why engineers want it

A subsystem is a cluster of parts and nets that only works when all of it is present: a clock is a
crystal plus its load caps plus the oscillator net; a reset scheme is a supervisor plus the reset net.
Any one piece dropped in a schematic edit leaves a subsystem that looks half-wired but is functionally
absent. The declaration states the intended subsystem so the check verifies the design realizes it.

### Impact

An architectural subsystem the design was intended to contain is missing a required part or net: no
clock, no reset, or a power tree with a rail that never got routed. The board looks complete but a
whole function does not come up.

![A declared subsystem missing its source part or a required net is flagged; the complete subsystem is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/subsystem.svg)

### Scope note

One rule per declared subsystem, named `intent/subsystem-<slug>` (the subsystem name slugified); names
must slugify uniquely within a declaration. A subsystem checks its source (matched by class or MPN, like
a module) and each of its required nets. Like every intent rule it iterates the declaration and probes
the design, never enumerating the expected subsystems from the netlist.

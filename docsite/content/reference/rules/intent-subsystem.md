---
title: "intent/subsystem"
description: "An architectural subsystem the design intent declares is missing a required part or net."
---

### Remedy

Add the missing part or net to the subsystem, or amend the declaration if the architecture changed and the intent document did not.

### What it means

The design intent declares named architectural blocks (a clock tree, a reset scheme, the power
tree) as `modules` entries that list `nets`, each evidenced by the nets that must all exist and
optionally by a source component named by class or MPN. This is the family doc for every
`intent/subsystem-<name>` rule, and each module declaring nets compiles to its own rule (so "clock architecture" and "reset architecture" bind and report independently) that
fails when its source component is absent or any of its required nets is missing.

### Why engineers want it

A subsystem is a cluster of parts and nets that only works when all of it is present, the way a
clock is a crystal plus its load caps plus the oscillator net; a reset scheme is a supervisor plus
the reset net. Any one piece dropped in a schematic edit leaves a subsystem that looks half-wired
but is functionally absent. The declaration states the intended subsystem so the check verifies the
design realizes it.

### Impact

An architectural subsystem the design was intended to contain is missing a required part or net,
leaving no clock, no reset, or a power tree with a rail that never got routed. The board looks
complete but a whole function does not come up.

![A declared subsystem missing its source part or a required net is flagged; the complete subsystem is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/subsystem.svg)

### Scope note

```yaml
intent:
  modules:
    - {name: main clock, class: crystal, nets: [XTAL_IN, XTAL_OUT]}
    - {name: power tree, nets: [5V0, 3V3, 1V8]}
```

Names must slugify uniquely within a declaration. A subsystem checks its source (matched by class or
MPN, like a module) and each of its required nets. A module with no `nets` is checked by
`module-missing` and `module-count` instead, and a `count` belongs only there. Like every intent rule it iterates the
declaration and probes the design, never enumerating the expected subsystems from the netlist.

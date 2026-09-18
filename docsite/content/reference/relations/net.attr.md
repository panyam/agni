---
title: "net.attr"
description: "a net-level attribute DECLARED by the source file (external, global, power_driven), the twin of component.attr; a role the engine derived is net.role"
---

### What it is

`net.attr(net, key, value)` yields one row per attribute a net carries, as the design file declared
it. It is the twin of `component.attr`, and it is what a net's `attributes` map looks like from the
query surface.

Common keys are `external` (the net may continue onto a sheet the read did not cover), `global` (a
power symbol or global label distributes it), and `power_driven` (a pin asserts it as a supply).

### For hardware engineers

These are statements the design file makes about a net, rather than conclusions the engine draws from
its name. `external` is the honest one to know about: it marks a net whose full connectivity was not
read, so a rule that would otherwise report a one-pin net stays quiet rather than reporting a defect
that is an artefact of what was opened.

### For software engineers

The net-side half of an asymmetry that lasted until agni 691. A component's attributes have been
queryable since WS3-074; a net's reached only the SPEC language, one boolean fact per key
(`net.attr.external`, `net.attr.global`), so the same question could be asked of a part and not of a
net. Rows are one per key per net, and a net with no attributes yields none.

Declared, not derived. What the ENGINE worked out about a net from its name is `net.role`.

### Go projector

`netAttrFacts` in `stdlib/relations/facts.go` walks `Model.Nets()` and emits a row per entry in each
net's `attributes` map, exactly as `componentAttrFacts` does for a component.

### Datalog

Every attribute on a net:

```
net.attr(?n, ?k, ?v) => ?n, ?k, ?v
```

The nets a read could not see the whole of, which most connectivity rules exclude:

```
net.attr(?n, "external", "true") => ?n
```

---
title: "component.pin"
description: "a part-type pin of a placed component"
---

### What it is

`component.pin(ref_des, pin)` yields one row per part-type pin of every placed component, where
`(ref_des, pin)` names the pin and `pin` is its designator. It is the base pin-tier relation. Four
others refine it: `pin.role`, `pin.type`, `pin.net` and `pin.name`. A row asserts only that the pin
exists on a placed part, nothing about its role, direction, or connectivity.

### For hardware engineers

This relation enumerates every pin of a part, as the symbol's pin list gives them. It has rows
only when the source carries part-pin data (a KiCad symbol library, an EDIF cell), so a bare
netlist with no symbols yields nothing. You query it to inventory pins, or as the left
side of a join that asks a follow-up question about each one (what net is it on, what is its
role). A multi-section part can share a pin, such as a power pin drawn in both halves of a dual
op-amp. Such a pin appears once, deduplicated by designator.

### For software engineers

A pin is a **member of a class instance**. [The analogy
guide](../../../../docsite/content/reference/analogy.md) makes `PartType` the class, `Component`
the instance, and the pin one declared field. `component.pin` lists those members for every
instance. Rows are 1:1 with `PinInst` entities, and the pin-level rules quantify over those. It
joins to `pin.role` / `pin.type` / `pin.net` / `pin.name` on `(ref_des, pin)`, and each adds one
attribute of the same member. An empty result means the design carries no part-pin data (a
netlist-only source). The other tiers are silent in the same way when their data is missing.

### Go projector

`pinFacts` in `stdlib/relations/facts.go` iterates `Model.Pins()` and emits one `component.pin(ref_des, pin)` row per
`PinInst`. The pin set is built once when the model loads. Each component's sections are walked,
and each declared pin designator is added once. A designator already seen in an earlier section is
not re-added, so a multi-section part's shared pins are counted once. One row per part-type pin;
empty when no component declares pins.

### Datalog

Every pin the design declares:

```
component.pin(?r, ?p) => ?r
```

Join to `pin.net` to keep only the pins that actually connect to a net (the connected ones):

```
component.pin(?r, ?p), pin.net(?r, ?p, ?n) => ?n
```

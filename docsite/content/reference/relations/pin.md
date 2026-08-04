---
title: "pin"
description: "a part-type pin of a placed component"
---

### What it is

`pin(ref_des, pin)` yields one row per part-type pin of every placed component: `(ref_des,
pin)` names the pin, where `pin` is its designator. It is the base pin-tier relation the other
three (`pin.role`, `pin.type`, `pin.net`) refine. A row asserts only that the pin exists on a
placed part, nothing about its role, direction, or connectivity.

### For hardware engineers

This is the enumeration of every pin the tool knows a part has, from the symbol's pin list.
It exists only when the source carries part-pin data (a KiCad symbol library, an EDIF cell), so
a bare netlist with no symbols yields nothing. You query it to inventory pins, or as the left
side of a join that asks a follow-up question about each one (what net is it on, what is its
role). Shared pins on a multi-section part (a power pin drawn in both halves of a dual op-amp)
appear once, deduplicated by designator.

### For software engineers

A pin is a **member of a class instance** (see ANALOGY.md: `PartType` is the class, `Component`
the instance, the pin is one declared field). `pin` is the projection that lists those members
for every instance. Rows are 1:1 with the `PinInst` entities the pin-level rules quantify over.
It joins to `pin.role` / `pin.type` / `pin.net` on `(ref_des, pin)`, each adding one attribute of
the same member. An empty result means the design carries no part-pin data (a netlist-only
source), the same silent-by-construction posture the other tiers have.

### Go projector

`pinFacts` in `check/facts.go` iterates `Model.Pins()` and emits one `pin(ref_des, pin)` row per
`PinInst`. The pin set is built once when the model loads: each component's sections are walked,
and each declared pin designator is added once (a designator already seen in an earlier section is
not re-added, so a multi-section part's shared pins are counted once). One row per part-type pin;
empty when no component declares pins.

### Datalog

Every pin the design declares:

```
pin(?r, ?p) => ?r
```

Join to `pin.net` to keep only the pins that actually land on a net (the connected ones):

```
pin(?r, ?p), pin.net(?r, ?p, ?n) => ?n
```

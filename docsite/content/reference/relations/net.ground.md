---
title: "net.ground"
description: "the net is a ground rail (name-derived)"
---

### What it is

`net.ground(net)` yields one row per net whose name reads as a ground node: `GND` or `EARTH`
anywhere in the leaf name, or a `VSS` prefix. It is name-derived, since a directionless netlist
carries the name as the only evidence that a net is ground.

`net.ground` is the ground-only subset of `rail`. `rail` covers both power and ground, because
`Model.IsPowerRail` ORs the ground test into the rail test. So every `net.ground` net is also a
`rail` net, and a rule reads `rail(?r), not net.ground(?r)` when it means "a supply rail, not
ground."

### For hardware engineers

Ground has to be told apart from a supply rail because rules treat the two differently. A grounded
crystal case pin is not the Vdd pin of an active oscillator; a decoupling cap to ground is a
different role than a cap between two supplies. During a review you query `net.ground` to confirm the
engine recognises your ground naming, and you subtract it from `rail` to reason about supplies alone.

### For software engineers

`net.ground` is a filtered projection over `Nets()`, the same shape as `rail`. Rows are 1:1 with
ground nets. An empty result means the read found no ground at all, which on a real design usually
points at a naming convention the lexicon does not yet cover rather than a board with no ground.

**A net is ground by NAME or by DECLARATION, whichever the source supports.** Most formats carry only
the name, which is why the naming lexicon exists. IPC-2581 states it outright on
`LogicalNet/@netClass`, so a net called `N$17` can be authoritatively ground with nothing in the name
to go on (WS1-051). The two sources are unioned at ingestion, not ranked, so a declaration never
costs you a role the name would have found.

### Go projector

`netGroundFacts` in `stdlib/relations/facts.go` walks `Model.Nets()` and emits a row for each net
where `Model.IsGroundNet` holds. That goes through `check.NetHasRole`, which prefers the role SET
stamped at ingestion (`ir.Net.roles`, filled by `classify.StampNetRoles`) and falls back to matching
the name when a net carries no roles at all. The naming lexicon's `IsGround` matches `GND`, `EARTH`,
or a `VSS` prefix on the hierarchy leaf, case-insensitive; the stamped set additionally carries any
role the source declared. One row per ground net; empty when neither source finds one.

### Datalog

List every ground net:

```
net.ground(?n) => ?n
```

Isolate the supply rails by subtracting ground from the rail set:

```
rail(?n), not net.ground(?n) => ?n
```

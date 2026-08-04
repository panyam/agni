---
title: "rail"
description: "the net is a power or ground rail"
---

### What it is

`rail(net)` yields one row per net the engine treats as a power or ground rail. It covers both
polarities: a supply net (`+5V`, `VCC`, `3V3`) and a ground net (`GND`, `VSS`) both answer `rail`.
A net qualifies when it is asserted-driven (a `PWR_FLAG` or equivalent directive), carries the
design-wide `global` attribute, or its name reads as a rail or ground name.

`net.ground` is the ground-only subset of this relation: every `net.ground` row is also a `rail`
row, but a supply rail answers `rail` and not `net.ground`. So `rail(?n), not net.ground(?n)`
isolates the supply rails.

### For hardware engineers

These are the distribution nets, the ones a design taps rather than routes point-to-point. During a
review you query `rail` to check that a rule's rail set matches your intent: a signal net that shows
up here has a name that collides with a supply convention or was marked driven when it should not be.
It is also the join a protection or pull-up check needs, since "does this signal reach a rail" is the
shape of many connectivity rules.

### For software engineers

A rail is a global singleton in the design graph (see ANALOGY.md): everything tied to `+5V` is one
electrical node, and a reachability walk must not follow an edge into it or the whole graph collapses
into one component. `rail` is the name of that singleton set. The relation is a filtered projection
over `Nets()`, so rows are 1:1 with nets that pass the rail predicate, and an empty result means the
read found no driven, global, or rail-named net.

### Go projector

`railFacts` in `check/facts.go` walks `Model.Nets()` and emits a row for each net where
`Model.IsPowerRail(name)` holds. `IsPowerRail` (in `check/locate.go`) ORs four conditions: the
`power_driven` attribute, the `global` attribute, `isGroundName`, and `isPowerRailName` (both from the
active naming lexicon in `check/rolenames.go`). One row per rail net; empty when the design has no
rail-named, global, or asserted-driven net.

### Datalog

List every power or ground rail:

```
rail(?n) => ?n
```

Find the components sitting on a rail (the loads and sources on power distribution):

```
rail(?n), component-on-net(?r, ?n) => ?r
```

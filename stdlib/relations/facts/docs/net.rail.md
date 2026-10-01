## net.rail

### What it is

`net.rail(net)` yields one row per net the engine treats as a power or ground rail. It covers both
polarities, so a supply net (`+5V`, `VCC`, `3V3`) and a ground net (`GND`, `VSS`) both answer `net.rail`.
A net qualifies when it is asserted-driven (a `PWR_FLAG` or equivalent directive), carries the
design-wide `global` attribute, or its name reads as a rail or ground name and it is not a
regulator's own feedback, switching, control or gate-drive node.

`net.ground` is the ground-only subset of this relation, so every `net.ground` row is also a `net.rail`
row while a supply rail answers `net.rail` and not `net.ground`. So `net.rail(?n), not net.ground(?n)`
isolates the supply rails.

### For hardware engineers

These are the distribution nets, the ones a design taps rather than routes point-to-point. During a
review you query `net.rail` to check that a rule's rail set matches your intent, since a signal net that shows
up here has a name that collides with a supply convention or was marked driven when it should not be.
It is also the join a protection or pull-up check needs, since "does this signal reach a rail" is the
question behind many connectivity rules.

### For software engineers

A rail is a global singleton in the design graph (see
[the analogy guide](../../../../docsite/content/reference/analogy.md)), since everything tied to
`+5V` is one electrical node, and a reachability walk must not follow an edge into it or the whole graph collapses
into one component. `net.rail` is the name of that singleton set. The relation is a filtered projection
over `Nets()`, so rows are 1:1 with nets that pass the rail predicate, and an empty result means the
read found no driven, global, or rail-named net.

### Go projector

`railFacts` in `stdlib/relations/facts.go` walks `Model.Nets()` and emits a row for each net where
`Model.IsPowerRail(name)` holds. `IsPowerRail` (in `core/check/locate.go`) ORs four conditions, which
are the `power_driven` attribute, the `global` attribute, `IsGroundNet`, and `IsRailNet` (both reading
the model's naming lexicon, and `IsRailNet` subtracting regulator-internal nets). One row per rail net; empty
when the design has no rail-named, global, or asserted-driven net.

### Datalog

List every power or ground rail:

```
net.rail(?n) => ?n
```

Find the components sitting on a rail (the loads and sources on power distribution):

```
net.rail(?n), component.net(?r, ?n) => ?r
```

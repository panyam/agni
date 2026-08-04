## net.ground

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

`net.ground` is a filtered projection over `Nets()`, the same shape as `rail` but with a narrower
name predicate. Rows are 1:1 with ground-named nets. An empty result means no net in the read matched
a ground name, which on a real design usually points at a naming convention the lexicon does not yet
cover rather than a board with no ground.

### Go projector

`netGroundFacts` in `check/facts.go` walks `Model.Nets()` and emits a row for each net where
`isGroundName(name)` holds. `isGroundName` (in `check/rule_decoupling_present.go`) delegates to the
active naming lexicon's `IsGround` (`check/rolenames.go`), matching `GND`, `EARTH`, or a `VSS`
prefix on the hierarchy leaf, case-insensitive. One row per ground-named net; empty when no net name
matches.

### Datalog

List every ground net:

```
net.ground(?n) => ?n
```

Isolate the supply rails by subtracting ground from the rail set:

```
rail(?n), not net.ground(?n) => ?n
```

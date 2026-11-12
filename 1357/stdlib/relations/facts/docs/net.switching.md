## net.switching

### What it is

`net.switching(net)` yields one row per net whose name reads as a regulator's power-stage node, meaning a leaf
name ending in `_SW`, `_BOOT`, `_PHASE`, or `_LX`. It is name-derived and is the twin of
`net.feedback`, since both relations name a net that matches the rail vocabulary and is not a rail.

### For hardware engineers

The switch node is the point between a buck converter's high-side and low-side transistors, before
the inductor. It does not sit at the output voltage. It slams between ground and the *input* rail at
the switching frequency, and that makes it the highest dV/dt net on most boards. `_BOOT` is the
bootstrap capacitor's node. It sits on top of the switch node and moves with it, so the high-side
gate driver has a supply above the rail it is switching. `_PHASE` and `_LX` are the switch node
under other vendors' spellings.

All of them are conventionally named after the rail the converter produces, so a 12V converter's
switch node is `12V_SW` and is never at 12V. A review cares about two consequences. Probing one is
worse than useless, because a test point there loads the fastest edge in the design and couples it into
whatever the probe is attached to. And any rule that reasons about a rail's voltage will be wrong
about this net by the whole input-to-output ratio.

During a review you query `net.switching` to list the power-stage nodes, and `net.rail` already
leaves it out alongside `net.feedback`, so a probe-point or decoupling rule does not treat a switch node
as ordinary distribution.

### For software engineers

A switch node's name makes it look like a member of an object it does not belong to. `12V_SW`
reads as a member of the 12V rail, but it is a separate net with a different value. `net.switching`
is a filtered projection over `Nets()` with the naming predicate, so rows are 1:1 with
switching-named nets, and an empty result means no net name matched the power-stage lexicon.

### Go projector

`switchingFacts` in `stdlib/relations/facts.go` walks `Model.Nets()` and emits a row for each net
carrying the `switching` role, falling back to `Model.IsSwitchingName` for a net that skipped the
ingestion stamp. The role is stamped by `classify.StampNetRoles` from the active naming lexicon, so a
project extends the patterns in its `conventions:` section rather than patching the engine. One row
per switching-named net; empty when no net matches.

### Datalog

List every power-stage node:

```
net.switching(?n) => ?n
```

Every supply rail that is neither a sense node nor a power-stage node. A probe-point or pull-up rule
may treat this set as ordinary distribution:

```
net.rail(?n), not net.feedback(?n), not net.switching(?n) => ?n
```

Note that `net.rail` already excludes a rail-NAMED net carrying either role, because `Model.IsRailNet`
subtracts them before the rail role is granted. A net marked `global` or asserted-driven still
answers `net.rail` whatever its name, since `Model.IsPowerRail` admits those before it consults the
name. Write the form above anyway when the query sits beside one that does not go through the rail
relation, such as a rule built from the name FFIs.

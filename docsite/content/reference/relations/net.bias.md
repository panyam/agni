---
title: "net.bias"
description: "a bias resistor holds the net at a rail (high) or ground (low); absent when unbiased or held by a divider"
---

### What it is

`net.bias(net, level)` yields one row per net a bias resistor holds at a rail: `high` when the
resistor reaches a supply, `low` when it reaches ground. A net with no bias resistor produces no row,
and so does a net held by a divider, so `not net.bias(?n, ?_)` reads as "unbiased", a genuinely
different state from "biased the other way".

### For hardware engineers

A bias resistor decides what a line does when nothing is actively driving it. A reset line that must
sit released between drives needs a pull-up; an enable that must default off needs a pull-down. The
resistor is what makes the resting state a design decision rather than an accident.

Getting it backwards is a classic bring-up failure: a pull-down on an active-low reset holds the part
in reset from the moment power comes up, and the board looks dead. One resistor to the wrong net,
invisible until someone meters the pin.

Query this to see what every line rests at, or join it against a declared intent to find lines that
rest at the wrong level.

### Two clauses, and the second is the one that gets forgotten

The bias resistor commonly sits directly between the net and its rail. It does not have to. It can
reach the rail through further passives, a filter or a second resistor, and a check that only looked
for the direct arrangement would silently report those nets as unbiased.

Both forms count here. `profiles.pullupRule` learned this the hard way (WS3-108): its walk-based form
could not enter a wide rail at all, so a direct clause had to be added beside it. Keeping both in one
predicate is what stops the next consumer reimplementing half of it.

### A divider reports neither

A net with both a pull-up and a pull-down sits at an intermediate level. It is not held at either
rail, so it yields no row rather than an arbitrary one. A caller asking "is this held asserted" gets
the honest answer instead of a coin flip.

### Go projector

`netBiasFacts` in `stdlib/relations/facts.go` calls `check.NetBias`, which lives in
`core/check/guards.go` beside the other Model-level predicates. The Go rules and any datalog query
therefore read ONE definition, and this relation projects it rather than reimplementing it.

### Datalog

Every biased net and which way:

```
net.bias(?n, ?level) => ?n, ?level
```

Lines that rest low, the ones an active-low reset must not be among:

```
net.bias(?n, "low") => ?n
```

Unbiased nets that a part still drives, which is where an undeclared internal pull is doing the work:

```
component-on-net(?r, ?n), not net.bias(?n, ?_) => ?n, ?r
```

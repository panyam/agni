---
title: "num.unit"
description: "binds the number's unit (\"V\", \"A\"), or absent for a number without one; the number must be bound"
---

### What it is

`num.unit(number, unit)` binds `unit` to the unit of `number`, such as `"V"` or `"A"`. The number
must be bound. Calling it with an unbound number is an evaluation error. A number without a unit
binds an absent `unit`, not the empty string, so `absent(?u)` finds bare numbers. A value that is not
a number gives no row, absent included. With `unit` bound too, `num.unit` tests whether the number
has that unit. `num.unit` is a computed predicate (kind `predicate` in the catalog), not a stored
fact.

### For hardware engineers

A datasheet limit table mixes voltages and currents in one column. `num.unit` says which is which,
so a voltage check can keep the volt rows and leave the current limits alone. It also shows the
unit of a computed value, since the agni table prints only the number.

### For software engineers

`num.unit` is a getter on a unit-carrying number. An aggregate over numbers in different units is
absent, because the engine does not guess a conversion. Group by `num.unit` to get one answer per
unit instead.

### Go projector

`num.unit` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with `num.value`, in `Numbers` in jaala's
`stdlib/numbers.go`. `core/facts/registry.go` installs that library into the vocabulary every query
uses.

### Datalog

Each datasheet upper limit with its unit:

```
param.range(?mpn, ?s, ?k, ?min, ?max), num.unit(?max, ?u) => ?mpn, ?s, ?max, ?u
```

Only the limits stated in volts:

```
param.range(?mpn, ?s, ?k, ?min, ?max), num.unit(?max, "V") => ?mpn, ?s, ?k, ?max
```

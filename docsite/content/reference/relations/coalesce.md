---
title: "coalesce"
description: "binds the value, or the fallback when the value is absent"
---

### What it is

`coalesce(value, fallback, result)` binds `result` to `value`, or to `fallback` when `value` is
absent. The first two arguments must be bound, and an absent value counts as bound. Calling it with
either of them unbound is an evaluation error.

Over two absent inputs `coalesce` binds an absent `result`. It still answers a row, so a later
`absent(?result)` can find the case where neither input was stated. More fallbacks chain through
more calls, `coalesce(?a, ?b, ?t), coalesce(?t, ?c, ?v)`. `coalesce` is a computed predicate (kind
`predicate` in the catalog), not a stored fact.

### For hardware engineers

A datasheet range with no stated floor still needs a floor for some checks. A rail input has a
minimum of zero volts in practice, and a check can say so once with `coalesce(?min, 0V, ?lo)`. The
row then compares like every other row, and the stated minimum wins wherever the datasheet gives one.

### For software engineers

`coalesce` is SQL's `COALESCE(a, b)` with exactly two inputs, written as a relation with its result
as the third argument. It tests for absent, not for an empty string or a zero. A stated `""` or `0`
passes through unchanged.

### Go projector

`coalesce` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with `absent`, in `Absent` in jaala's
`stdlib/absent.go`. `core/facts/registry.go` installs that library into the vocabulary every query
uses.

### Datalog

Every datasheet limit with its lower bound, reading zero where the datasheet states none:

```
param.range(?mpn, ?s, ?k, ?min, ?max), coalesce(?min, 0, ?lo) => ?mpn, ?s, ?k, ?lo, ?max
```

The limits that state neither bound. The upper bound falls back to the lower, and the result is
absent only when both are:

```
param.range(?mpn, ?s, ?k, ?min, ?max), coalesce(?max, ?min, ?b), absent(?b) => ?mpn, ?s, ?k
```

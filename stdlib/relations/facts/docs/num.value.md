## num.value

### What it is

`num.value(number, value)` binds `value` to `number` with its unit removed, so 5 V and 5 A both bind
5. The number must be bound. Calling it with an unbound number is an evaluation error. A value that
is not a number gives no row, absent included. `num.value` is a computed predicate (kind `predicate`
in the catalog), not a stored fact.

The bare value is the number as stored. A literal with a prefix folds to its base unit first, so
`num.value(3300mV, ?x)` binds 3.3.

### For hardware engineers

Some questions ignore the unit on purpose. A report may want the raw figure from every limit in a
datasheet table, volts and amps side by side. `num.value` gives that figure, and `num.unit` gives the
unit beside it.

### For software engineers

`num.value` is a getter that drops the type. A min, max or sum over numbers in different units is
absent, because the engine refuses to mix them. Strip the units with `num.value` when the raw numbers
are what the query means.

### Go projector

`num.value` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with `num.unit`, in `Numbers` in jaala's
`stdlib/numbers.go`. `core/facts/registry.go` installs that library into the vocabulary every query
uses.

### Datalog

Each rail's voltage as a bare number, beside its unit:

```
net.nominal_voltage(?n, ?v), num.value(?v, ?x), num.unit(?v, ?u) => ?n, ?x, ?u
```

A prefixed literal, folded to its base unit:

```
num.value(3300mV, ?x) => ?x
```

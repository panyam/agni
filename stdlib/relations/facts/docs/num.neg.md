## num.neg

### What it is

`num.neg(a, b)` relates two numbers by `b = -a`. It runs from either side. With `a` bound it binds
`b`, and with `b` bound it binds `a`. With neither bound it is an evaluation error. The unit carries
over, so the negation of 3.3 V is -3.3 V. A value that is not a number gives no row. `num.neg` is a
computed predicate (kind `predicate` in the catalog), not a stored fact.

### For hardware engineers

A split supply has a negative rail with the same magnitude as its positive one. An op-amp fed from
±12 V needs both ends checked against its supply range. `num.neg` writes the negative end from the
positive one.

### For software engineers

`num.neg` is unary minus as a relation. Since it runs both ways, a test on the result also works,
as in `num.neg(?x, -3.3V)`. The agni table prints the number alone, and `num.unit` reads the unit
back.

### Go projector

`num.neg` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with `num.add`, `num.sub`, `num.mul` and
`num.div`, in `Arithmetic` in jaala's `stdlib/arith.go`. `core/facts/registry.go` installs that
library into the vocabulary every query uses.

### Datalog

Each rail's voltage negated, with the unit kept:

```
net.nominal_voltage(?n, ?v), num.neg(?v, ?x), num.unit(?x, ?u) => ?n, ?x, ?u
```

Run backward from the result:

```
num.neg(?x, -3.3V), num.unit(?x, ?u) => ?x, ?u
```

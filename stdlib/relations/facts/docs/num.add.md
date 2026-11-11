## num.add

### What it is

`num.add(a, b, c)` relates three numbers by `c = a + b`. It runs from any two of the three. With `a`
and `b` bound it binds `c`. With `c` and one operand bound it binds the other operand. With fewer than
two bound it is an evaluation error.

A sum needs one unit. A bare number takes the other side's unit, so `num.add(3.3V, 0.5, ?c)` binds
3.8 V. Adding volts to amps has no result and gives no row, as does a value that is not a number. A
forward sum is rounded to 15 significant digits when the rounding still recovers both operands. A
backward answer keeps full float precision, so `num.add(?a, 1.8V, 3.3V)` binds 1.4999999999999998.
`num.add` is a computed predicate (kind `predicate` in the catalog), not a stored fact.

### For hardware engineers

A margin check adds a tolerance to a nominal value. A rail at 3.3 V with 0.5 V of overshoot reaches
3.8 V, and that sum is the value to hold against a part's absolute maximum. `num.add` writes that
sum in the query and keeps the volts attached.

### For software engineers

`num.add` is `+` as a relation, so it also solves for a missing operand. A query writes arithmetic
through these functions. `?y = ?a + ?b` runs as `num.add(?a, ?b, ?y)`. The agni table prints the
number alone, and `num.unit` reads the unit back.

### Go projector

`num.add` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with `num.sub`, `num.mul`, `num.div` and
`num.neg`, in `Arithmetic` in jaala's `stdlib/arith.go`. `core/facts/registry.go` installs that
library into the vocabulary every query uses.

### Datalog

Each rail with 0.5 V of overshoot added, and the unit of the sum:

```
net.nominal_voltage(?n, ?v), num.add(?v, 0.5V, ?x), num.unit(?x, ?u) => ?n, ?x, ?u
```

Two literals:

```
num.add(2, 3, ?c) => ?c
```

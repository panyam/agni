## num.sub

### What it is

`num.sub(a, b, c)` relates three numbers by `c = a - b`. It runs from any two of the three. With `a`
and `b` bound it binds `c`. With `c` and one operand bound it binds the other operand. With fewer than
two bound it is an evaluation error.

Units follow the same rule as `num.add`. Both sides need one unit, and a bare number takes the other
side's. Subtracting amps from volts gives no row, as does a value that is not a number. A forward
difference is rounded to 15 significant digits when the rounding still recovers both operands, so
`num.sub(12V, 1.8V, ?c)` binds 10.2. `num.sub` is a computed predicate (kind `predicate` in the
catalog), not a stored fact.

### For hardware engineers

Headroom is a difference. A regulator fed from a 12 V input has 12 V minus its output rail across
it, and a linear regulator burns that drop as heat. `num.sub` computes the drop for every rail in
one query.

### For software engineers

`num.sub` is `-` as a relation. Bound on the result it solves for either operand, so `num.sub(5V,
?b, 2V)` binds 3 V. The agni table prints the number alone, and `num.unit` reads the unit back.

### Go projector

`num.sub` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with `num.add`, `num.mul`, `num.div` and
`num.neg`, in `Arithmetic` in jaala's `stdlib/arith.go`. `core/facts/registry.go` installs that
library into the vocabulary every query uses.

### Datalog

The drop from the 12 V input to each rail:

```
net.nominal_voltage(?n, ?v), num.sub(12V, ?v, ?h) => ?n, ?h
```

Solving for the second operand:

```
num.sub(5V, ?b, 2V), num.unit(?b, ?u) => ?b, ?u
```

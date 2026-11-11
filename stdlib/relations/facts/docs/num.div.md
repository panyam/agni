## num.div

### What it is

`num.div(a, b, c)` relates three numbers by `c = a / b`. It runs from any two of the three. With `a`
and `b` bound it binds `c`. With `c` and one operand bound it binds the other operand. With fewer than
two bound it is an evaluation error.

Units divide. Volts over amps is ohms, and volts over ohms is amps. Division by zero has no result
and gives no row, so `num.div(1, 0, ?x)` answers nothing rather than an infinity. A value that is not
a number also gives no row. `num.div` is a computed predicate (kind `predicate` in the catalog), not
a stored fact.

### For hardware engineers

A current-limiting resistor is a quotient. A rail driving 10 mA into a load needs the rail voltage
over 10 mA, which is 330 Ω on a 3.3 V rail. `num.div` computes it for every rail, and the result
comes out in ohms.

### For software engineers

`num.div` is `/` as a relation that refuses to divide by zero. Bound on the result it solves for an
operand, so `num.div(3.3V, ?r, 0.001A)` binds the resistance. A backward answer keeps full float
precision, so that call binds 3299.9999999999995. The agni table prints the number alone, and
`num.unit` reads the unit back.

### Go projector

`num.div` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with `num.add`, `num.sub`, `num.mul` and
`num.neg`, in `Arithmetic` in jaala's `stdlib/arith.go`. `core/facts/registry.go` installs that
library into the vocabulary every query uses.

### Datalog

The resistance that draws 10 mA from each rail:

```
net.nominal_voltage(?n, ?v), num.div(?v, 0.01A, ?r), num.unit(?r, ?u) => ?n, ?r, ?u
```

Ohm's law over literals, 5 V across 1 kΩ:

```
num.div(5V, 1000ohm, ?i), num.unit(?i, ?u) => ?i, ?u
```

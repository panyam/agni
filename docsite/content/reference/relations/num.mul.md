---
title: "num.mul"
description: "c = a * b, run from any two of the three; units follow the vocabulary's table"
---

### What it is

`num.mul(a, b, c)` relates three numbers by `c = a * b`. It runs from any two of the three. With `a`
and `b` bound it binds `c`. With `c` and one operand bound it divides to bind the other operand. With
fewer than two bound it is an evaluation error.

Units multiply. Amps times ohms is volts, and volts times amps is watts, because the vocabulary
defines each electrical unit through the SI base units. Solving for an operand when the other one is
zero has no result and gives no row. So does a value that is not a number. `num.mul` is a computed
predicate (kind `predicate` in the catalog), not a stored fact.

### For hardware engineers

Ohm's law and power are products. 2 mA through 1 kΩ is 2 V, and a 3.3 V rail delivering 0.5 A is
1.65 W. `num.mul` computes either, and the result carries the right unit with no conversion table in
the query.

### For software engineers

`num.mul` is `*` as a relation, with dimensional analysis on the units. `num.mul(?i, ?r, ?v)` finds
whichever of the three is missing, so one relation covers V = IR solved three ways. The agni table
prints the number alone, and `num.unit` reads the unit back.

### Go projector

`num.mul` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with `num.add`, `num.sub`, `num.div` and
`num.neg`, in `Arithmetic` in jaala's `stdlib/arith.go`. The unit table comes from `Units` in the
same file. `core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

The power each rail delivers at 0.5 A, with its unit:

```
net.nominal_voltage(?n, ?v), num.mul(?v, 0.5A, ?p), num.unit(?p, ?u) => ?n, ?p, ?u
```

Ohm's law over literals:

```
num.mul(0.002A, 1000ohm, ?v), num.unit(?v, ?u) => ?v, ?u
```

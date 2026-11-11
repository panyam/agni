## num.mul

### What it is

`num.mul(a, b, c)` relates three numbers by `c = a * b`.

`num.mul` comes from jaala's standard library. The
[Arithmetic](https://panyam.github.io/jaala/reference/builtins/#arithmetic) section of jaala's
built-ins reference says which arguments must be bound and what an absent or malformed input does,
and its examples run in jaala's own tests.

### For hardware engineers

Ohm's law and power are products. 2 mA through 1 kΩ is 2 V, and a 3.3 V rail delivering 0.5 A is
1.65 W. `num.mul` computes either, and the result carries the right unit with no conversion table in
the query.

### For software engineers

`num.mul` is `*` as a relation, with dimensional analysis on the units. `num.mul(?i, ?r, ?v)` finds
whichever of the three is missing, so one relation covers V = IR solved three ways. The agni table
prints the number alone, and `num.unit` reads the unit back.

### Datalog

The power each rail delivers at 0.5 A, with its unit:

```
net.nominal_voltage(?n, ?v), num.mul(?v, 0.5A, ?p), num.unit(?p, ?u) => ?n, ?p, ?u
```

Ohm's law over literals:

```
num.mul(0.002A, 1000ohm, ?v), num.unit(?v, ?u) => ?v, ?u
```

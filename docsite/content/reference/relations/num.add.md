---
title: "num.add"
description: "c = a + b, run from any two of the three; units follow the vocabulary's table"
---

### What it is

`num.add(a, b, c)` relates three numbers by `c = a + b`.

`num.add` comes from jaala's standard library. The
[Arithmetic](https://panyam.github.io/jaala/reference/builtins/#arithmetic) section of jaala's
built-ins reference says which arguments must be bound and what an absent or malformed input does,
and its examples run in jaala's own tests.

### For hardware engineers

A margin check adds a tolerance to a nominal value. A rail at 3.3 V with 0.5 V of overshoot reaches
3.8 V, and that sum is the value to hold against a part's absolute maximum. `num.add` writes that
sum in the query and keeps the volts attached.

### For software engineers

`num.add` is `+` as a relation, so it also solves for a missing operand. A query writes arithmetic
through these functions. `?y = ?a + ?b` runs as `num.add(?a, ?b, ?y)`. The agni table prints the
number alone, and `num.unit` reads the unit back.

### Datalog

Each rail with 0.5 V of overshoot added, and the unit of the sum:

```
net.nominal_voltage(?n, ?v), num.add(?v, 0.5V, ?x), num.unit(?x, ?u) => ?n, ?x, ?u
```

Two literals:

```
num.add(2, 3, ?c) => ?c
```

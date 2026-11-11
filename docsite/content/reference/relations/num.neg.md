---
title: "num.neg"
description: "b = -a, run from either; the unit carries over"
---

### What it is

`num.neg(a, b)` relates two numbers by `b = -a`.

`num.neg` comes from jaala's standard library. The
[Arithmetic](https://panyam.github.io/jaala/reference/builtins/#arithmetic) section of jaala's
built-ins reference says which arguments must be bound and what an absent or malformed input does,
and its examples run in jaala's own tests.

### For hardware engineers

A split supply has a negative rail with the same magnitude as its positive one. An op-amp fed from
±12 V needs both ends checked against its supply range. `num.neg` writes the negative end from the
positive one.

### For software engineers

`num.neg` is unary minus as a relation. Since it runs both ways, a test on the result also works, as
in `num.neg(?x, -3.3V)`. The agni table prints the number alone, and `num.unit` reads the unit back.

### Datalog

Each rail's voltage negated, with the unit kept:

```
net.nominal_voltage(?n, ?v), num.neg(?v, ?x), num.unit(?x, ?u) => ?n, ?x, ?u
```

Run backward from the result:

```
num.neg(?x, -3.3V), num.unit(?x, ?u) => ?x, ?u
```

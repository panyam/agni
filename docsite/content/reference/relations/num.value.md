---
title: "num.value"
description: "binds the number without its unit, so 5 V and 5 A both bind 5; the number must be bound"
---

### What it is

`num.value(number, value)` binds `value` to `number` with its unit removed, so 5 V and 5 A both bind
5.

`num.value` comes from jaala's standard library. The [Numbers and
units](https://panyam.github.io/jaala/reference/builtins/#numbers-and-units) section of jaala's
built-ins reference says which arguments must be bound and what an absent or malformed input does,
and its examples run in jaala's own tests.

### For hardware engineers

Some questions ignore the unit on purpose. A report may want the raw figure from every limit in a
datasheet table, volts and amps side by side. `num.value` gives that figure, and `num.unit` gives
the unit beside it.

### For software engineers

`num.value` is a getter that drops the type. Strip the units with `num.value` when the raw numbers
are what the query means.

### Datalog

Each rail's voltage as a bare number, beside its unit:

```
net.nominal_voltage(?n, ?v), num.value(?v, ?x), num.unit(?v, ?u) => ?n, ?x, ?u
```

A prefixed literal, folded to its base unit:

```
num.value(3300mV, ?x) => ?x
```

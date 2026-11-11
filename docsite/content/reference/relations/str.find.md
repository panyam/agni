---
title: "str.find"
description: "binds each non-empty match of an unanchored regular expression with its index from 0; given an index, binds that match alone, a negative index counting from the end"
---

### What it is

`str.find(string, regex, index, match)` binds each match of a regular expression in `string`,
numbered from 0.

`str.find` comes from jaala's standard library. The [Building
text](https://panyam.github.io/jaala/reference/builtins/#building-text) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A net name carries numbers in several places: a bus instance, a lane, a bit. `str.find` reads them
one at a time, which `str.split` cannot do when the separators vary. A regex reads every run of
digits, including the 2 inside `I2C_SCL`, so write the expression to match only the part you mean.

### For software engineers

It is Go's `regexp.FindAllString`, one row per match.

### Datalog

The first run of digits in each net name:

```
entity(?n, "net"), str.find(?n, "[0-9]+", 0, ?m) => ?n, ?m
```

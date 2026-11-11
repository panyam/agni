---
title: "str.split"
description: "binds each part of the string between separators with its index from 0; given an index, binds that part alone, a negative index counting from the end (-1 is the last part)"
---

### What it is

`str.split(string, separator, index, part)` binds each part of `string` between separators, numbered
from 0.

`str.split` comes from jaala's standard library. The [Building
text](https://panyam.github.io/jaala/reference/builtins/#building-text) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A hierarchical net name is a path, such as `/CSI/CAM0.CSI2_CLK+`, and a flat one joins a block to a
signal, as in `CAN1_TXD`. Splitting a path on `/` and reading index -1 gives the leaf, whatever the
depth. Splitting a flat name on `_` and reading -1 gives the signal.

### For software engineers

It is Go's `strings.Split`, one row per part.

### Datalog

The last underscore-separated part of each net name:

```
entity(?n, "net"), str.split(?n, "_", -1, ?leaf) => ?n, ?leaf
```

Every part of one name, numbered:

```
entity(?n, "net"), str.split(?n, "_", ?i, ?part), str.prefix(?n, "CAN1_CANH") => ?i, ?part
```

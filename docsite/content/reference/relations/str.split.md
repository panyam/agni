---
title: "str.split"
description: "binds each part of the string between separators with its index from 0; given an index, binds that part alone, a negative index counting from the end (-1 is the last part)"
---

### What it is

`str.split(string, separator, index, part)` binds each part of `string` between separators,
numbered from 0. With `index` bound it binds that part alone. A negative index counts from the end,
so -1 is the last part and -2 the one before it. An index past either end gives no row.

`string` and `separator` must be bound. A string without the separator is one part, itself, and an
empty separator splits between characters. `str.split` is a computed predicate (kind `predicate` in
the catalog), not a stored fact.

### For hardware engineers

A hierarchical net name is a path, such as `/CSI/CAM0.CSI2_CLK+`, and a flat one joins a block to a
signal, as in `CAN1_TXD`. Splitting a path on `/` and reading index -1 gives the leaf, whatever the
depth. Splitting a flat name on `_` and reading -1 gives the signal.

### For software engineers

It is Go's `strings.Split` with each part numbered, plus negative indexing from the end, as in
Python.

### Go projector

`str.split` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it in `Strings` in jaala's `stdlib/strings.go`, and
`core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

The last underscore-separated part of each net name:

```
entity(?n, "net"), str.split(?n, "_", -1, ?leaf) => ?n, ?leaf
```

Every part of one name, numbered:

```
entity(?n, "net"), str.split(?n, "_", ?i, ?part), str.prefix(?n, "CAN1_CANH") => ?i, ?part
```

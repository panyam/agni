---
title: "str.replace"
description: "binds the string with every occurrence of old replaced by new"
---

### What it is

`str.replace(string, old, new, result)` binds `result` as `string` with every occurrence of `old`
replaced by `new`.

`str.replace` comes from jaala's standard library. The [Building
text](https://panyam.github.io/jaala/reference/builtins/#building-text) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

Two instances of one block are usually named alike, `CAN1_TXD` and `CAN2_TXD`. Rewriting one
instance's prefix into the other's names the matching net, so a query can ask whether the second
instance has every net the first one has.

### For software engineers

It is Go's `strings.ReplaceAll` exposed as a function from its first three arguments to its fourth.

### Datalog

Each `CAN1` net beside the name its `CAN2` twin would carry:

```
entity(?n, "net"), str.prefix(?n, "CAN1"), str.replace(?n, "CAN1", "CAN2", ?m) => ?n, ?m
```

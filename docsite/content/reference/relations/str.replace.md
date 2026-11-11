---
title: "str.replace"
description: "binds the string with every occurrence of old replaced by new"
---

### What it is

`str.replace(string, old, new, result)` binds `result` as `string` with every occurrence of `old`
replaced by `new`. It runs forward only, so `string`, `old` and `new` must be bound. With `result`
bound too, it tests rather than binds.

The match is exact and case-sensitive. `str.replace` is a computed predicate (kind `predicate` in
the catalog), not a stored fact.

### For hardware engineers

Two instances of one block are usually named alike, `CAN1_TXD` and `CAN2_TXD`. Rewriting one
instance's prefix into the other's names the matching net, so a query can ask whether the second
instance has every net the first one has.

### For software engineers

It is Go's `strings.ReplaceAll` exposed as a function from its first three arguments to its fourth.

### Go projector

`str.replace` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it in `Strings` in jaala's `stdlib/strings.go`, and
`core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

Each `CAN1` net beside the name its `CAN2` twin would carry:

```
entity(?n, "net"), str.prefix(?n, "CAN1"), str.replace(?n, "CAN1", "CAN2", ?m) => ?n, ?m
```

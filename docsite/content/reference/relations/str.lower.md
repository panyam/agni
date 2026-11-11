---
title: "str.lower"
description: "binds the string in lower case"
---

### What it is

`str.lower(string, lower)` binds `lower` as `string` in lower case. It runs forward only, so `string`
must be bound. With `lower` bound too, it tests rather than binds. `str.lower` is a computed predicate
(kind `predicate` in the catalog), not a stored fact.

### For hardware engineers

Net names are compared exactly, so `can1_txd` and `CAN1_TXD` are two names. Folding both sides to
one case lets a query compare names from two tools, or two revisions, that disagree only in case.

### For software engineers

It is Go's `strings.ToLower`, which folds by Unicode rules rather than ASCII alone.

### Go projector

`str.lower` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it in `Strings` in jaala's `stdlib/strings.go`, and
`core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

Each `CAN1` net beside its name in lower case:

```
entity(?n, "net"), str.prefix(?n, "CAN1"), str.lower(?n, ?l) => ?n, ?l
```

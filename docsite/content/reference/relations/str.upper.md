---
title: "str.upper"
description: "binds the string in upper case"
---

### What it is

`str.upper(string, upper)` binds `upper` as `string` in upper case. It runs forward only, so `string`
must be bound. With `upper` bound too, it tests rather than binds. `str.upper` is a computed predicate
(kind `predicate` in the catalog), not a stored fact.

### For hardware engineers

Net names are compared exactly, so `can1_txd` and `CAN1_TXD` are two names. Folding both sides to
one case lets a query compare names from two tools, or two revisions, that disagree only in case.

### For software engineers

It is Go's `strings.ToUpper`, which folds by Unicode rules rather than ASCII alone.

### Go projector

`str.upper` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it in `Strings` in jaala's `stdlib/strings.go`, and
`core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

A name in upper case, beside the same name capitalized:

```
str.upper("can1_txd", ?u), str.capitalize("can1_txd", ?c) => ?u, ?c
```

---
title: "str.upper"
description: "binds the string in upper case"
---

### What it is

`str.upper(string, upper)` binds `upper` as `string` in upper case.

`str.upper` comes from jaala's standard library. The [Building
text](https://panyam.github.io/jaala/reference/builtins/#building-text) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

Net names are compared exactly, so `can1_txd` and `CAN1_TXD` are two names. Folding both sides to
one case lets a query compare names from two tools, or two revisions, that disagree only in case.

### For software engineers

It is Go's `strings.ToUpper`, which folds by Unicode rules rather than ASCII alone.

### Datalog

A name in upper case, beside the same name capitalized:

```
str.upper("can1_txd", ?u), str.capitalize("can1_txd", ?c) => ?u, ?c
```

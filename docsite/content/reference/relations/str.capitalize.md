---
title: "str.capitalize"
description: "binds the string with its first character in upper case and the rest as written"
---

### What it is

`str.capitalize(string, capitalized)` binds `capitalized` as `string` with its first character in
upper case and the rest left as written. So `can1_txd` becomes `Can1_txd`, and an empty string stays
empty. It runs forward only, so `string` must be bound. With `capitalized` bound too, it tests
rather than binds. `str.capitalize` is a computed predicate (kind `predicate` in the catalog), not a
stored fact.

### For hardware engineers

A query's answer often feeds a report a person reads. A class such as `connector` or a role such
as `rail` reads better there with a leading capital, and the rest of the word stays as written.

### For software engineers

It upper-cases the first rune and copies the rest unchanged. Unlike Python's `str.capitalize` it
does not lower-case the remainder.

### Go projector

`str.capitalize` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it in `Strings` in jaala's `stdlib/strings.go`, and
`core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

A name in upper case, beside the same name capitalized:

```
str.upper("can1_txd", ?u), str.capitalize("can1_txd", ?c) => ?u, ?c
```

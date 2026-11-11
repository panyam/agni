---
title: "list.member"
description: "binds each value of the list with its index from 0; given a value, binds where it is"
---

### What it is

`list.member(list, index, value)` binds one row per member of `list`, with `index` counting from 0
and `value` holding that member. The list must be bound. Calling it with an unbound list is an
evaluation error. `index` and `value` may each be bound or free. With `value` bound it answers where
that value sits, one row per occurrence. With `index` bound it answers the member there.

A value that is not a list gives no row, absent included. `list.member` takes no negative index. Use
`list.at` to count from the end. `list.member` is a computed predicate (kind `predicate` in the
catalog), not a stored fact.

### For hardware engineers

A rule often collects a group, filters the group as a whole, and then needs each member back. One
such question asks for the parts of each class with more than two members. `list.member` unwinds the
group into one row per reference designator, so the next clause can join each part to its nets.

### For software engineers

`list.member` is `for i, x := range xs` as a relation, or SQL's `UNNEST ... WITH ORDINALITY`. Bound
to a value it acts as `slices.Index`, except that a repeated value answers every position.

### Go projector

`list.member` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it, together with `list.length` and `list.at`, in
`ListValues` in jaala's `stdlib/lists.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses.

### Datalog

Every part of each class, with its position in the class's list:

```
parts(?c, list(?r)) :- component.class(?r, ?c);
parts(?c, ?l), list.member(?l, ?i, ?r) => ?c, ?i, ?r
```

The classes that list U2, and where U2 sits in each:

```
parts(?c, list(?r)) :- component.class(?r, ?c);
parts(?c, ?l), list.member(?l, ?i, "U2") => ?c, ?i
```

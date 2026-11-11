---
title: "list.member"
description: "binds each value of the list with its index from 0; given a value, binds where it is"
---

### What it is

`list.member(list, index, value)` binds one row per member of `list`, with `index` counting from 0
and `value` holding that member.

`list.member` comes from jaala's standard library. The
[Lists](https://panyam.github.io/jaala/reference/builtins/#lists) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A rule often collects a group, filters the group as a whole, and then needs each member back. One
such question asks for the parts of each class with more than two members. `list.member` unwinds the
group into one row per reference designator, so the next clause can join each part to its nets.

### For software engineers

`list.member` is `for i, x := range xs` as a relation, or SQL's `UNNEST ... WITH ORDINALITY`. Bound
to a value it acts as `slices.Index`, except that a repeated value answers every position.

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

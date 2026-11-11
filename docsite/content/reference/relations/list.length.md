---
title: "list.length"
description: "binds how many values the list holds"
---

### What it is

`list.length(list, length)` binds `length` to how many values `list` holds.

`list.length` comes from jaala's standard library. The
[Lists](https://panyam.github.io/jaala/reference/builtins/#lists) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A grouped answer often matters for its size. A check wants every part class with more than two
members, or every rail with fewer decoupling capacitors than the design guide asks for.
`list.length` counts a collected group so a later clause can compare the count.

### For software engineers

`list.length` is `len(xs)` over a list value. A list built in one rule can be measured and filtered
in the next, as `HAVING COUNT(*) > 2` filters a SQL group. The list keeps its members, so a later
clause can still unwind it with `list.member`.

### Datalog

The part classes with more than two parts on the board:

```
parts(?c, list(?r)) :- component.class(?r, ?c);
parts(?c, ?l), list.length(?l, ?n), ?n > 2 => ?c, ?n
```

A literal list works too:

```
list.length("[\"a\",\"b\",\"c\"]", ?n) => ?n
```

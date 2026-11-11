---
title: "list.length"
description: "binds how many values the list holds"
---

### What it is

`list.length(list, length)` binds `length` to how many values `list` holds. The list must be bound.
Calling it with an unbound list is an evaluation error. The `list(...)` aggregate writes a list
value as text, such as `["R1","R2","R3"]`, and a query can write the same text as a string. The
empty list `"[]"` has length 0.

A value that is not a list gives no row rather than an error. A number, an absent value and text such
as `"a b"` all fall in that case. `list.length` is a computed predicate (kind `predicate` in the
catalog), not a stored fact.

### For hardware engineers

A grouped answer often matters for its size. A check wants every part class with more than two
members, or every rail with fewer decoupling capacitors than the design guide asks for. `list.length`
counts a collected group so a later clause can compare the count.

### For software engineers

`list.length` is `len(xs)` over a list value. A list built in one rule can be measured and filtered
in the next, as `HAVING COUNT(*) > 2` filters a SQL group. The list keeps its members, so a later
clause can still unwind it with `list.member`.

### Go projector

`list.length` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it, together with `list.member` and `list.at`, in
`ListValues` in jaala's `stdlib/lists.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses.

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

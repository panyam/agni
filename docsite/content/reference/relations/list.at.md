---
title: "list.at"
description: "binds the value at an index from 0, a negative one counting from the end (-1 is the last)"
---

### What it is

`list.at(list, index, value)` binds `value` to the member of `list` at `index`. Indexes count from
0. A negative index counts from the end, so -1 is the last member and -2 the one before it. Both the
list and the index must be bound. Calling it with either unbound is an evaluation error.

An index past either end gives no row. So does an index that is not a whole number, and so does a
value that is not a list, absent included. `list.at` is a computed predicate (kind `predicate` in the
catalog), not a stored fact.

### For hardware engineers

A path through a board is a list of nets in walk order. Its first member is where the walk started
and its last is where it ended. `list.at` reads one end without listing the rest. The same works on
a group of reference designators, collected in order.

### For software engineers

`list.at` is Python's `xs[i]`, negative indexes included, with an out-of-range index answering no row
instead of raising. To scan every position use `list.member`, which binds the index as well.

### Go projector

`list.at` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with `list.member` and `list.length`, in
`ListValues` in jaala's `stdlib/lists.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses.

### Datalog

The first and last reference designator of each part class:

```
parts(?c, list(?r)) :- component.class(?r, ?c);
parts(?c, ?l), list.at(?l, 0, ?first), list.at(?l, -1, ?last) => ?c, ?first, ?last
```

The last member of a literal list:

```
list.at("[\"a\",\"b\",\"c\"]", -1, ?x) => ?x
```

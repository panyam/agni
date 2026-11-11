## list.at

### What it is

`list.at(list, index, value)` binds `value` to the member of `list` at `index`.

`list.at` comes from jaala's standard library. The
[Lists](https://panyam.github.io/jaala/reference/builtins/#lists) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A path through a board is a list of nets in walk order. Its first member is where the walk started
and its last is where it ended. `list.at` reads one end without listing the rest. The same works on
a group of reference designators, collected in order.

### For software engineers

`list.at` is Python's `xs[i]`. To scan every position use `list.member`, which binds the index as
well.

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

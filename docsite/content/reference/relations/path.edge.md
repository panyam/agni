---
title: "path.edge"
description: "binds each edge of a path over a relation of three arguments, index from 0"
---

### What it is

`path.edge(path, index, edge)` binds one row per edge of `path`, with `index` counting from 0 and
`edge` holding the edge's name.

`graph.paths` binds the path value these functions read. The `path` column of `net.route` is display
text, not a path value, so no `path.*` function reads it.

`path.edge` comes from jaala's standard library. The
[Paths](https://panyam.github.io/jaala/reference/builtins/#paths) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

The edges of a path through a board are the parts it crossed. A path from the input to a rail names
each regulator on the way. `path.edge` turns that into one row per reference designator, so a query
can ask which rails depend on one regulator.

### For software engineers

`path.edge` is `for i, e := range path.Edges`. Outside the `shortest` mode, two parallel edges
between the same nodes make two paths, and the edge names tell those paths apart.

### Datalog

These examples walk a `hop` relation of the examples' own. It steps from one net to another through
any part on both, names that part as the edge, and leaves ground out. Each part crossed from the 12
V input to the 1.8 V rail:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", "PMIC_IO_1V8", "shortest", 1, 0, ?p),
  path.edge(?p, ?i, ?ref) => ?i, ?ref
```

Every net whose path from the 12 V input crosses U1:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", ?to, "shortest", 1, 0, ?p),
  path.edge(?p, _, "U1") => ?to
```

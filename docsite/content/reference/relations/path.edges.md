---
title: "path.edges"
description: "binds the path's edges as a list"
---

### What it is

`path.edges(path, edges)` binds `edges` to the edges of `path` as one list value, in walk order.

`graph.paths` binds the path value these functions read. The `path` column of `net.route` is display
text, not a path value, so no `path.*` function reads it.

`path.edges` comes from jaala's standard library. The
[Paths](https://panyam.github.io/jaala/reference/builtins/#paths) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

The parts on a path are the evidence for a connection. A row that lists `["U1","U2"]` beside a rail
says that both regulators sit between the input and that rail. `path.edges` puts the parts in one
cell, so a reviewer reads them without opening the schematic.

### For software engineers

`path.edges` is the `Edges` slice of a path as a value. It is the list form of `path.edge`, and
`list.length` and `list.at` read it.

### Datalog

These examples walk a `hop` relation of the examples' own. It steps from one net to another through
any part on both, names that part as the edge, and leaves ground out. The parts on the shortest path
from the 12 V input to every net it reaches:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", ?to, "shortest", 1, 0, ?p),
  path.edges(?p, ?e) => ?to, ?e
```

The first part on the path to the 1.8 V rail:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", "PMIC_IO_1V8", "shortest", 1, 0, ?p),
  path.edges(?p, ?e), list.at(?e, 0, ?first) => ?first
```

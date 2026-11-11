---
title: "path.edge"
description: "binds each edge of a path over a relation of three arguments, index from 0"
---

### What it is

`path.edge(path, index, edge)` binds one row per edge of `path`, with `index` counting from 0 and
`edge` holding the edge's name. The path must be bound. Calling it with an unbound path is an
evaluation error. With `index` bound it answers that one edge, and with `edge` bound it tests
whether the path crosses it.

Only a path over a relation of three arguments names its edges. A path over a relation of two
arguments gives no row, and so does a value that is not a path, absent included. `path.edge` is a
computed predicate (kind `predicate` in the catalog), not a stored fact.

`graph.paths` binds a path value as text. Over a relation of two arguments the path lists its
nodes, as in `["a","b","c"]`. Over a relation of three arguments, from, edge and to, it names the
edges too, as in `{"nodes":["a","b"],"edges":["R1"]}`. The `path` column of `net.route` is display
text, not a path value, and every `path.*` function gives no row on it.

### For hardware engineers

The edges of a path through a board are the parts it crossed. A path from the input to a rail
names each regulator on the way. `path.edge` turns that into one row per reference designator, so a
query can ask which rails depend on one regulator.

### For software engineers

`path.edge` is `for i, e := range path.Edges`. Outside the `shortest` mode, two parallel
edges between the same nodes make two paths, and the edge names tell those paths apart.

### Go projector

`path.edge` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with the other `path.*` functions, in
`PathValues` in jaala's `stdlib/paths.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses.

### Datalog

These examples walk a `hop` relation of the examples' own. It steps from one net to another through
any part on both, names that part as the edge, and leaves ground out. Each part crossed from the 12 V
input to the 1.8 V rail:

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

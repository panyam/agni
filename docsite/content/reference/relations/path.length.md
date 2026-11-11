---
title: "path.length"
description: "binds how many steps the path takes"
---

### What it is

`path.length(path, steps)` binds `steps` to how many steps `path` takes. A path through three nodes
takes two steps. The path must be bound. Calling it with an unbound path is an evaluation error. A
value that is not a path gives no row, absent included. `path.length` is a computed predicate (kind
`predicate` in the catalog), not a stored fact.

`graph.paths` binds a path value as text. Over a relation of two arguments the path lists its
nodes, as in `["a","b","c"]`. Over a relation of three arguments, from, edge and to, it names the
edges too, as in `{"nodes":["a","b"],"edges":["R1"]}`. The `path` column of `net.route` is display
text, not a path value, and every `path.*` function gives no row on it.

### For hardware engineers

The number of parts between two nets says how indirect a connection is. A rail one regulator away
from the input is a direct child. A rail two regulators away is cascaded, and its sequencing and
efficiency depend on the stage in between. `path.length` counts those stages.

### For software engineers

`path.length` is the edge count of a graph path, one less than its node count. Filter on it to keep
only the paths of a given depth, as a BFS keeps its frontier by level.

### Go projector

`path.length` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it, together with the other `path.*` functions, in
`PathValues` in jaala's `stdlib/paths.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses.

### Datalog

These examples walk a `hop` relation of the examples' own. It steps from one net to another through
any part on both, and it leaves ground out. The steps from the 12 V input to the 1.8 V rail:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", "PMIC_IO_1V8", "shortest", 1, 0, ?p),
  path.length(?p, ?n) => ?n
```

Every net two or more parts away from the 12 V input:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", ?to, "shortest", 1, 0, ?p),
  path.length(?p, ?n), ?n >= 2 => ?to, ?n
```

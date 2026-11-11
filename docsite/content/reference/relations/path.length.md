---
title: "path.length"
description: "binds how many steps the path takes"
---

### What it is

`path.length(path, steps)` binds `steps` to how many steps `path` takes.

`graph.paths` binds the path value these functions read. The `path` column of `net.route` is display
text, not a path value, so no `path.*` function reads it.

`path.length` comes from jaala's standard library. The
[Paths](https://panyam.github.io/jaala/reference/builtins/#paths) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

The number of parts between two nets says how indirect a connection is. A rail one regulator away
from the input is a direct child. A rail two regulators away is cascaded, and its sequencing and
efficiency depend on the stage in between. `path.length` counts those stages.

### For software engineers

`path.length` is the edge count of a graph path, one less than its node count. Filter on it to keep
only the paths of a given depth, as a BFS keeps its frontier by level.

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

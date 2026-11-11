---
title: "path.node"
description: "binds each node of the path, index from 0"
---

### What it is

`path.node(path, index, node)` binds one row per node of `path`, with `index` counting from 0 and
`node` holding that node.

`graph.paths` binds the path value these functions read. The `path` column of `net.route` is display
text, not a path value, so no `path.*` function reads it.

`path.node` comes from jaala's standard library. The
[Paths](https://panyam.github.io/jaala/reference/builtins/#paths) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A power tree question often asks which rail a path passes through on the way down. The 1.8 V rail
fed from the 3.3 V rail depends on the 3.3 V regulator coming up first. `path.node` names each net
in walk order, so a query can find the intermediate rail.

### For software engineers

`path.node` is `for i, n := range path.Nodes` as a relation. With the node bound it is a membership
test, and `not path.node(?p, _, "X")` keeps the paths that avoid X.

### Datalog

These examples walk a `hop` relation of the examples' own. It steps from one net to another through
any part on both, and it leaves ground out. Each net on the way from the 12 V input to the 1.8 V
rail, in order:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", "PMIC_IO_1V8", "shortest", 1, 0, ?p),
  path.node(?p, ?i, ?net) => ?i, ?net
```

The nets whose path from the 12 V input passes through the 3.3 V rail:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", ?to, "shortest", 1, 0, ?p),
  path.node(?p, _, "PMIC_CORE_3V3"), ?to != "PMIC_CORE_3V3" => ?to
```

## path.node

### What it is

`path.node(path, index, node)` binds one row per node of `path`, with `index` counting from 0 and
`node` holding that node. The path must be bound. Calling it with an unbound path is an evaluation
error. `index` and `node` may each be bound or free. With `index` bound it answers the node at that
position, and with `node` bound it tests whether the path passes through it.

`path.node` takes no negative index. An index of -1 gives no row. To read the last node, take
`path.nodes` and `list.at` with -1. A value that is not a path gives no row, absent included.
`path.node` is a computed predicate (kind `predicate` in the catalog), not a stored fact.

`graph.paths` binds a path value as text. Over a relation of two arguments the path lists its
nodes, as in `["a","b","c"]`. Over a relation of three arguments, from, edge and to, it names the
edges too, as in `{"nodes":["a","b"],"edges":["R1"]}`. The `path` column of `net.route` is display
text, not a path value, and every `path.*` function gives no row on it.

### For hardware engineers

A power tree question often asks which rail a path passes through on the way down. The 1.8 V rail
fed from the 3.3 V rail depends on the 3.3 V regulator coming up first. `path.node` names each net
in walk order, so a query can find the intermediate rail.

### For software engineers

`path.node` is `for i, n := range path.Nodes` as a relation. With the node bound it is a membership
test, and `not path.node(?p, _, "X")` keeps the paths that avoid X.

### Go projector

`path.node` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with the other `path.*` functions, in
`PathValues` in jaala's `stdlib/paths.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses.

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

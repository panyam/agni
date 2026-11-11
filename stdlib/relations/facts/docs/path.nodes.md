## path.nodes

### What it is

`path.nodes(path, nodes)` binds `nodes` to the nodes of `path` as one list value, in walk order. The
path must be bound. Calling it with an unbound path is an evaluation error. A value that is not a
path gives no row, absent included. The list works with every `list.*` function. `path.nodes` is a
computed predicate (kind `predicate` in the catalog), not a stored fact.

`graph.paths` binds a path value as text. Over a relation of two arguments the path lists its
nodes, as in `["a","b","c"]`. Over a relation of three arguments, from, edge and to, it names the
edges too, as in `{"nodes":["a","b"],"edges":["R1"]}`. The `path` column of `net.route` is display
text, not a path value, and every `path.*` function gives no row on it.

### For hardware engineers

A review wants the whole chain from the input to a rail in one cell, not one row per net.
`path.nodes` gives that chain as a list, and `list.at` reads either end of it.

### For software engineers

`path.nodes` is the `Nodes` slice of a path as a value. Once it is a list, `list.length`,
`list.at` and `list.member` read it, and `list.at` accepts a negative index where `path.node` does
not.

### Go projector

`path.nodes` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with the other `path.*` functions, in
`PathValues` in jaala's `stdlib/paths.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses.

### Datalog

These examples walk a `hop` relation of the examples' own. It steps from one net to another through
any part on both, and it leaves ground out. The nets and parts from the 12 V input to the 1.8 V rail,
as two lists:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", "PMIC_IO_1V8", "shortest", 1, 0, ?p),
  path.nodes(?p, ?l), path.edges(?p, ?e) => ?l, ?e
```

The last net of the same path, and how many nets it holds:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", "PMIC_IO_1V8", "shortest", 1, 0, ?p),
  path.nodes(?p, ?l), list.at(?l, -1, ?last), list.length(?l, ?n) => ?last, ?n
```

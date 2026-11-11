## path.step

### What it is

`path.step(path, index, from, to)` binds one row per step of `path`. `index` counts from 0, and
`from` and `to` are the two nodes the step joins, in walk order. The path must be bound. Calling it
with an unbound path is an evaluation error. With `index` bound it answers that one step, and an
index outside the path gives no row. A value that is not a path gives no row, absent included.
`path.step` is a computed predicate (kind `predicate` in the catalog), not a stored fact.

`graph.paths` binds a path value as text. Over a relation of two arguments the path lists its
nodes, as in `["a","b","c"]`. Over a relation of three arguments, from, edge and to, it names the
edges too, as in `{"nodes":["a","b"],"edges":["R1"]}`. The `path` column of `net.route` is display
text, not a path value, and every `path.*` function gives no row on it.

### For hardware engineers

A power tree is a chain of conversions, each from one rail to the next. `path.step` lists those
conversions as pairs, so a query can join each pair to the regulator between them or compare the two
voltages.

### For software engineers

`path.step` is the pairwise window over a path's nodes, `zip(nodes, nodes[1:])` with the position.
It gives the adjacent pairs without a self-join on `path.node`.

### Go projector

`path.step` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with the other `path.*` functions, in
`PathValues` in jaala's `stdlib/paths.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses.

### Datalog

These examples walk a `hop` relation of the examples' own. It steps from one net to another through
any part on both, and it leaves ground out. Each step from the 12 V input to the 1.8 V rail:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", "PMIC_IO_1V8", "shortest", 1, 0, ?p),
  path.step(?p, ?i, ?from, ?to) => ?i, ?from, ?to
```

Each step with the part that makes it, joined on the index:

```
hop(?a, ?r, ?b) :- component.net(?r, ?a), component.net(?r, ?b), ?a != ?b,
  not net.ground(?a), not net.ground(?b);
graph.paths("hop", "PMIC_MAIN_12V0", "PMIC_IO_1V8", "shortest", 1, 0, ?p),
  path.step(?p, ?i, ?from, ?to), path.edge(?p, ?i, ?ref) => ?i, ?from, ?ref, ?to
```

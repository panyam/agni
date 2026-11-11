## path.nodes

### What it is

`path.nodes(path, nodes)` binds `nodes` to the nodes of `path` as one list value, in walk order.

`graph.paths` binds the path value these functions read. The `path` column of `net.route` is display
text, not a path value, so no `path.*` function reads it.

`path.nodes` comes from jaala's standard library. The
[Paths](https://panyam.github.io/jaala/reference/builtins/#paths) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A review wants the whole chain from the input to a rail in one cell, not one row per net.
`path.nodes` gives that chain as a list, and `list.at` reads either end of it.

### For software engineers

`path.nodes` is the `Nodes` slice of a path as a value. Once it is a list, `list.length`, `list.at`
and `list.member` read it.

### Datalog

These examples walk a `hop` relation of the examples' own. It steps from one net to another through
any part on both, and it leaves ground out. The nets and parts from the 12 V input to the 1.8 V
rail, as two lists:

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

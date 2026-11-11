## path.step

### What it is

`path.step(path, index, from, to)` binds one row per step of `path`.

`graph.paths` binds the path value these functions read. The `path` column of `net.route` is display
text, not a path value, so no `path.*` function reads it.

`path.step` comes from jaala's standard library. The
[Paths](https://panyam.github.io/jaala/reference/builtins/#paths) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A power tree is a chain of conversions, each from one rail to the next. `path.step` lists those
conversions as pairs, so a query can join each pair to the regulator between them or compare the two
voltages.

### For software engineers

`path.step` is the pairwise window over a path's nodes, `zip(nodes, nodes[1:])` with the position.
It gives the adjacent pairs without a self-join on `path.node`.

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

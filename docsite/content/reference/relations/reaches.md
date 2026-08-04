---
title: "reaches"
description: "transitive reachability through series pass elements (R/L/ferrite/fuse)"
---

### What it is

`reaches(from, net)` is true when `net` is reachable from `from` by walking THROUGH series pass
elements: resistors, inductors, ferrite beads, and fuses. It is the transitive-closure predicate
the protection rules use to answer "is there a component of class X somewhere on the path between
these two nets?" without hard-coding a topology.

Unlike the fact relations, `reaches` is not a stored projection. It is computed on demand from the
design graph, so it is a datalog *predicate* (kind `predicate` in the catalog), the recursive
counterpart to `net.bus_like`: `net.bus_like` names the nets the walk refuses to enter, and
`reaches` is the walk itself.

### For hardware engineers

The walk crosses a two-terminal series part (a resistor, bead, or fuse joins exactly two nets, so
crossing it is following the signal one hop along its path) and stops at anything that is not a
point-to-point series node: a capacitor (a DC block, the signal does not continue through it), a
ground plane or global rail or any rail-scale fan-out (a distribution node, not a path), and any
part with more than two nets (a transceiver or connector, not a series element). This is how an
ESD or input-protection review asks "does a clamp sit anywhere between this connector pin and the
device pin?" while a resistor or bead in the middle of the path does not break the question.

### For software engineers

`reaches` is transitive reachability over a filtered graph: the nodes are nets, an edge exists only
through a two-net pass element, and `net.bus_like` nets are excluded so the traversal cannot leak
into a global singleton and mark the whole design reachable. It is a bounded BFS (a hop cap guards
pathological depth; fan-out and finiteness bound it anyway), so a query over it terminates.

### Go projector

`reaches` has no `check/facts.go` projector because it is not a stored fact. The query engine
evaluates it as a built-in in `query/preds.go` (bounded by `reachHops`), delegating to
`check.Model.Reach` in `check/reach.go`, the same bounded walk the protection rules use, and the
same `isBusLike` stop predicate that `net.bus_like` exposes. Because it is computed, not projected,
it is outside the per-relation EDB doc requirement and is documented here as the reference behind
the walk.

### Datalog

Every net reachable from a starting net, through series parts:

```
reaches("VBUS_IN", ?net) => ?net
```

The components that sit on those reachable nets (what a protection walk would find):

```
reaches("VBUS_IN", ?net), component-on-net(?r, ?net) => ?r
```

### Schematic

![The walk crosses two-pin series parts (R, ferrite, fuse) and stops at a DC-blocking cap or a bus-like net]({{.Site.PathPrefix}}/static/images/catalog/relations/reaches.svg)

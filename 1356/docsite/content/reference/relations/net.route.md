---
title: "net.route"
description: "the same walk as `net.reaches`, with the route it found bound as a readable value (`VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3`), so a connectivity answer carries the evidence for itself; one route per pair, and a route never ends on a rail because the walk refuses one"
---

### What it is

`net.route(from, net, path)` makes the same walk as `net.reaches`, and binds the route it found as a
value instead of discarding it. It holds for a pair exactly when `net.reaches(from, net)` holds, and
`path` binds a readable rendering of what the walk crossed on the way:

```
VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3
```

The names outside the brackets are nets, in crossing order. The name inside each bracket is the
series part the walk passed through between them. A net reaches itself at distance zero, so its
route is its own name.

The engine could answer whether two points were connected long before it could show how, which meant
a reviewer had no way to check the answer (agni issue 518). This relation is the query-side half of
the fix. A connectivity answer now carries its own evidence, so a reviewer can read a hundred rows
rather than re-ask them one at a time.

### For hardware engineers

A resistor, ferrite or fuse in the middle of a signal splits the net without breaking the
connection, so "is this pin joined to that one" is a question about a path across whatever parts sit
in the way. `net.reaches` answers it. This says which parts those were.

Usually you wanted the parts. "VDD_3V3 is reachable from VBUS" is a fact, and you then go and look
it up in the schematic. "VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3" is the same fact with the series
bead and the sense resistor named. A reader can decide from it whether that path is the intended one
without opening anything.

The walk deliberately leaves out two things. **A route never ends on a rail or a plane.** The walk
refuses a bus-like net outright, because a net every part on the board touches joins everything to
everything and proves nothing. **A capacitor is never crossed**, because it is a DC block rather
than a pass element, even though it sits on the net and appears in every connection list.

`agni trace` is the pin-to-pin form. It does end on a rail, and it reports the test points on each
net of the route. This relation is net-to-net, because a query binds nets.

### For software engineers

`net.route` and `net.reaches` generate over the same filtered graph. Its nodes are nets, an edge
exists only through a two-net pass element, and bus-like nets are excluded so the traversal cannot
leak into a global singleton. `path` is a projection of the BFS tree, rendered.

**`net.route` gives one route per pair, not every route.** The walk is a breadth-first search and the
path is its tree path, so where two resistors bridge the same two nets the answer names one of them and says nothing
about the other. It is *a* route and the shortest one, never an enumeration.

`path` is a string, so every query column is still scalar and a route survives into a csv cell, a
markdown table and a rule's finding without anything downstream learning a new type. Binding one and
comparing two of them is legal and meaningless. The same is true of the net names in the first two
arguments.

**A query cannot tell "no route" from "no such net."** Both are zero rows. That is ordinary datalog,
and it is the right semantics for a rule, since a rule asking about `VBUS` on a board that has none
should say nothing rather than fail. When you need that distinction, `agni trace` keeps the three
outcomes apart and exits non-zero on an endpoint that names nothing.

### Go projector

None. Like `net.reaches`, this is computed on demand from the design graph rather than stored, so it
is a datalog *predicate* (kind `predicate` in the catalog) rather than an EDB relation with a
projector in `stdlib/relations/facts.go`. Its generator `genRoute` (`core/query/preds.go`) drives
`check.Model.Reach` and renders each answer with `Reach.RouteLine`. `RouteLine` is the fourth
reading of one walk, beside `PathTo`, `ThroughOnPath` and `StepsTo`. `genReaches` and `genRoute`
share `walk`, so the two cannot drift about what is connected.

An empty result means no series path within the walk's radius, or a `from` this design does not
have. See the note above about which.

### Datalog

Everything a rail reaches, with the route to each:

```
net.route("VBUS", ?net, ?path) => ?net, ?path
```

Where a signal ends up and what stands in the way, as a document to save:

```
net.route("SPI_CS", ?net, ?path) => ?net, ?path
```

The route to every net carrying a test point, with the test point named. The
path column lets a reader check the answer without opening the schematic:

```
net.route(?from, ?net, ?path), component.net(?ref, ?net), component.class(?ref, "test_point")
  => ?from, ?net, ?ref, ?path
```

An unbound `from` walks from every net on the board. agni's evaluator plans each rule body, so a
walk starts from whatever binds `from` first, whichever order the body is written in. Before the
planner existed, one shipped rule opened with the walk, and it took `agni check` from thirteen
seconds to not finishing at all (WS3-114). When nothing in the body binds `from`, the query asked
for the full walk, and on a large board it costs accordingly. Each walk charges a query's work budget
for the connections it examines as well as for the rows it emits, so a budgeted full walk stops
partway through instead of after the last start net.

### Where this is going

`net.reaches` and `net.route` are the same walk asked twice because a path
question still has no way to state its own radius or its own edge class, so each caller hand-codes
one. Issue 374 designs a topology-pattern surface where the radius is a quantifier and the edge
class is a character class, and where a match carries the path it found as a matter of course. If
that ships, both of these become canned patterns over it.

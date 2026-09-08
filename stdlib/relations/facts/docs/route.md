## route

### What it is

`route(from, net, path)` is the walk `reaches` makes, with the route it found bound as a value
instead of discarded. It holds for the same pairs `reaches(from, net)` holds for, and `path` binds a
readable rendering of what the walk crossed to get there:

```
VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3
```

The names outside the brackets are nets, in crossing order. The name inside each bracket is the
series part the walk passed through between them. A net reaches itself at distance zero, so its
route is its own name.

The engine could answer whether two points were connected long before it could show how, which meant
a reviewer had no way to check the answer (agni issue 518). This is the query-side half of that: a
connectivity answer that carries its own evidence, so a hundred rows can be read rather than
re-asked one at a time.

### For hardware engineers

A resistor, ferrite or fuse in the middle of a signal splits the net without breaking the
connection, so "is this pin joined to that one" is a question about a path across whatever parts sit
in the way. `reaches` answers it. This says which parts those were.

That is usually the part you actually wanted. "VDD_3V3 is reachable from VBUS" is a fact you then go
and look up in the schematic; "VBUS -> [R5] -> VBUS_F -> [L1] -> VDD_3V3" is the same fact with the series
bead and the sense resistor named, which is enough to decide whether that path is the intended one
without opening anything.

Two things it deliberately does not do. **A route never ends on a rail or a plane.** The walk refuses
a bus-like net outright, because a net every part on the board touches joins everything to everything
and proves nothing. **A capacitor is never crossed**, because it is a DC block rather than a pass
element, even though it sits on the net and appears in every connection list.

For the pin-to-pin form, which does end on a rail and reports the test points sitting on each net of
the route, use `agni trace` instead. This relation is net-to-net, because that is what a query binds.

### For software engineers

A generator over the same filtered graph `reaches` walks: nodes are nets, an edge exists only
through a two-net pass element, and bus-like nets are excluded so the traversal cannot leak into a
global singleton. `path` is a projection of the BFS tree, rendered.

**One route per pair, not every route.** The walk is a breadth-first search and the path is its tree
path, so where two resistors bridge the same two nets the answer names one of them and says nothing
about the other. It is *a* route and the shortest one, never an enumeration.

`path` is a string, so every query column stays scalar and a route survives into a csv cell, a
markdown table and a rule's finding without anything downstream learning a new type. Binding one and
comparing two of them is legal and meaningless, which is equally true of the net names in the first
two arguments.

**A query cannot tell "no route" from "no such net."** Both are zero rows, which is ordinary datalog
and is the right semantics for a rule (a rule asking about `VBUS` on a board that has none should
stay silent, not fail). When that distinction is the thing you need, `agni trace` keeps the three
outcomes apart and exits non-zero on an endpoint that names nothing.

### Go projector

None. Like `reaches`, this is computed on demand from the design graph rather than stored, so it is
a datalog *predicate* (kind `predicate` in the catalog) rather than an EDB relation with a projector
in `stdlib/relations/facts.go`. The evaluator's `extendRoute` (`core/query/preds.go`) drives
`check.Model.Reach` and renders each answer with `Reach.RouteLine`, which is the fourth reading of
one walk beside `PathTo`, `ThroughOnPath` and `StepsTo`. `extendReaches` and `extendRoute` share
`extendWalk`, so the two cannot drift about what is connected.

An empty result means no series path within the walk's radius, or a `from` this design does not
have. See the note above about which.

### Datalog

Everything a rail reaches, with the route to each:

```
route("VBUS", ?net, ?path) => ?net, ?path
```

Where a signal ends up and what stands in the way, as a document to save:

```
route("SPI_CS", ?net, ?path) => ?net, ?path
```

The route to every net that reaches a regulator's output, joined to the parts sitting there. The
path column is what makes the answer checkable without opening the schematic:

```
route(?from, ?net, ?path), component-on-net(?ref, ?net), component.class(?ref, "test_point")
  => ?from, ?net, ?ref, ?path
```

Lead with a bound or constant `from` wherever you can. An unbound first argument walks from every
net on the board, which is the shape `GeneratorFirstRules` reports and which took one shipped rule
from thirteen seconds to not finishing at all.

### Where this is going

`reaches` and `route` are the same walk asked twice, which is a symptom rather than a design: a path
question still has no way to state its own radius or its own edge class, so each caller hand-codes
one. Issue 374 designs a topology-pattern surface where the radius is a quantifier and the edge
class is a character class, and where a match carries the path it found as a matter of course. If
that lands, both of these become canned patterns over it.

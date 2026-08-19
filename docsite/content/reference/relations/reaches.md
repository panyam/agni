---
title: "reaches"
description: "transitive reachability through series pass elements (R/L/ferrite/fuse); the optional third argument binds the EXACT number of crossings, so a radius is written `reaches(?a,?b,?h), ?h <= 2` and not `reaches(?a,?b,2)`, which means exactly two"
---

### What it is

`reaches(from, net)` is true when `net` is reachable from `from` by walking THROUGH series pass
elements: resistors, inductors, ferrite beads, and fuses. It is the transitive-closure predicate
the protection rules use to answer "is there a component of class X somewhere on the path between
these two nets?" without hard-coding a topology.

`reaches(from, net, hops)` is the same walk with the distance exposed, so a rule states the radius
its own question needs. Read the next section before using it: `hops` is an exact count, not a
budget.

Unlike the fact relations, `reaches` is computed on demand from the design graph rather than stored, so it is a datalog *predicate* (kind `predicate` in the catalog), the recursive
counterpart to `net.bus_like`: `net.bus_like` names the nets the walk refuses to enter, and
`reaches` is the walk itself.

### Distance, and the trap in it

`hops` binds the ACTUAL number of series crossings, reflexive at 0 (a net reaches itself). Because a
datalog argument binds by equality, putting a bare number in that slot means *exactly* that distance:

```
reaches(?n, ?rn, 2)           # exactly 2 crossings — SKIPS a part sitting 1 away
reaches(?n, ?rn, ?h), ?h <= 2 # within 2 crossings — what a protection question means
```

The first line is the spelling most people reach for and it is almost never what they want. Use the
comparison form for a radius.

Why the radius belongs in the rule at all: the engine holds more than one of them, deliberately. The
query built-in searches the whole neighborhood (`topologyReachHops`), because a topology question
like "what is connected to what through passives" wants distant answers. The protection guards ask at
`check.ProtectionReachHops` (2), and the power-entry walk at `check.PowerPathReachHops` (3), because
"is a clamp electrically adjacent to this pin" wants only near ones. A discharge pushes through every
series element before the clamp conducts, so a TVS six resistors away protects what is downstream of
itself, not the pin. Composing a protection predicate out of the wide default would credit that
distant TVS and report a genuinely unprotected pin as clean, which is a false pass rather than a
missing result.

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

`reaches` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The query
engine evaluates it as a built-in in `core/query/preds.go` (bounded by `topologyReachHops`),
delegating to `check.Model.Reach` in `core/check/reach.go` and the same `IsBusLike` stop predicate
that `net.bus_like` exposes. It is the same walk the protection rules run, at a wider radius (see
Distance above): same traversal, different question. The distance the third argument binds is
`Reach.Depth`, recorded by the BFS as it goes rather than re-derived from the `Parent` chain, which
can disagree where parallel passes bridge the same two nets. Because it is computed, not projected,
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

The same question at a protection radius, the way a rule scoped like `esd-protection` asks it,
over a TVS within two series crossings of the net:

```
reaches(?n, ?rn, ?h), ?h <= 2, component-on-net(?t, ?rn), component.class(?t, "tvs") => ?n, ?t
```

How far away each reachable net is, the query to run when a radius is not behaving as
expected:

```
reaches("VBUS_IN", ?net, ?hops) => ?net, ?hops
```

### Schematic

![The walk crosses two-pin series parts (R, ferrite, fuse) and stops at a DC-blocking cap or a bus-like net]({{.Site.PathPrefix}}/static/images/catalog/relations/reaches.svg)

### Where this is going

The hop-radius trap above, the per-caller radius constants, and the baked-in edge class are all
symptoms of the same thing: a path question has no way to state itself, so each caller hand-codes a
walk. Issue 374 designs a topology-pattern surface where the radius is a quantifier, the edge class
is a character class, and a match carries the path it found. If that lands, `reaches` becomes one
canned pattern and this page becomes the migration note.

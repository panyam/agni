## entity

### What it is

`entity(name, kind)` yields one row per named thing in the design: every component by its reference
designator, every net by its name, every unmodeled bus by its label. `kind` is `component`, `net` or
`bus`. A finding's subject and a picked entity on the canvas carry the same vocabulary.

It is the only relation whose range is EXISTENCE rather than a relationship. Every other one ranges
over an association: a component on a net, a pin's role, a rail's declared voltage.
Before this relation, some questions had to borrow another relation's range, such as "what is in
this design" or "what is called something like this". They quietly inherited that relation's blind
spots.

An unnamed net and an anonymous bus wire emit nothing. A row with an empty name could never match a
name search and would answer "" to a question about what things are called. The thing has no name,
so leaving the row out is the correct answer rather than a gap.

Pins are absent by design. A pin's identity is two fields, so it cannot be one `name` without
inventing a composite string, and nothing else in the fact base would join against that string.
`component.pin(ref_des, pin)` already enumerates them.

### Absence is not a pass

An empty result means the design declared nothing of that kind under that name, not that the thing is
absent from the board. If the reader could not resolve a part, the part still exists in the IR and
answers here. If a part sits on a sheet the read never opened, it does not. When a design is read
sheet by sheet, keep `net.external(?net)` in mind as you scope a name search.

### For hardware engineers

This is the index at the front of the drawing set, listing everything this design names and
what sort of thing each one is rather than what is connected to what. You reach for it when you know
part of a name and want to find the thing: every net with `CAN` in it, every reference designator
starting with `U`, every bus label the reader saw.

Searching through `component.net` looks equivalent and is not, so this is a relation of its own. A
connection relation has no row for a part with no connections, a net with nothing on it, or a bus
that was detected but never expanded. Those are exactly the things worth finding during a review,
since an unconnected part is usually either a mounting hole or a mistake.

### For software engineers

It is an enumeration over the design graph's nodes, where every other netlist relation is an enumeration
over its edges or its node attributes. A search that walks edges to find a node misses every
isolated node, and this relation closes that bug.

Rows are 1:1 with entities and carry no ordering guarantee beyond the IR's own. The relation joins to
anything keyed by the same identifier. `entity(?n, "net"), net.pin_count(?n, ?c)` counts fan-out for
every net, including the nets with nothing on them. The same query through `component.net` would
silently drop those nets.

### Go projector

`entityFacts` in `stdlib/relations/facts.go` iterates `Model.Components()`, `Model.Nets()` and
`Model.UnmodeledBuses()`, emitting one row each with the entity's own IR site as the citation. It
reads no datasheet and no board data, so it answers on a bare netlist the same way it answers on a
full read.

### Datalog

Everything the design names, with its kind:

```
entity(?name, ?kind) => ?name, ?kind
```

Find things by a fragment of their name, as a search box does:

```
entity(?name, ?kind), str.contains(?name, "CAN") => ?name, ?kind
```

Reference designators under a prefix, or a glob over net names:

```
entity(?name, "component"), str.prefix(?name, "U") => ?name
entity(?name, "net"), str.glob(?name, "*_CLK") => ?name
```

The isolated cases a connection relation cannot reach. Parts on no net at all:

```
entity(?ref, "component"), not component.net(?ref, ?any) => ?ref
```

Nets with nothing on them:

```
entity(?net, "net"), not component.net(?any, ?net) => ?net
```

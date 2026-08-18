## entity

### What it is

`entity(name, kind)` yields one row per named thing in the design: every component by its reference
designator, every net by its name, every unmodeled bus by its label. `kind` is `component`, `net` or
`bus`, the same vocabulary a finding's subject and a picked entity on the canvas carry.

It is the only relation whose range is EXISTENCE rather than a relationship. Every other one ranges
over an association: a component on a net, a pin's role, a rail's declared voltage. That difference is
the whole point. Before this relation, a question like "what is in this design" or "what is called
something like this" had to borrow another relation's range, and quietly inherited that relation's
blind spots.

An unnamed net and an anonymous bus wire emit nothing. A row with an empty name could never match a
name search and would answer "" to a question about what things are called, so its absence is the
honest report rather than a gap.

Pins are absent by design. A pin's identity is two fields, so it cannot be one `name` without
inventing a composite string nothing else in the fact base would join against. `pin(ref_des, pin)`
already enumerates them.

### Absence is not a pass

An empty result means the design declared nothing of that kind under that name, not that the thing is
absent from the board. A part the reader could not resolve still exists in the IR and answers here;
a part on a sheet the read never opened does not. Scope a name search with `net.external(?net)` in
mind when a design is read sheet by sheet.

### For hardware engineers

This is the index at the front of the drawing set. Not "what is connected to what", just "here is
everything this design names, and what sort of thing each one is". You reach for it when you know
part of a name and want to find the thing: every net with `CAN` in it, every reference designator
starting with `U`, every bus label the reader saw.

The reason it exists as its own relation is a practical one. Searching through `component-on-net`
looks equivalent and is not. A part with no connections, a net with nothing on it, a bus that was
detected but never expanded: none of them appear in a connection relation, and those are exactly the
things worth finding during a review, since an unconnected part is usually either a mounting hole or
a mistake.

### For software engineers

An enumeration over the design graph's nodes, where every other netlist relation is an enumeration
over its edges or its node attributes. Searching for a node by walking edges misses every isolated
node, which is the bug this closes.

Rows are 1:1 with entities and carry no ordering guarantee beyond the IR's own. The relation joins to
anything keyed by the same identifier: `entity(?n, "net"), net.pin_count(?n, ?c)` counts fan-out for
every net including the ones nothing sits on, where the same query through `component-on-net` would
silently drop them.

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

Find things by a fragment of their name, the shape a search box runs:

```
entity(?name, ?kind), contains(?name, "CAN") => ?name, ?kind
```

Reference designators under a prefix, or a glob over net names:

```
entity(?name, "component"), prefix(?name, "U") => ?name
entity(?name, "net"), glob(?name, "*_CLK") => ?name
```

The isolated cases a connection relation cannot reach. Parts on no net at all:

```
entity(?ref, "component"), not component-on-net(?ref, ?any) => ?ref
```

Nets with nothing on them:

```
entity(?net, "net"), not component-on-net(?any, ?net) => ?net
```

## component-on-net

### What it is

`component-on-net(ref_des, net)` yields one row for each place a component connects to a net: the
reference designator and the net name. A component with three pins on three different nets
produces three rows; a net with eight parts on it produces eight. This is the netlist's core
adjacency, the link between the two entities every other netlist relation is keyed on.

It is the workhorse join. Most multi-relation queries pass through it to get from a component to
its nets or from a net to its components, and the other relations (`net.pin_count`, `net.bus_like`,
`net.max_voltage`, `component.class`) hang off one end or the other of this edge.

### For hardware engineers

This is "what is connected to what," the raw wire list. On its own a row just says R1 touches
`VBUS`. Its value is in the joins: which parts share a rail, whether a connector and a clamp sit
on the same signal, what is loaded onto ground. When you want to answer a connectivity question
about a specific net or part, you start here and add relations that qualify one side.

### For software engineers

This is the many-to-many edge table between components and nets, the adjacency list of the design
graph (see ANALOGY.md: a net is a shared channel aliasing pins of many instances). A component
maps to many nets and a net to many components, so neither column is unique. It is the natural
join key: any query relating a component's properties to a net's properties (or vice versa) joins
through it, the way you would join two tables through a link table. It is a projection over
`Nets()` and their connection lists, so rows are 1:1 with connections; it is empty only for a
design with no connections at all.

### Go projector

`componentOnNetFacts` in `check/facts.go` walks `Model.Nets()` and, for each net, emits one row
per entry in `net.Connections` (the component ref as subject, the net name as object). Cardinality
is one row per (component, net) connection, so a part appears once per net it lands on and a net
once per part on it. Empty only when no net carries any connection.

### Datalog

Every component-to-net link:

```
component-on-net(?r, ?n) => ?r, ?n
```

Join to `component.class` to find every diode and the nets it sits on (the shape most rules build
on, a property on one entity pulled through the edge to the other):

```
component-on-net(?r, ?n), component.class(?r, "diode") => ?r, ?n
```

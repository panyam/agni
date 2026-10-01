## component.net_count

### What it is

`component.net_count(ref_des, count)` yields one row per component, pairing its reference
designator with the number of distinct nets its pins land on. It is the part-side twin of
`net.pin_count`. Every component gets a row, so a part wired to nothing answers `0` rather than
being absent.

The count is of NETS, not pins. A resistor across `VBUS` and `VBUS_F` counts 2, and a jumper with
both pins on `GND` counts 1.

### For hardware engineers

Most coverage questions about passives start with "for each two-terminal part". A resistor,
capacitor or diode on exactly two nets is the ordinary case, and the question is usually about
both of those nets: are they both probeable, is one of them a rail, does the part sit between a
connector and a clamp. This relation picks those parts out directly.

The other two values are worth a look during a review:

- **1** is a two-terminal part with both ends on the same net. That is either a deliberate short
  (a zero-ohm link, a DNP jumper) or a wiring mistake that shorts the part out.
- **0** is a part placed and wired to nothing, which is usually a leftover or a footprint that
  never got its nets.

### For software engineers

This is the degree of each component node in the design graph, counting distinct neighbouring
nets rather than edges. It is a derived count of `component.net`, and for every ref it equals the
number of distinct `?n` in `component.net(ref, ?n)`, and a test holds the two to that. It is
total over components, so an absent row means the ref is not in the design, never that it has no
connections.

It exists because a rule body has no aggregation. "Parts on exactly two nets" otherwise needs
three copies of `component.net` and a negation (agni issue 727).

### Go projector

`componentNetCountFacts` in `stdlib/relations/facts.go` walks `Model.Components()` and then each
net's connection list, the same source `component.net` reads, and emits one row per ref with the
count in the numeric slot. It reads connections rather than part-type pins, so it answers on a bare
netlist that carries no pin data. A ref that appears in a connection with no component record still
gets a row, citing nothing because there is no placement to cite. The count is dimensionless.

### Datalog

Every component and how many nets it touches:

```
component.net_count(?r, ?c) => ?r, ?c
```

Capacitors on exactly two nets, with both nets named. The `?a < ?b` keeps one row per part rather
than one per ordering:

```
component.class(?r, "capacitor"), component.net_count(?r, 2), component.net(?r, ?a), component.net(?r, ?b), ?a < ?b => ?r, ?a, ?b
```

Parts wired to nothing:

```
component.net_count(?r, 0) => ?r
```

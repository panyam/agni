---
title: "net.hop"
description: "one series crossing: a two-net pass element (R/L/ferrite/fuse) bridging two nets, emitted in both directions. `reaches` at one step, and countable where `reaches` is not: two resistors bridging the same pair are two hops and one reach"
---

### What it is

`net.hop(from, through, to)` is one series crossing: the component `through` touches exactly two
nets, and those are `from` and `to`. Every crossing is emitted in both directions, so a question can
start from either end without knowing which way the projector wrote the row.

It is [`reaches`](../../../../reference/relations/reaches/) at one step, and the two answer different
questions. `reaches` is transitive and tells you a net is somewhere in another's neighbourhood; it
names nothing about what was crossed to get there, and it reports each destination once however many
routes reach it. `net.hop` names the part and can be counted.

### For hardware engineers

A resistor, inductor, ferrite bead or fuse placed in the middle of a signal does not break the
connection, and it does split the net: the two ends of the part are electrically different points, so
one wire on the schematic is two nets in the netlist. A hop is that part, with the net on each side.

The counting is what this is for. Two pull-up resistors from one bus to one rail are an ordinary
defect, usually because a module carries its own termination for a bus the board already pulls, and
the effective resistance is half what either was sized for. To a transitive reachability question
that board looks exactly like a correct one, since the rail is reachable either way. Two hops and one
reach is the difference.

A capacitor is never a hop. It blocks DC, so the signal does not continue through it, which is the
same rule the reach walk applies. A part touching one net is a stub and a part touching three is not
a series element, and neither is a crossing.

### For software engineers

The edge relation of the design's series graph, where the nodes are nets and an edge exists through
a two-terminal pass element. `reaches` is its transitive closure. Rows are 1:2 with a crossing (one
per direction), and the relation's size is the number of two-net pass elements, so it is small on any
board.

Ground is NOT excluded, deliberately, though the pull-up walk in the engine does exclude it. Baking
one rule's question into a fact everyone reads is the thing the fact layer is meant not to do, so a
rule that must not cross ground writes `not net.ground(?to)` and says so where a reader can see it.

An empty result means the design has no two-net pass elements, which is ordinary for a small netlist
and never means the question could not be asked.

### Go projector

`netHopFacts` in `stdlib/relations/facts.go`, over `Model.Nets()` and `Model.ComponentClass`. It
groups nets by pass component in one pass rather than scanning per component, and skips a component
whose two connections land on the same net, since that is a short rather than a crossing.

### Datalog

Every crossing, and what it goes through:

```
net.hop(?from, ?through, ?to) => ?from, ?through, ?to
```

How many distinct parts bridge a net to a rail, which is the question `reaches` cannot answer. A net
answering 2 or more has more than one path to its supply, which on an I2C bus is the double pull-up
above:

```
net.hop(?n, ?r, ?rail), rail(?rail), component.class(?r, "resistor") => ?n, count(?r)
```

Series parts on a path out of a named net, one step at a time:

```
net.hop("SDA", ?through, ?to) => ?through, ?to
```

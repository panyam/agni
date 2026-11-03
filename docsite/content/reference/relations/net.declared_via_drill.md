---
title: "net.declared_via_drill"
description: "the via drill a net SHOULD route at, cascaded across its classes by priority (join this, not the per-class rows)"
---

### What it is

`net.declared_via_drill(net, mm)` yields the via drill that the project's own net-class declarations
set for a net, held in metres and shown in mm. The net SHOULD be routed at that drill. There is one
row per net the project actually constrained. It is the resolved answer, taken after the cascade
across the net's classes has run.

Pair it with `board.via_drill(net, mm)`, the ACTUAL routed value in the same units. A comparison of the
two uses no number that the engine invented.

### For hardware engineers

A net can sit in several classes at once, and the classes can disagree. KiCad does not pick one class
and use it wholesale. It fills each constraint from the highest-priority class that states that
particular constraint, and the Default class fills whatever is left over. So a net can take its
clearance from one class and its via drill from another, exactly as the tool does when it routes.

This relation is that same resolution, so the number here is the number your layout tool would
enforce. Ask it when you want to know what a net was supposed to be, not what it is.

### For software engineers

**Join THIS, not `netclass.via_drill`.** Membership is 1:many (WS1-050), so joining
`net.netclass(?net, ?class)` to the per-class relation fans out over every class the net belongs to,
and a comparison then fires against classes that lost the cascade. So a net can be routed correctly
to its winning class's value and still produce a confident, wrong finding.

Rows are 1:1 with constrained nets. If none of a net's classes states this quantity, the net yields
no row. A rule joining it then selects only nets that the project genuinely constrained, and `not
net.declared_via_drill(?n, ?_)` reads as unconstrained.

### Go projector

`netDeclaredFacts` in `stdlib/relations/facts.go`. For each net it orders the net's classes by the
project's `priority` (ascending, unstated and unknown classes last) and takes the first class that
states the field, independently per field. The citation records WHICH class supplied the value
(`net_settings:<class>`), so a finding can say where the limit came from.

### Absence is not a pass

Only a KiCad project read populates this, and only when the project declares class definitions. On
every other source, a rule comparing declared against actual finds nothing to compare and reports
clean. A review cannot tell that from a genuine pass. Such a rule gates on `check.CapNetClassDefs`,
queryable as `design.has_netclass_defs` (and on the board tier for the actual side), so an absent
declaration reads not-applicable.

### Datalog

What every constrained net should route at:

```
net.declared_via_drill(?net, ?mm) => ?net, ?mm
```

Nets routed smaller than their own project demands:

```
net.declared_via_drill(?net, ?declared), board.via_drill(?net, ?actual), ?actual < ?declared => ?net, ?actual, ?declared
```

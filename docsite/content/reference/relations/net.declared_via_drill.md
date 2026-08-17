---
title: "net.declared_via_drill"
description: "the via drill a net SHOULD route at, cascaded across its classes by priority (join this, not the per-class rows)"
---

### What it is

`net.declared_via_drill(net, mm)` yields the via drill a net SHOULD be routed at according to the project's own
net-class declarations, in millimetres, one row per net the project actually constrained. It is the
resolved answer: the cascade across the net's classes has already happened.

Pair it with `board.via_drill(net, mm)`, the ACTUAL routed value in the same units. Declared versus actual, with
no number the engine invented.

### For hardware engineers

A net can sit in several classes at once, and the classes can disagree. KiCad does not pick one class
and use it wholesale. It fills each constraint from the highest-priority class that states that
particular constraint, and the Default class fills whatever is left over. So a net can take its
clearance from one class and its via drill from another, exactly as the tool does when it routes.

This relation is that same resolution, so the number here is the number your layout tool would
enforce. Ask it when you want to know what a net was supposed to be, not what it is.

### For software engineers

**Join THIS, not `netclass.via_drill`.** That is the entire reason this relation exists. Membership is
1:many (WS1-050), so joining `net.netclass(?net, ?class)` to the per-class relation fans out over
every class the net belongs to, and a comparison then fires against classes that lost the cascade. A
net legitimately routed to its winning class's value produces a confident, wrong finding.

Rows are 1:1 with constrained nets. A net whose classes state this quantity nowhere yields no row, so
a rule joining it selects only nets the project genuinely constrained, and `not
net.declared_via_drill(?n, ?_)` reads as unconstrained.

### Go projector

`netDeclaredFacts` in `stdlib/relations/facts.go`. For each net it orders the net's classes by the
project's `priority` (ascending, unstated and unknown classes last) and takes the first class that
states the field, independently per field. The citation records WHICH class supplied the value
(`net_settings:<class>`), so a finding can say where the limit came from.

### Absence is not a pass

Only a KiCad project read populates this, and only when the project declares class definitions. A
rule comparing declared against actual finds nothing to compare on every other source and reports
clean, which a review cannot tell from a genuine pass. Such a rule gates on `has_netclass_defs` (and
on the board tier for the actual side), so an absent declaration reads not-applicable.

### Datalog

What every constrained net should route at:

```
net.declared_via_drill(?net, ?mm) => ?net, ?mm
```

Nets routed smaller than their own project demands:

```
net.declared_via_drill(?net, ?declared), board.via_drill(?net, ?actual), ?actual < ?declared => ?net, ?actual, ?declared
```

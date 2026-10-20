---
title: "component.dnp"
description: "the part is marked do-not-populate: it stays in the netlist but is not fitted, so nothing should credit it with a role"
---

### What it is

`component.dnp(ref_des)` yields one row per component the design marks do-not-populate. The part
stays in the netlist, with its class, its pins and its nets, and this relation is what says it is
not fitted on the assembled board. Absence means the part is fitted, which is the normal case.

### For hardware engineers

A do-not-populate part has a footprint and copper but no part on it, usually because it is one side
of an assembly option: a 0 Ohm link that selects one of two supplies, a resistor that pulls a camera
line to 1.8 V beside a fitted one to 3.3 V, a TVS footprint left empty on a cost-reduced build. It
does nothing electrically, so a capacitor marked DNP decouples nothing and a TVS marked DNP clamps
nothing. A review that counts such a part as protection reads a board as safe that is not. This
relation lets a question leave those parts out.

A test point is the exception. Its pad is copper on the bare board, so a probe still lands on it,
and a pad-only test point is usually marked DNP because nothing is placed on it. The shipped
`net.has_test_point` family therefore counts unfitted test points.

### For software engineers

This is a unary predicate over components (see [the analogy
guide](../../../../docsite/content/reference/analogy.md)), keyed by ref-des so it joins
`component.class` and `component.net`. A query that credits a part with a role negates it, as in
`component.class(?r, "tvs"), not component.dnp(?r)`. The Go rules ask `Model.IsFitted` for the same
fact, so a query and a rule credit the same parts.

### Go projector

`componentDNPFacts` in `stdlib/relations/facts.go` walks `Model.Components()` and emits a row for
each ref-des `Model.IsFitted` reports unfitted. Fittedness is read from the design (KiCad's `dnp`
flag) when the model is built, and the row cites the component's own source location.

### Datalog

Every part the design leaves unfitted:

```
component.dnp(?r) => ?r
```

The TVS clamps that are actually fitted on a net:

```
component.net(?r, ?n), component.class(?r, "tvs"), not component.dnp(?r) => ?r, ?n
```

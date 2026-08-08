---
title: "has_netclass_defs"
description: "one row when the design declares net-class definitions at all (absent it, a declared-vs-actual rule has no limit to compare against and reads clean)"
---

### What it is

`has_netclass_defs(present)` yields exactly one row, with subject `true`, when the design declares any
net-class DEFINITIONS at all. No rows otherwise. It is the design-level marker for the declared side
of a declared-vs-actual comparison.

### For hardware engineers

It answers "does this project state what its classes are supposed to route at?" before you trust any
answer that depends on it. A conformance check over a project that declares nothing has nothing to
check, and that is a different result from a project that checks out clean.

### For software engineers

**Deliberately distinct from `has_netclass`, because membership and definitions are independent.**
`net_settings` carries assignments and class definitions in separate blocks, so a project can assign
nets to a class it never defines, and can define classes it assigns to nothing. A declared-vs-actual
rule needs the DEFINITIONS. Gating it on the membership marker would let a project with assignments
and no definitions report a clean pass over zero comparisons.

Same shape and purpose as `has_netclass`, `has_nc_channel`, and `types_power_out`: a design-level
presence row that lets an ad-hoc query ask whether a question is answerable on this design before
trusting the answer.

### Go projector

`hasNetClassDefsFacts` in `stdlib/relations/facts.go` emits the single row when
`Model.NetClassDefs()` is non-empty.

### Absence is not a pass

That is what this relation is for. It is the queryable half of the gate; the rule half is the
capability a declared-vs-actual rule declares, so `check.Available` reports not-applicable with a
reason instead of letting the rule find nothing and read clean.

### Datalog

Ask whether the project constrains anything before comparing against it:

```
has_netclass_defs(?_), net.declared_track_width(?net, ?mm) => ?net, ?mm
```

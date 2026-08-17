---
title: "netclass.clearance"
description: "the clearance a net class declares its nets should route at (millimetres)"
---

### What it is

`netclass.clearance(class, mm)` yields the copper-to-copper spacing a net class DECLARES its nets should route at, in
millimetres, one row per class that states one. A class that states none yields no row, which is a
meaningful fact rather than a gap: that field then cascades to a lower-priority class.

### For hardware engineers

This is the number you typed into Board Setup for that class, read back. Query it to confirm the
engine sees the constraints you set.

**There is no conformance check on this one yet.** Clearance IS checked today, but by a purpose-built Go rule against a universal fabrication floor (0.127mm), which is a different question: that rule asks whether the board is manufacturable, this relation says what the project itself demanded.

### For software engineers

**Keyed by CLASS, not by net, and there is no cascaded per-net counterpart for this quantity.** Only
`net.declared_track_width` and `net.declared_via_drill` are derived, because only those two have a
board-tier actual to be compared against (WS3-111 scope). Do not join this to `net.netclass` and
treat the result as a per-net limit: a net in several classes matches several rows, and resolving
which one wins is the cascade this relation has deliberately not done for you.

### Go projector

`netClassDefFacts` in `stdlib/relations/facts.go` walks `Model.NetClassDefs()`, the
`ir.Constraint` nodes of kind `netclass`, and parses the `clearance` param. Populated in the I/O layer
by `kicad.AnnotateNetClassDefs` from the sibling `.kicad_pro` `net_settings.classes[]` (WS3-111).

### Absence is not a pass

Only a KiCad project read populates this. An EDIF netlist, an IPC-2581 board, a bare `.kicad_sch`,
and a KiCad project that defines no classes all leave it empty. `has_netclass_defs` is the marker
that separates those cases, and it is deliberately distinct from `has_netclass`: membership and
definitions are independent blocks of `net_settings`, so a project can assign nets to a class it
never defined.

### Datalog

Every class and its declared value:

```
netclass.clearance(?class, ?mm) => ?class, ?mm
```

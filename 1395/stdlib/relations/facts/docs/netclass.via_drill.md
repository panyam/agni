## netclass.via_drill

### What it is

`netclass.via_drill(class, mm)` yields one row for each net class that DECLARES a via drill diameter
for its nets. The value is held in metres and shown in mm. A class that states no via drill diameter
yields no row. That absence is a meaningful fact rather than a gap, because the field then cascades
to a lower-priority class.

This is the DECLARED side. `board.via_drill(net, mm)` is the ACTUAL side, in the same units, so the
two join without conversion.

### For hardware engineers

This is the number you typed into Board Setup for that class, read back. Query it to confirm the
engine sees the constraints you set, and to check the project against itself. The declared drill
records what you intended, and the routed drill records what happened. A review that compares the
two contains no number the tool invented.

### For software engineers

**Keyed by CLASS, not by net, and it is NOT what a rule should compare against.** A net can be in
several classes, so joining `net.netclass` to this relation fans out and compares the net's copper
against EVERY class it belongs to. A net correctly routed at its winning class's drill will then
"fail" against a losing class's. Join `net.declared_via_drill(net, mm)` instead, which resolves the
cascade first. This relation exists for introspection and for authoring, not for comparison.

### Go projector

`netClassDefFacts` in `stdlib/relations/facts.go` walks `Model.NetClassDefs()`, the `ir.Constraint`
nodes of kind `netclass`, and parses the `via_drill` param. Populated in the I/O layer by
`kicad.AnnotateNetClassDefs` from the sibling `.kicad_pro` `net_settings.classes[]` (WS3-111).

### Absence is not a pass

Only a KiCad project read populates this. An EDIF netlist, an IPC-2581 board, a bare `.kicad_sch`,
and a KiCad project that defines no classes all leave it empty, and a rule scoped by it would find
nothing and read clean. `design.has_netclass_defs` separates those cases. It is deliberately
distinct from `design.has_netclass`, because membership and definitions are independent blocks of
`net_settings`. So a project can assign nets to a class it never defined.

### Datalog

Every class and its declared drill:

```
netclass.via_drill(?class, ?mm) => ?class, ?mm
```

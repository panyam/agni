## netclass.via_diameter

### What it is

`netclass.via_diameter(class, mm)` yields the via pad diameter that a net class DECLARES for its
nets, held in metres and shown in mm, one row per class that states one. A class that states none
yields no row. That absence is a meaningful fact rather than a gap, because the field then cascades
to a lower-priority class.

### For hardware engineers

This is the number you typed into Board Setup for that class, read back. Query it to confirm the
engine sees the constraints you set.

**There is no conformance check on this one yet.** The board tier projects via DRILL but not via pad
diameter, so there is nothing yet to compare this against.

### For software engineers

**Keyed by CLASS, not by net, and there is no cascaded per-net counterpart for this quantity.** Only
`net.declared_track_width` and `net.declared_via_drill` are derived, because only those two have a
board-tier actual to be compared against (WS3-111 scope). Do not join this to `net.netclass` and
treat the result as a per-net limit, because a net in several classes matches several rows.
Resolving which one wins is the cascade, and this relation deliberately does not do it for you.

### Go projector

`netClassDefFacts` in `stdlib/relations/facts.go` walks `Model.NetClassDefs()`, the `ir.Constraint`
nodes of kind `netclass`, and parses the `via_diameter` param. Populated in the I/O layer by
`kicad.AnnotateNetClassDefs` from the sibling `.kicad_pro` `net_settings.classes[]` (WS3-111).

### Absence is not a pass

Only a KiCad project read populates this. An EDIF netlist, an IPC-2581 board, a bare `.kicad_sch`,
and a KiCad project that defines no classes all leave it empty. The marker
`design.has_netclass_defs` separates those cases. It is deliberately distinct from
`design.has_netclass`, because membership and definitions are independent blocks of
`net_settings`. A project can assign nets to a class it never defined.

### Datalog

Every class and its declared value:

```
netclass.via_diameter(?class, ?mm) => ?class, ?mm
```

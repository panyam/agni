---
title: "board.via_drill"
description: "a via's drill diameter on a net (millimetres)"
---

### What it is

`board.via_drill(net, mm)` yields one row per net that has vias, giving the net's MINIMUM via
drill diameter in millimetres. `net` is the net name (the join key to `ir.Net.name`); `mm` is a
number, so a query can compare it directly (`?d < 0.3`). A net with several via sizes reports
only its smallest drill, not every via.

### For hardware engineers

The smallest drill on a net is the one most likely to be below a fab's minimum drill capability
or to carry too little copper for the current through it, so that is the number a review checks.
Query this to find nets with vias drilled under a house minimum, or to confirm a high-current net
does not route through an undersized via. The value is the physical drill diameter, so the
threshold you compare against is a real millimetre figure.

### For software engineers

The routed net carries a set of vias, each with a drill diameter. `board.via_drill` is an
aggregate projection over that set: a reduce to the minimum drill, keyed by net. Reporting the
minimum (the safety-relevant extreme) rather than one row per via keeps the relation a compact
per-net answer rather than a full via dump. Rows are 1:1 with nets that have at least one via; a
net with tracks but no vias contributes no row. The stored unit is nanometres; the projector
converts to millimetres so a query reads a natural threshold.

### Go projector

`boardFacts` in `check/facts.go` walks `Model.BoardNets()` and, for each net, calls the helper
`minViaDrillNm(bn.Vias)`, which returns the smallest `Drill` across the net's vias (and a false
ok when the net has no vias, in which case no row is emitted). The nanometre minimum is converted
with `nmToMM` and emitted as `board.via_drill(net, mm)` with the numeric value populated for
comparison.

The board tier is EMPTY on a netlist-only design. `Model.BoardNets()` returns nothing unless the
design was loaded with board geometry (`NewModelWithBoard`, fed a `.kicad_pcb` or IPC-2581 board
sidecar). For a query this is silent-by-construction: `board.via_drill` yields zero rows on any
design without board geometry, the same posture the datasheet tier takes without `--params`. A
query returning nothing does not mean every via is large; it can mean the design carries no board
at all.

### Datalog

Every net and its minimum via drill:

```
board.via_drill(?n, ?d) => ?n, ?d
```

Nets with a via drilled below 0.3 mm:

```
board.via_drill(?n, ?d), ?d < 0.3 => ?n, ?d
```

Both need a board-bearing design (a `.kicad_pcb` or an IPC-2581 file); on a netlist-only load they
return nothing.

### Schematic

![A net whose minimum via has a large drill versus one with a small drill]({{.Site.PathPrefix}}/static/images/catalog/relations/board.via_drill.svg)

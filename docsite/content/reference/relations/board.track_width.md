---
title: "board.track_width"
description: "a copper track's width on a net (millimetres)"
---

### What it is

`board.track_width(net, mm)` yields one row per net that has routed copper, giving the net's
MINIMUM track width in millimetres. `net` is the net name (the join key to `ir.Net.name`); `mm`
is a number, so a query can compare it directly (`?w < 0.2`). A net routed with several widths
reports only its thinnest segment, not every segment.

### For hardware engineers

The narrowest copper on a net sets its worst-case current-carrying capacity, so that is the
number a review cares about. A power or ground net that necks down to a thin trace somewhere is a
current-density risk even if most of its copper is wide. Query this to find nets routed below a
width floor, or to sanity-check that a high-current rail never drops under its intended minimum.
The value is the physical track width, so the threshold you compare against is a real millimetre
figure.

### For software engineers

The routed net is a set of track segments, each with a width. `board.track_width` is an aggregate
projection over that set: a reduce to the minimum width, keyed by net. Reporting the minimum (the
safety-relevant extreme) rather than one row per raw segment keeps the relation a compact per-net
answer rather than a full segment dump. Rows are 1:1 with nets that have at least one track; a net
with pads or vias but no routed track contributes no row. The stored unit is nanometres; the
projector converts to millimetres so a query reads a natural threshold.

### Go projector

`boardFacts` in `check/facts.go` walks `Model.BoardNets()` and, for each net, calls the helper
`minSegmentWidthNm(bn.Segments)`, which returns the smallest `Width` across the net's track
segments (and a false ok when the net has no segments, in which case no row is emitted). The
nanometre minimum is converted with `nmToMM` and emitted as `board.track_width(net, mm)` with the
numeric value populated for comparison.

The board tier is EMPTY on a netlist-only design. `Model.BoardNets()` returns nothing unless the
design was loaded with board geometry (`NewModelWithBoard`, fed a `.kicad_pcb` or IPC-2581 board
sidecar). For a query this is silent-by-construction: `board.track_width` yields zero rows on any
design without board geometry, the same posture the datasheet tier takes without `--params`. A
query returning nothing does not mean every track is wide; it can mean the design carries no board
at all.

### Datalog

Every net and its minimum track width:

```
board.track_width(?n, ?w) => ?n, ?w
```

Nets whose thinnest track drops below 0.2 mm:

```
board.track_width(?n, ?w), ?w < 0.2 => ?n, ?w
```

Both need a board-bearing design (a `.kicad_pcb` or an IPC-2581 file); on a netlist-only load they
return nothing.

### Schematic

![A net whose minimum track is wide versus one that necks down to a thin trace]({{.Site.PathPrefix}}/static/images/catalog/relations/board.track_width.svg)

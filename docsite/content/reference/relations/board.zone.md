---
title: "board.zone"
description: "the designer drew a zone to be filled with a net on a board layer; intent, not copper"
---

### What it is

`board.zone(net, layer)` yields one row for each layer on which the designer drew a zone for a net.
A zone is a region meant to be filled with the net's copper: a KiCad zone, an Allegro shape
boundary, an Altium polygon pour's outline. It states intent. The copper a fill made is in
`board.copper` with kind `pour`, where the file stores it.

### For hardware engineers

A zone's outline overlaps other nets' tracks on purpose, because the fill clears around them, so
a zone tells you what was planned rather than what was etched. Use it to ask which planes a design
meant to have, or to compare the plan against the copper. On a KiCad board, which keeps no fill,
it is the only view of the planes.

### For software engineers

Each `BoardZone` from `Model.BoardZones()` contributes its (net, layer) pair, deduplicated, so a net
with three zones on one layer yields one row. A zone on several layers was split into one zone per
layer by the reader. A zone on no net, such as a keepout, yields a row with an empty net.

### Go projector

`boardFacts` in `stdlib/relations/facts.go` walks `Model.BoardZones()` after the nets and emits one
`board.zone(net, layer)` row per distinct pair, citing the zone.

The board tier is EMPTY on a netlist-only design, so the relation yields nothing there.

### Datalog

Every zone the designer drew, by net and layer:

```
board.zone(?n, ?layer) => ?n, ?layer
```

Zones whose net has no poured copper on that layer, which on a board that stores its fills is a
zone that was never filled:

```
board.zone(?n, ?layer), not board.copper(?n, ?layer, "pour") => ?n, ?layer
```

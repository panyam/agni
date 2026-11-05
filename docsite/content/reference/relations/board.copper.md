---
title: "board.copper"
description: "a net has copper of a kind (track, via or pour) on a board layer, as etched"
---

### What it is

`board.copper(net, layer, kind)` yields one row for each kind of copper a net has on a board layer.
`kind` is `track` for a routed track, `via` for a via, and `pour` for filled copper such as a plane
or a pour. A track or pour sits on the layer it is drawn on. A via names the two layers it joins,
the outer pair for a through via, and not the inner layers it passes through. `net` is the net name
and `layer` the layer word the board reader carried through (`F.Cu`, `In1.Cu`, `B.Cu`).

### For hardware engineers

This is the copper as it will be etched. A pour counts here only when the file stores the copper
itself, which Allegro, Altium, IPC-2581 and ODB++ do. KiCad stores the outline a zone is filled from
and computes the copper whenever it fills, so on a KiCad board a zone shows in `board.zone` and not
here. Use it to ask which nets have a plane on an inner layer, or which nets reach a layer only
through vias.

### For software engineers

Think of a net's copper as three lists, tracks, vias and filled shapes, each item labelled with its
layer. `board.copper` is the distinct (layer, kind) pairs across those lists, so a net with forty
tracks on `F.Cu` yields one `track` row there. Rows are sorted by layer and then kind. An empty
result means the design has no board tier, or no net has copper with a layer.

### Go projector

`boardFacts` in `stdlib/relations/facts.go` walks `Model.BoardNets()` and calls `netCopper(bn)`,
which collects each segment's layer as `track`, each via's two layers as `via`, and each shape's
layer as `pour`, deduplicated and sorted. It emits one row per pair.

The board tier is EMPTY on a netlist-only design, so the relation yields nothing there. An empty
answer can mean the design carries no board at all.

### Datalog

Every net with a pour on the first inner layer:

```
board.copper(?n, "In1.Cu", "pour") => ?n
```

Nets that reach the back copper only through vias, with no track or pour there:

```
board.copper(?n, "B.Cu", "via"), not board.copper(?n, "B.Cu", "track"), not board.copper(?n, "B.Cu", "pour") => ?n
```

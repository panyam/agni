---
title: "copper-clearance"
description: "Copper of two different nets sits closer than the minimum the board declares, or the 0.127mm fabrication floor when it declares none."
---

### Remedy

Pull the two nets apart to the fab's minimum clearance. A gap below it is a short waiting on etch variance or a solder bridge.

### What it means

Two track segments on the same copper layer, belonging to different
nets, whose copper edges (centerline distance minus half of each width) come closer than the
minimum clearance the board declares, or, when it declares none, than 0.127mm (5mil), the loosest
mainstream fab's minimum spacing (the corpus JLCPCB rules).

![two different-net traces too close are flagged; adequate spacing is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/copper-clearance.svg)

### Why engineers want it

Clearance is the other half of every fab capability sheet.
Below it, etching cannot guarantee separation and solder bridges what etching spared, so
the board acquires connections the schematic never had.

### Impact

Order-time rejection at best; intermittent shorts in the field at worst.

### Scope note

The floor follows track-width's posture. A board that declares its own minimum is held to that instead, because the declaration is the designer saying which process the board is routed for. Today that is a KiCad project's `board.design_settings.rules`, read whichever of the project's files names the design, and every finding names which of the two it was measured against (agni issue 933). A gap may sit up to 0.0005mm under the floor and still meet it, which is KiCad's
own DRC epsilon. The distance between two diagonal tracks is not a whole number of nanometres, so
copper routed exactly at the minimum measures a nanometre or two under it.

The rule measures segment against segment on one layer only, because pad and zone clearances wait
on pad-shape geometry facts, and same-net spacing is not a defect. The pairwise walk is
O(S²) with an early bounding-box reject; fine at corpus scale (hundreds of segments),
and BenchmarkCopperClearance documents where a spatial index (WS3-004) becomes necessary. One
verdict per net PAIR, naming both nets; the finding's subject is the alphabetically first net
with the other as context, and its message names both and the worst gap.

### Query structure

the pairwise spatial join the Phase-1 AST does not express:

    select (S1, S2) in board.segments x board.segments
      where net(S1) != net(S2) and layer(S1) == layer(S2)
        and edge_distance(S1, S2) < floor - 0.0005mm

where floor is the declared minimum clearance, else 0.127mm.

Reads: board.copper, board.rules. Tier P. Primitives: select, geometry-distance (candidate primitive,
not yet in the AST, and this rule is its evidence).

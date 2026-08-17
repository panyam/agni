---
title: "annular-width"
description: "A via's annular ring is thinner than the loosest common fabrication floor (0.075mm)."
---

### What it means

A via whose copper ring, (pad diameter minus drill) / 2, is below
0.075mm, the loosest mainstream floor (the corpus JLCPCB rules).

![a via with a thin copper ring is flagged; an adequate ring is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/annular-width.svg)

### Why engineers want it

Drills wander within tolerance; the annular ring is the margin
that keeps a wandered drill inside its pad. Too little ring means breakout: the barrel
tangent to (or outside) the pad edge.

### Impact

Intermittent or open via connections that pass visual inspection.

### Scope note

Same floor-default posture as track-width; per-design values are WS3-006.

### Query structure

select nets with any via whose ring is below the floor.

    select N in board.nets where count(V in N.vias where annular(V) < 0.075mm) >= 1

Reads: board.copper. Tier P.

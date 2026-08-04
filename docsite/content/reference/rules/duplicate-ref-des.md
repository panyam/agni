---
title: "duplicate-ref-des"
description: "A reference designator is claimed by more than one distinct physical part."
---

### What it means

One reference designator (U1, R5) assigned to two genuinely distinct physical
parts, as opposed to the legitimate case of one multi-unit part whose gates share a ref-des.

### Why engineers want it

A duplicated ref-des is a common annotation slip. The two parts merge
into one BOM line (one gets built, the other silently omitted) and their connections collide on the
same net-join key, so the netlist is quietly wrong. Every capture tool's ERC flags it.

### Why this is a reader diagnostic, not an IR check

The IR keys components by ref-des and folds a
multi-unit part into one component with sections (WS1-001), so by the time the netlist exists the
collision is gone and a duplicate is indistinguishable from a multi-unit part. Only the reader, at
ingestion, can tell them apart using its format's own rule (KiCad: the same unit claimed twice). It
records the verdict in InputDiagnostics.RefDesCollisions; this rule only reports it (docs/19,
"Where a rule runs").

![One ref-des on two distinct parts is flagged; units of one multi-unit part sharing a ref-des is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/duplicate-ref-des.svg)

### Query structure

report each collision the reader recorded.

    select C in ref_des_collisions

Reads: ref_des_collision. Tier P. Site: diagnostic (reader-detected).

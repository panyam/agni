---
title: "duplicate-ref-des"
description: "A reference designator is claimed by more than one distinct physical part."
---

### Remedy

Re-annotate the schematic so each physical part holds its own designator, then re-check the BOM, since whichever part was silently merged is the one to look at first.

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

### What it means when this rule does not run

A rule that only reports a reader's verdict has nothing to say when the reader never reached one, and
"nothing to say" is what a clean design also looks like. So a reader states whether it computed the
diagnostic (InputDiagnostics.supplied), and this rule is gated to **not-applicable** where it did
not, rather than reporting a pass nobody earned (agni issue 309).

| Format | Detects | Rule reads |
|---|---|---|
| KiCad | the same unit claimed twice | a real result |
| gEDA | the same slot claimed twice, or two unslotted placements sharing a designator | a real result |
| xschem | a repeated instance name, which the format declares unique | a real result |
| IPC-2581 | a repeated refDes, with no gate construct to group placements | a real result |
| EDIF | nothing: a multi-gate part is instances sharing a designator with no unit to tell them apart | not applicable, with that reason |

An EDIF design is the case worth understanding. The gap is the format's, not the reader's: detecting
a collision there would mean reporting every multi-gate part as a duplicate. Not-applicable is the
honest answer, and it is now the answer a report shows.

![One ref-des on two distinct parts is flagged; units of one multi-unit part sharing a ref-des is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/duplicate-ref-des.svg)

### Query structure

report each collision the reader recorded.

    select C in ref_des_collisions

Reads: ref_des_collision. Tier P. Site: diagnostic (reader-detected).

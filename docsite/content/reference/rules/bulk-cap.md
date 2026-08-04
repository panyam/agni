---
title: "bulk-cap"
description: "A named power rail carries no capacitor at all (no bulk reservoir)."
---

### What it means

Every named power rail (a net an author gave a power symbol or asserted
with a power flag) should carry at least one capacitor.

### Why engineers want it

Decoupling is per-pin; bulk capacitance is per-rail. A rail with
literally no capacitance sags on every load step, and LDO datasheets require an output
capacitor for loop stability. This is the aggregate per-rail complement to
decoupling-present.

### Impact

Rail droop under transients, regulator instability, resets that only reproduce
under real load patterns.

![A power rail with no capacitor anywhere is flagged; a rail carrying bulk and decoupling caps is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/bulk-cap.svg)

### Scope note

Rail identity is the global / power_driven net facts (power symbols and
power flags), not pin directions, so a rail feeding only passives is still covered;
decoupling-present quantifies over power-input pins instead, and the two deliberately
overlap on a rail that has power pins and no caps. Ground-named nets are excluded
(capacitors land ON ground from every rail; "ground has no bulk cap" is not a defect), and
unresolved external nets are skipped (the cap may live in an unread sheet). Distinguishing
bulk from local decoupling by value is a datasheet-joined refinement (WS10).

### Query structure

select rails by fact, require a capacitor member.

    select N in nets where (global(N) or power_driven(N)) and not ground_name(N)
      and not exists P in N.connections where class(P) == capacitor

Reads: component.class, net.attributes (global, power_driven, external), net.names (the
ground-name skip), on_net. Tier R.

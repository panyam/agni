---
title: "supply-exceeds-abs-max"
description: "A power-input pin sits on a rail whose nominal voltage exceeds the part's absolute-maximum supply rating."
---

### Remedy

Feed the part from a rail inside its rated supply range, or change the part for one rated to the rail it sits on. An absolute maximum is a damage threshold rather than a tolerance to design against.

### What it means

A component joined to a seeded datasheet spec (by MPN) has a power-input
pin on a rail whose name states a nominal voltage above the spec's absolute-maximum supply
rating (VIN/VDD/VCC-family symbol, limit kind ABSOLUTE_MAX).

![a rail above the datasheet abs-max is flagged; a rail under it is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/supply-exceeds-abs-max.svg)

### Why engineers want it

This is the wedge of the datasheet layer (docs/20): the limit is
not a heuristic threshold baked into a rule, it is the vendor's own number, carried with
provenance (document revision, page, table, extraction method, confidence) into the finding.
Reviewers verify the citation, not the tool.

### Evidence honesty

Every input that cannot be trusted is a skip, never a guess:
- no MPN, unseeded MPN, or no seeded set at all -> silent (skip-not-false-pass);
- limit rows that are under-specified or carry text-only conditions are not compared
  (param.MachineComparable, docs/20 comparison semantics);
- a row printed in a prefixed unit (mV, kV) is reduced to volts by the parameter layer's one
  conversion table, so a spec seeded as the sheet prints it compares; a unit that table does not
  recognize is skipped rather than scaled by a guess;
- a rail name with no parseable nominal, or with conflicting nominals, is not compared
  (the net name is the only voltage evidence a netlist carries).

### Query structure

join components to specs by MPN; for each power_in pin, parse the
attached rail's nominal from its net name and compare against the most restrictive
machine-comparable abs-max supply row.

    select C in components where spec(C) != nil
      for P in pins(C) where electrical_type(P) == power_in
        nominal(net(P)) > min(supply_abs_max(spec(C))) -> finding

Reads: param.supply_abs_max, pin.electrical_type, net.name, on_net. Tier R.

---
title: "cap-voltage"
description: "A capacitor's datasheet rated voltage does not clear the worst rail it touches times the derate factor."
---

### What it means

A component classed as a capacitor, joined by MPN to a seeded datasheet
spec carrying a rated voltage (VDC/WV/VR-family symbol or a "Rated Voltage" row), sits on a
rail whose declared voltage times the derate factor (`1.25`) exceeds that rating.

![a cap whose derated rail exceeds its rating is flagged; margin is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/cap-voltage.svg)

### Why engineers want it

This is the stakeholder-named cap-voltage rule: the classic
review checklist item ("is every cap rated for its rail, with margin?") turned into a check
whose limit is the vendor's number with provenance, not a rule-of-thumb constant in code.
The finding cites the datasheet page/table so the margin is verifiable, which is the
datasheet layer's whole posture (docs/20).

### Evidence honesty

Every untrusted input is a skip, never a guess: no MPN / unseeded MPN
/ no seeded corpus; no machine-comparable rated-voltage row (docs/20 comparison semantics);
units other than "V"; a rail with neither a max_voltage attribute nor a name-derived
nominal. The worst (highest) known rail among the cap's nets governs.

### Query structure

Spec-authored (docs/19 "a rule is a value"); the join and float
compare live in the cap_voltage_detail SpecFunc, which returns the violation sentence or ""
— so the rule body stays AST and the derived Reads carry the param join as named relations.

    select C in components where class(C) == capacitor
      and cap_voltage_detail(C) != ""    // Vrated < worst_rail_V x 1.25, seeded and comparable

Reads: param.cap_rated_voltage, net.max_voltage, component.mpn, component.class, on_net. Tier R.

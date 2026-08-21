---
title: "decoupling-present"
description: "A power rail feeds power-input pins but has no decoupling capacitor on it."
---

### Remedy

Add a decoupling capacitor from the rail to ground at each supply pin, and place it at the pin in layout. A capacitor drawn on the rail but placed across the board does not decouple it.

### What it means

Every supply rail that feeds a power-input pin (a chip's VCC/VDD) should carry
at least one capacitor. The cap is the local charge reservoir that serves the chip's switching
transients; the regulator is electrically far away.

### Why engineers want it

Missing decoupling is a classic review catch and a recurring
field-reliability defect: the design works at the bench, then resets or corrupts under load or
temperature. Vendor app notes (TI, ST, NXP) specify decoupling per power pin precisely because it
is external and easy to forget.

### Impact

Supply sag and ground bounce at the pin: intermittent resets, logic corruption, EMC
failures. Rarely visible at first power-on, expensive to find later.

![Power rail with no capacitor is flagged; rail with a decoupling cap is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/decoupling-present.svg)

### Scope note

This is the netlist presence check: "a capacitor somewhere on the rail". It does
not check value, count-per-pin, or placement distance, since proximity is a board-tier notion and the
value/impedance version is a datasheet-joined rule (Tier X). Ground-named nets (GND/VSS/...) are
skipped: their pins are power_in too, but decoupling is a supply-rail concept and the same
capacitor already sits on the ground side. Cross-sheet rails are skipped (the cap may live on a
sheet we did not read). Capacitor identity is the shared component.class fact.

### Query structure

select power nets with no capacitor member.

    select N in nets where any(power_in(P)) and not ground(N)
      and not exists C in N.connections where class(C) == capacitor

Reads: component.class, net.attributes (external), net.names (the ground-name skip), on_net,
pin.electrical_type. Tier R.

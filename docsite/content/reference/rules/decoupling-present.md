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

### Two nets that declare a supply pin and are not rails

A power-input pin is much weaker evidence of a supply rail than it looks, and on one real
multi-thousand-component board every one of this rule's 14 findings was a false positive. Two shapes
account for most of them and the rule now excludes both.

**A net driving a transistor's GATE is a control node.** A gate drive carries a controller output and
a FET gate, and nothing about it wants a capacitor to ground.

The guard asks about the gate rather than about the transistor, deliberately. A high-side load
switch's OUTPUT carries the FET's source, feeds downstream supply pins, and wants decoupling exactly
as much as a regulator output does. Excluding every net with a transistor on it would silence that.

**A net carrying an inductor beside a transistor is a switching node.** That is a buck converter's
switch node, and the advice to fit a capacitor there would short the switch to ground every cycle. A
rule that tells you to destroy the circuit is worse than a rule that says nothing.

The inductor only disqualifies alongside a transistor. On its own it is an LC or ferrite FILTER, and
a filtered supply is a rail that genuinely wants decoupling on the far side.

Both are proxies for a question this rule cannot ask yet. "The switching node of a buck converter" is
a topology question, an inductor between a switch node and an output with the capacitor beyond it,
which is what the topology pattern work is for. Expect these class checks to be replaced by a pattern
that states it properly.

### Query structure

select power nets that are plausibly rails and carry no capacitor member.

    select N in nets where any(power_in(P)) and not ground(N)
      and not exists Q in N.connections where class(Q) == transistor and role(Q) == gate
      and not (exists L in N.connections where class(L) == inductor
               and exists Q in N.connections where class(Q) == transistor)
      and not exists C in N.connections where class(C) == capacitor

Reads: component.class, net.attributes (external), net.names (the ground-name skip), on_net,
pin.electrical_type. Tier R.

The gate guard also consults pin.role, which the declared read set does not list: that field is
validated against the declarative twin, and the twin carries the comparison without the scope guards.
The omission is safe in the direction that matters, since a format carrying no pin data yields
RoleUnknown and the guard does not fire there.

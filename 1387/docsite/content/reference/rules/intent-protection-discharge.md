---
title: "intent/protection-discharge"
description: "A rail the design intent declares needs a discharge path has no bleeder resistor."
---

### Remedy

Fit a bleeder resistor across the rail, sized so the rail discharges within the time the design assumes when it powers down.

### What it means

The design intent declares that a named rail must have a discharge path, a bleeder resistor that
bridges the rail to ground so stored charge drains when the supply is removed. This rule fails when
the declared rail carries no such resistor. It probes that exact net for a component that is a
resistor AND also touches a ground net, and a declared rail with none is flagged.

### Why engineers want it

A rail with bulk capacitance remains energized after power-down. A bleeder gives it a defined,
intended discharge path so the rail is safe to touch and predictable on the next power cycle. The
netlist does not state which rails need bleeding, since that is an architecture decision. So the
intent declaration names the rail and this rule verifies the resistor-to-ground is present.

### Impact

The design intended a rail to discharge, and the rail holds charge after power-down instead. On a
high-voltage rail that is a shock or arc hazard. It can also make power sequencing unpredictable on
the next cycle, because a capacitor never bled down.

![A declared rail with no bleeder is flagged; the same rail with a resistor to ground is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/protection-discharge.svg)

### Scope note

`discharge` is realized by a resistor that also reaches a ground net. The ground is name-derived,
by the same predicate that the `net.ground` fact uses. It is one of the per-kind protection rules
(`intent/protection-ovp`, `intent/protection-discharge`), each emitted only when the declaration
carries that kind. The rule iterates the declared protections and probes the design; it never derives
the protected-rail set from the netlist.

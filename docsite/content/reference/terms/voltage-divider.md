---
title: "Voltage divider"
label: "divider"
summary: "Two resistors in series between a rail and ground, with a tap between them. The tap rests at a fixed fraction of the rail, set by the ratio of the two."
level: EE3
---

A regulator needs to know its own output voltage, scaled down to a reference it can compare against.
An ADC needs a 12 V input scaled into its 3.3 V range. Both use two resistors in series, from the
voltage to ground, and read the point between them.

{{ includeFile "figures/voltage-divider.svg" }}

The tap rests at the top voltage times R2 over R1 plus R2. Equal resistors give half. A regulator's
feedback divider sets its output this way, and changing one resistor changes the rail.

A divider looks like a [pull-up](../pull-up/) and a [pull-down](../pull-down/) on the same net,
and agni reads it that way. The bias check reports a net pulled both ways as neither. So a rule
asking whether a reset or a strap is pulled to the right level can't answer on a divider.
[`property-reset-polarity`](../../rules/intent-property-reset-polarity/) reports it inconclusive,
with a message saying a divider holds the line between the rails. The level a receiver reads then
depends on the ratio against its input thresholds, and nothing in a netlist states those thresholds.

Probing a divider tap disturbs it. Probe capacitance on a regulator's feedback node shifts what the
regulator sees, and the output moves while you measure it.

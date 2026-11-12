---
title: "intent/load-switch-trip-below-budget"
description: "A load switch's current limit is set below the peak current the design intent declares for the rail it feeds."
---

### Remedy

Raise the switch's current limit above the rail's declared peak, or lower the peak the rail is budgeted for. As set, the switch trips in normal operation.

### What it checks

A load switch can be built from a controller and an external MOSFET. It limits current at a point
the designer sets. The controller states a threshold voltage, the designer chooses a sense resistor,
and the limit is the threshold divided by that resistance. The limit must be above the rail's actual
draw. When it is below, the switch opens at the load the design was drawn for. The rail then never
comes up under load, and the protection is the fault.

The draw comes from the `peak` declared on the rail's `nets` entry. The threshold comes from the controller's seeded
datasheet. The sense resistance comes from the design. All three are required, and any one missing
means no verdict rather than a pass.

### For hardware engineers

Sizing a load switch is a window, not a limit. The pass element sets the upper edge. If the limit
is above what the FET survives, the FET fails before the controller acts, and a high-side switch that
fails short hands the full rail to the load. `load-switch-trip-above-fet-rating` reports that edge.
This rule reports the lower edge, which fails the other way. If the limit is under the load current,
the switch never passes the design's own current.

The two edges need different evidence, so they are different rules in different places. The
upper edge is two datasheet numbers, so it is decidable from the design plus the seeded parts. The
lower edge needs the load current, and nothing in a design states that. A netlist carries connectivity,
not current. The controller's datasheet cannot know what was hung off the switch. Adding up every
load's rated draw would need a seeded datasheet for nearly every part on the board, plus an
assumption about which loads draw at once. So the author declares the demand, and the engine compiles
the rule from that declaration.

The trip point is arithmetic on two parts, and the schematic shows both without comment. One is a
milliohm shunt, and a reviewer reads past its value. The other is a threshold buried in the
controller's electrical table. Neither looks wrong on the page. It shows up at bring-up as a rail
that cycles under load.

### Read this before binding a review item to it

**Silence is not a sized switch.** The rule reports no finding in four cases, and only the last of
them means the design is fine:

- The declared rail is not in the design. The voltage-domain and subsystem forms report that as a
  missing-rail defect, and reporting it here too would put one defect under two items.
- No controller-based load switch reaches the rail. The rail may be unswitched. It may be switched
  by an INTEGRATED part, one component with no external FET, which leaves the topology walk nothing
  to resolve. Or the resolver may have refused its switch, because the controller, FET or shunt was
  ambiguous.
- The trip current could not be computed, because the controller is unseeded or states no overcurrent
  threshold, or the shunt's value is not stated in ohms in the design. The review runner's `needs-data`
  covers the design-wide form of this, where nothing on the board states a threshold. This rule feeds
  `needs-data` by declaring the symbols it joins on. `needs-data` does not cover the narrower case
  where some other part is seeded and this controller is not, and that case still reads pass.
- The limit is at or above the declared peak.

**A pass here says only that the limit is above the declared draw.** It does not say the limit is
below what the FET survives, and it does not say the FET runs cool at that current.

**The topology walk needs pin roles on the MOSFET.** The walk resolves the switch by finding the
controller that drives the FET's GATE. Some source formats do not carry enough information to assign
gate, source and drain roles. Such a format resolves no switch, and this rule is silent on every
design read from it. The silence comes from the reader, not from the design.

### How the switch is found, and what it refuses

The pass element is the MOSFET whose gate net carries exactly one part that states an overcurrent
threshold. The shunt is the one resistor below an ohm with every terminal on a net the controller
also touches. That shape is the structural signature of Kelvin sensing.

Every step refuses ambiguity rather than guessing. Two candidate controllers on a gate net yield no
switch at all. So do two candidate shunts, a gate net shared by two pass elements, and a shunt whose
value the design does not state in ohms. Imagine the walk picked the wrong resistor. The over-current
verdict it computed would look exactly as authoritative as a correct one. So a wrong answer costs
more here than no answer.

A gate resistor between the controller and the FET defeats the walk. This is a known limit rather
than an oversight. A walk that crossed passives on a gate net would also cross a gate pull-down into
the source net's neighbourhood, and reach the wrong controller.

### Which switch, where two reach one rail

The one with the HIGHEST limit, and both sides of the switch count as reaching the rail (a series
element carries the same current on its input and its output).

The rule takes the highest, not the lowest, for the same reason `intent/rail-current-capacity`
takes the highest-rated supply. If it picked the smallest of several limits, it would report a
nuisance trip on a switch that gates a different branch. Where the evidence is ambiguous the rule
takes the reading that does not fire. The cost is a missed finding on a rail genuinely gated by the
smaller of two switches.

### The dissipation figure in the finding

Where the pass FET's RDS(on) is seeded, the finding also reports what the FET dissipates at the
declared current. It is reported and never judged.

Judging it needs a thermal limit, made of a package thermal resistance, an ambient, and the junction
rise that the house accepts. The parameter layer reads no datasheet row that states one, and no
declaration field carries one. So a rule that failed on dissipation would be failing against a
threshold nobody declared. The figure is there because a reviewer needs it next. The fix for a limit
set too low is a smaller shunt, and a smaller shunt only helps if the FET can carry the budgeted
current at all.

### Declaring it

Declare the same `peak` that the regulator-sizing rules read, with no extra field:

```yaml
intent:
  nets:
    VSW_CAM: {peak: 2.5}
```

Declare the budget on either side of the switch. There is deliberately no way to declare "this rail
is switched". A design with no switch on a budgeted rail resolves none, and the rule is silent. An
author could forget such a field, and the field would turn that omission into a defect.

### Fixing a finding

Either the shunt is too large or the budget is wrong, and they are not equally easy to tell apart.
Work out the intended limit first, from the budget and the FET's rating together, then pick the
shunt from the controller's threshold. Check the FET's dissipation at the new limit before changing
the shunt. If the new limit is past what the pass element can carry, this finding turns into
`load-switch-trip-above-fet-rating`.

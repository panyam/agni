## load-switch-trip-below-budget

### What it checks

A load switch built from a controller and an external MOSFET limits current at a point the designer
sets: a threshold voltage the controller states, divided by a sense resistor the designer chose. That
limit must sit above the current the rail actually draws. When it sits below, the switch opens on the
load the design was drawn for, so the rail never comes up under load and the protection is the fault.

The draw comes from the declaration's `rail_budgets`. The threshold comes from the controller's seeded
datasheet. The sense resistance comes from the design. All three are required, and any one missing
means no verdict rather than a pass.

### For hardware engineers

Sizing a load switch is a window, not a limit. The upper edge is the pass element: a limit set above
what the FET survives means the FET fails before the controller acts, and a high-side switch that fails
short hands the full rail to the load. `load-switch-trip-above-fet-rating` reports that edge. This rule
reports the lower edge, which fails the other way: a limit under the load current means the switch
never passes the design's own current.

The two edges need different evidence, so they are different rules in different places. The
upper edge is two datasheet numbers, so it is decidable from the design plus the seeded parts. The
lower edge needs the load current, and nothing in a design states that. A netlist carries connectivity,
not current. The controller's datasheet cannot know what was hung off the switch. Adding up every
load's rated draw would need a seeded datasheet for nearly every part on the board plus an assumption
about which loads draw at once. So the demand is declared, and the rule that reads it is compiled out
of the declaration.

The trip point is arithmetic on parts the schematic shows without comment: a milliohm shunt whose value
a reviewer reads past, and a threshold buried in the controller's electrical table. Neither looks wrong
on the page. It shows up at bring-up as a rail that cycles under load.

### Read this before binding a review item to it

**Silence is not a sized switch.** The rule reports nothing at all in four cases, and only the last of
them means the design is fine:

- The declared rail is not in the design. That is a missing-rail defect the voltage-domain and
  subsystem forms report; reporting it here too would put one defect under two items.
- No controller-based load switch reaches the rail. The rail may be unswitched, or switched by an
  INTEGRATED part (one component with no external FET, so there is nothing for the topology walk to
  resolve), or gated by a switch the resolver refused because its controller, FET or shunt was
  ambiguous.
- The trip current could not be computed: the controller is unseeded or states no overcurrent
  threshold, or the shunt's value is not stated in ohms in the design. The review runner's `needs-data`
  covers the design-wide form of this (nothing on the board states a threshold), which this rule feeds
  by declaring the symbols it joins on. It does not cover the narrower case where some other part is
  seeded and this controller is not, which still reads pass.
- The limit is at or above the declared peak.

**A pass here says one thing only: the limit is above the declared draw.** It does not say the limit is
below what the FET survives, and it does not say the FET runs cool at that current.

**The topology walk needs pin roles on the MOSFET.** The switch is resolved by finding the controller
that drives the FET's GATE, so a source format that does not carry enough information to assign gate,
source and drain roles resolves no switch and this rule is silent on every design read from it. That is
a property of the reader, not of the design.

### How the switch is found, and what it refuses

The pass element is the MOSFET whose gate net carries exactly one part that states an overcurrent
threshold. The shunt is the one resistor every terminal of which lands on a net the controller also
touches (the structural signature of Kelvin sensing), below an ohm.

Every step refuses ambiguity rather than guessing. Two candidate controllers on a gate net, two
candidate shunts, a gate net shared by two pass elements, a shunt whose value the design does not state
in ohms: each of those yields no switch at all. An over-current verdict computed from the wrong
resistor looks exactly as authoritative as a correct one, so a wrong answer costs more here
than no answer.

A gate resistor between the controller and the FET defeats the walk. That is a known limit rather than
an oversight: crossing passives on a gate net would also cross a gate pull-down into the source net's
neighbourhood, and reach the wrong controller.

### Which switch, where two reach one rail

The one with the HIGHEST limit, and both sides of the switch count as reaching the rail (a series
element carries the same current on its input and its output).

Highest, not lowest, for the reason `intent/rail-current-capacity` takes the highest-rated supply:
picking the smallest limit among several would report a nuisance trip on a switch that gates a
different branch. Where the evidence is ambiguous the rule takes the reading that does not fire. The
cost is a missed finding on a rail genuinely gated by the smaller of two switches.

### The dissipation figure in the finding

Where the pass FET's RDS(on) is seeded, the finding also reports what the FET dissipates at the
declared current. It is reported and never judged, and the distinction is deliberate.

Judging it needs a thermal limit: a package thermal resistance, an ambient, a junction rise the house
accepts. No datasheet row the parameter layer reads states one and no declaration field carries one, so
a rule that failed on dissipation would be failing against a threshold nobody declared. The figure is
there because it is the number a reviewer needs next: the fix for a limit set too low is a smaller
shunt, and that only helps if the FET can carry the budgeted current at all.

### Declaring it

The same `rail_budgets` the regulator-sizing rules read. No extra field:

```yaml
rail_budgets:
  - {rail: VSW_CAM, peak: 2.5}
```

Declare the budget on either side of the switch. There is deliberately no way to declare "this rail is
switched": a design with no switch on a budgeted rail resolves none and stays silent, and a field an
author could forget would turn that omission into a defect.

### Fixing a finding

Either the shunt is too large or the budget is wrong, and they are not equally easy to tell apart. Work
out the intended limit first, from the budget and the FET's rating together, then pick the shunt from
the controller's threshold. Check the FET's dissipation at the new limit before changing the shunt: a
limit raised past what the pass element can carry trades this finding for
`load-switch-trip-above-fet-rating`.

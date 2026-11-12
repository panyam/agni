## rail-current-capacity

### What it checks

Suppose the design intent declares a current budget for a rail. A part rated to deliver at least that
much must supply the rail. The budget comes from the declaration; the rating comes from the supplying
part's seeded datasheet. When the rating is below the budget, the rail is over-subscribed by the
design's own numbers.

Both halves are required. Without a declaration there is nothing to check against, and without a
seeded datasheet there is no rating to check.

### For hardware engineers

A regulator states a maximum output current on its datasheet. Past it the part does not deliver more
but protects itself, as the output droops, or a current limit folds it back, or thermal shutdown
cycles it. None of that is visible in a schematic, because the schematic shows what is connected and
never how much any of it draws.

So the design intent has to declare the number. A netlist carries connectivity, not current. The
regulator's own datasheet cannot tell you what the designer hung off it. Adding up every load's rated
draw would need a seeded datasheet for nearly every part on the board. It would also need an
assumption about which loads draw at once. That assumption is an architecture decision, and the design
does not state it as a fact.

### Read this before binding a review item to it

**This rule is silent on a rail that no seeded part supplies.** It has nothing to compare, and firing
there would report a defect the design may not have.

That case should get the review runner's `needs-data` verdict. These rules feed it by declaring the
datasheet symbols they join on. That gate is design-wide, so a bound item reads needs-data rather than
pass only when **nothing** on the board states an output current. It does not yet catch a narrower
case. Some regulator is seeded, the one feeding this particular rail is not, and the item still reads
pass. Seed the supplying part before treating a pass here as a sized rail.

**A rating stated in milliamps is reduced to amps and compared.** Seed output currents as the sheet
prints them. The parameter layer holds one conversion table, next to the comparability predicates, and
every extractor reads through it, so no rule contains a scale factor. When the table does not
recognize a unit, the value is skipped rather than scaled by a guess. That leaves a narrow residual
case. The symbol is seeded, needs-data does not cover it, and the item reads pass.

Until agni issue 148 a milliamp rating was skipped instead. Milliamps are the ordinary spelling for a
sub-amp regulator, so a spec transcribed as printed left this rule with nothing to compare. The rule
then scored the rail a clean pass while the architecture's own numbers over-subscribed it.

`intent/rail-current-margin` reports the separate question of whether the rail clears the budget
with headroom. **A pass here means "rated for the declared load", not "adequately sized".** A supply
that exactly meets the peak passes this rule and, when a `margin_factor` is declared, fires the
margin rule.

### What counts as the supply

The supply is the highest-rated seeded part on the rail or within one series element of it. So a
ferrite bead or a sense resistor between the regulator and the rail does not hide the supply.

Where several parts qualify, the rule takes the highest-rated, not the lowest. A rail can be within
reach of more than one seeded part, and a multi-channel regulator states a rating per channel with no
way to say which channel this net is. Picking the smallest of those would report a shortfall that the
design does not have, so where the evidence is ambiguous the rule takes the reading that does not
fire. The cost is a missed finding on a rail genuinely fed by the smaller of two supplies.

This needs no power-output pin typing, so the rule works the same on a format that does not classify
power outputs as on one that does.

### Declaring it

```yaml
intent:
  nets:
    +3V3: {peak: 0.8}
    +1V8: {peak: 0.35}
```

`peak` is in amps and must be positive. Every supply meets a zero budget, so a zero declaration could
only pass. One budget per rail.

There is deliberately no `typical` field. Neither shipped rule would read one. An author who fills in
a number believes it is being verified, so a declared number that nothing checks is worse than an
absent one. Adding it later is additive, and it should arrive with the rule that consumes it.

### Fixing a finding

Either the supply is too small or the budget is wrong. Both happen, and they are not equally easy to
tell apart, because a budget written early in a design often outlives the loads it was written for.
Confirm the budget against the current architecture before specifying a bigger part.

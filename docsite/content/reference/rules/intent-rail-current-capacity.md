---
title: "intent/rail-current-capacity"
description: "The part supplying a rail is rated below the peak current the design intent declares for that rail."
---

### What it checks

A rail whose current budget the design intent declares must be supplied by a part rated to deliver at
least that much. The budget comes from the declaration; the rating comes from the supplying part's
seeded datasheet. When the rating is below the budget, the rail is over-subscribed by the design's own
numbers.

Both halves are required. Without a declaration there is nothing to check against, and without a
seeded datasheet there is no rating to check.

### For hardware engineers

A regulator states a maximum output current on its datasheet. Past it the part does not deliver more,
it protects itself: the output droops, or a current limit folds it back, or thermal shutdown cycles it.
None of that is visible in a schematic, because the schematic shows what is connected and never how
much any of it draws.

That is why the number has to be declared. A netlist carries connectivity, not current. The
regulator's own datasheet cannot tell you what the designer hung off it. Adding up every load's rated
draw would need a seeded datasheet for nearly every part on the board, plus an assumption about which
loads draw at once, which is an architecture decision rather than a fact the design states.

### Read this before binding a review item to it

**This rule is silent on a rail no seeded part supplies.** It has nothing to compare, and firing there
would report a defect the design may not have.

The honest verdict for that case is the review runner's `needs-data`, which these rules feed by
declaring the datasheet symbols they join on. That gate is design-wide: if **nothing** on the board
states an output current, a bound item reads needs-data rather than pass. It does not yet catch the
narrower case where some regulator is seeded and the one feeding this particular rail is not, which
still reads pass. Seed the supplying part before treating a pass here as a sized rail.

**A rating stated in milliamps is skipped, not converted.** Unlike units are under-specified for
comparison across the whole parameter layer, and scaling here would put the engine's one silent unit
conversion inside a rule that decides pass or fail. Seed output currents in amps.

`intent/rail-current-margin` reports the separate question of whether the rail clears the budget with
headroom. **A pass here means "rated for the declared load", not "adequately sized".** A supply that
exactly meets the peak passes this rule and fires the margin rule.

### What counts as the supply

The highest-rated seeded part on the rail or within one series element of it, so a ferrite bead or a
sense resistor between the regulator and the rail does not hide the supply.

Highest-rated, not lowest, where several qualify. A rail can be within reach of more than one seeded
part, and a multi-channel regulator states a rating per channel with no way to say which channel this
net is. Picking the smallest of those would report a shortfall the design does not have, so where the
evidence is ambiguous the rule takes the reading that does not fire. The cost is a missed finding on a
rail genuinely fed by the smaller of two supplies.

This needs no power-output pin typing, so the rule works the same on a format that does not classify
power outputs as on one that does.

### Declaring it

```yaml
rail_budgets:
  - {rail: +3V3, peak: 0.8}
  - {rail: +1V8, peak: 0.35}
```

`peak` is in amps and must be positive: a zero budget is met by every supply, so it would be a
declaration that can only pass. One budget per rail.

There is deliberately no `typical` field. Neither shipped rule would read one, and a declared number
nothing checks is worse than an absent one, because an author who fills it in believes it is being
verified. Adding it later is additive, and it should arrive with the rule that consumes it.

### Fixing a finding

Either the supply is too small or the budget is wrong. Both happen, and they are not equally easy to
tell apart: a budget written early in a design often outlives the loads it was written for. Confirm
the budget against the current architecture before specifying a bigger part.

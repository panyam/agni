---
title: "intent/rail-current-margin"
description: "A rail's supply meets its declared peak current budget but not the declared margin factor over it."
---

### What it checks

A rail's supply must be rated for its declared peak budget multiplied by the declared margin factor.
A factor of 1.2 asks for 20% headroom over the budget.

The rule fires only in the band between the two thresholds: the supply clears the peak but falls short
of peak x factor. Below the peak it stays silent, because that is
`intent/rail-current-capacity`'s finding and one defect should not be reported under two review items.

### For hardware engineers

A current budget is an estimate. It is assembled early, from parts that are not all chosen yet, at
temperatures nobody has measured, and it does not capture inrush, a load added late in the schedule,
or a part drawing more than its typical figure because it is running hot.

Headroom is what absorbs that. Sizing a supply to exactly the budget means the first thing the budget
missed turns into a droop, and by then the board is built.

### Read this before binding a review item to it

**The margin factor is declared policy, not physics.** Nothing in electronics says 20%. Different
teams use different numbers for different rail classes, and the right number for a rail feeding a
processor's core is not the right number for one feeding an LED. So the factor is an input to the
check, never a constant inside it, the same discipline that keeps naming vocabularies in a lexicon
instead of in rule literals.

**It has no default.** With no `margin_factor` declared this rule is not compiled at all, so a review
item bound to it reads `needs-design-intent` and names the missing input. That is deliberate: a
default would let an item read PASS against a policy number nobody on the project ever stated, and a
pass nobody can trace back to a decision is the thing this whole family of checks exists to prevent.

It shares `intent/rail-current-capacity`'s two limits, which are stated in full on that card: the rule
is silent on a rail no seeded part supplies, and the needs-data gate that covers the unseeded case is
design-wide rather than per-rail.

Thermal derating is a separate question and is not modelled here. A regulator's usable output falls
with ambient temperature and with how much voltage it is dropping, so a part rated 1A on its front page
may not deliver 1A in the enclosure this board ships in. This rule compares against the datasheet
number as stated.

### Declaring it

```yaml
rail_budgets:
  - {rail: +3V3, peak: 0.8}
margin_factor: 1.2
```

One factor applies to every budget in the declaration. It must be greater than 1: a factor of 1
restates the capacity rule, and anything below 1 asks for a supply smaller than the budget. Both are
rejected at load rather than becoming a second rule that duplicates or inverts the first.

### Fixing a finding

The supply carries the load but has no room. Specify a larger part, cut the budget, or record a
deliberate decision to run this rail closer to its rating than the house factor allows. The third is a
legitimate answer, and it belongs in the declaration or the review record rather than in silence.

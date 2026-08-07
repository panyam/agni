## property-reset-polarity

### What it checks

A net the design intent declares as an active-low (or active-high) reset must not be **biased to its
asserted level**. Declared active-low with a pull-down, or active-high with a pull-up, is a
contradiction between what the design says it wants and what it actually does.

### Read this before binding a review item to it

**A pass from this rule means "no contradiction found", NOT "polarity confirmed".** That distinction
is the whole character of the check and it is not a defect in the implementation — it is what a
netlist can support.

A netlist states polarity nowhere. The only structural evidence is the bias resistor, and plenty of
correct designs carry none: a supervisor or PMIC with an internal pull-up drives the reset line by
itself, and the schematic shows a bare net. So the rule can catch a design that contradicts its own
declaration, and it cannot confirm one that merely does not evidence it.

The engine has no per-subject not-applicable — a review outcome is per item and follows whether the
rule fired — so the choice was between staying silent on the undecidable case (what this does) and
failing a declaration the design simply does not evidence, which would report a non-defect. Reporting
unverifiable declarations is a useful separate check; it is deliberately not this one.

Its sibling `property-ac-coupled` is genuinely decidable and does not carry this caveat.

### For hardware engineers

An **active-low** reset (usually drawn `RESET_N`, `nRST`, `RST#`) holds the part in reset while the
line is LOW and lets it run when the line is HIGH. So the resting state must be high, which is what a
pull-up to the rail provides.

Put a pull-down on that line instead and the part is held in reset from the moment power comes up.
The board looks dead, and the cause is one resistor to the wrong net — a bring-up failure that reads
as a broken part or bad firmware for as long as it takes someone to meter the reset pin.

Active-high is the mirror image: it should rest low, so a pull-up holds it permanently asserted.

### What counts as bias

A resistor on the declared net whose other end reaches a power rail (pull-up) or a ground net
(pull-down).

A net with **both** — a divider — reports neither. A divider sets an intermediate level, so it does
not hold the line at either rail, and calling it a contradiction would be wrong.

### Declaring it

```yaml
net_properties:
  - {net: SYS_RESET_N, property: reset-polarity, value: low}
  - {net: PHY_ENABLE,  property: reset-polarity, value: high}
```

The `value` is the assertion, and it is required: without it the rule has nothing to contradict, so an
omitted or misspelled level is rejected at load rather than becoming a rule that silently never fires.

### Fixing a finding

The bias resistor goes to the wrong net, or the declared polarity is wrong. Check the part's datasheet
before moving the resistor — if the declaration is what is wrong, moving the resistor would create the
bug the rule was reporting.

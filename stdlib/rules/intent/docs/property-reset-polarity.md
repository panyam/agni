## property-reset-polarity

### What it checks

A net the design intent declares as an active-low (or active-high) reset must not be **biased to its
asserted level**. Declared active-low with a pull-down, or active-high with a pull-up, is a
contradiction between what the design says it wants and what it actually does.

### What it does when it cannot tell

**A netlist states polarity nowhere.** The only structural evidence is a bias resistor, and plenty of
correct designs carry none: a supervisor or PMIC with an internal pull-up drives the reset line by
itself and the schematic shows a bare net.

The rule now **says so** rather than staying quiet. A declared reset with no bias reports an
INCONCLUSIVE finding, and a review item bound to it reads `inconclusive`, never `pass`. The message
names what could not be resolved and what to check by hand.

This used to be silence, and a passing item then meant only "no contradiction found" rather than
"polarity confirmed", a distinction that had to be carried in prose here, in the declaration comment
and in a test name, and that anyone reading a green report would never see. It is now in the report
itself.

A **divider** reports inconclusive too, with a different message: two resistors hold the line at an
intermediate level rather than at either rail, so which level the receiver reads depends on the
ratio against its input thresholds. Telling you "no bias" on a board that visibly has two resistors
would send you looking for the wrong thing.

Its sibling `property-ac-coupled` is decidable by looking and carries no inconclusive case.

### For hardware engineers

An **active-low** reset (usually drawn `RESET_N`, `nRST`, `RST#`) holds the part in reset while the
line is LOW and lets it run when the line is HIGH. So the resting state must be high, and a
pull-up to the rail provides it.

Put a pull-down on that line instead and the part is held in reset from the moment power comes up.
The board looks dead, and the cause is one resistor to the wrong net, a bring-up failure that reads
as a broken part or bad firmware for as long as it takes someone to meter the reset pin.

Active-high is the mirror image: it should rest low, so a pull-up holds it permanently asserted.

### What counts as bias

A resistor on the declared net whose other end reaches a power rail (pull-up) or a ground net
(pull-down).

A net with **both**, a divider, reports neither. A divider sets an intermediate level, so it does
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
before moving the resistor, because if the declaration is what is wrong, moving the resistor would create the
bug the rule was reporting.

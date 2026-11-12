## property-reset-polarity

### What it checks

Suppose the design intent declares a net as an active-low (or active-high) reset. That net must not
be **biased to its asserted level**. If a declared active-low reset has a pull-down, or a declared
active-high reset has a pull-up, the design does the opposite of what it says it wants.

### What it does when it cannot tell

**A netlist states polarity nowhere.** The only structural evidence is a bias resistor, and plenty
of correct designs carry none, because a supervisor or PMIC with an internal pull-up drives the
reset line by itself and the schematic shows a bare net.

The rule now **says so** rather than saying nothing. A declared reset with no bias reports an
INCONCLUSIVE finding, and a review item bound to it reads `inconclusive`, never `pass`. The message
names what could not be resolved and what to check by hand.

The rule used to say nothing here. A passing item then meant only "no contradiction found" rather
than "polarity confirmed". That distinction had to be written in prose here, in the declaration
comment and in a test name, and nobody reading a green report would ever see it. It is now in the
report itself.

A **divider** reports inconclusive too, with a different message. Two resistors hold the line at an
intermediate level rather than at either rail. The level the receiver reads then depends on the
ratio against its input thresholds. If the rule said "no bias" on a board that visibly has two
resistors, you would go looking for the wrong thing.

Its sibling `property-ac-coupled` is decidable by looking and has no inconclusive case.

### For hardware engineers

An **active-low** reset (usually drawn `RESET_N`, `nRST`, `RST#`) holds the part in reset while the
line is LOW and lets it run when the line is HIGH. So the resting state must be high, and a
pull-up to the rail provides it.

Put a pull-down on that line instead and the part is held in reset from the moment power comes up.
The board looks dead, and the cause is one resistor to the wrong net. At bring-up the failure reads
as a broken part or bad firmware until someone meters the reset pin.

Active-high is the mirror image, resting low, so a pull-up holds it permanently asserted.

### What counts as bias

A resistor on the declared net counts as bias when its other end reaches a power rail (pull-up) or
a ground net (pull-down).

A net with **both** is a divider. It counts as neither direction and reports inconclusive, as above.
A divider sets an intermediate level, so it does not hold the line at either rail, and calling it a
contradiction would be wrong.

### Declaring it

```yaml
intent:
  nets:
    SYS_RESET_N: {reset: low}
    PHY_ENABLE:  {reset: high}
```

The level is the assertion, and it is required. Without it the rule has nothing to contradict,
so an omitted or misspelled level is rejected at load rather than becoming a rule that
silently never fires.

### Fixing a finding

The bias resistor goes to the wrong net, or the declared polarity is wrong. Check the part's datasheet
before moving the resistor. If the declaration is wrong, moving the resistor would make the board
match it, and the board would then have the bug the rule was reporting.

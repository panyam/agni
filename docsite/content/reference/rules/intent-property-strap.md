---
title: "intent/property-strap"
description: "A boot/config strap net is biased to the OPPOSITE level from the one the design intent declares it should latch."
---

### What it checks

A net the design intent declares as a boot/config strap must not be biased to the **opposite** level
from the one it is meant to latch. Declared `high` with a pull-down, or declared `low` with a pull-up,
means the part will come up configured the other way from what the design intended.

### It is not `property-reset-polarity` under another name

The two read the same evidence and ask inverted questions, which is worth being precise about because
a reviewer who assumes they are the same check will misread both.

`reset-polarity`'s `value` is the level that **asserts** reset. Bias toward it is the defect, bias away
from it is correct, so the rule can only ever catch one of the two ways a reset line goes wrong.

`strap`'s `value` is the level the pin should **latch**. Bias toward it is correct and bias away from
it is the defect, in **either** direction. So this rule catches both mistakes, not one.

### Read this before binding a review item to it

**A pass means "no contradiction found", not "strap confirmed"**, the same caveat
`property-reset-polarity` carries and for the same cause.

Strap pins almost always carry an internal pull, and the standard datasheet instruction is to fit an
external resistor **only for the non-default state**. So a design declaring the default level with no
resistor on that net is correct and extremely common. Firing on absent bias would report a non-defect
on the majority of real straps, so the rule stays silent there.

**It checks direction, not resistance.** "Is this strap resistor an appropriate value" is a real and
separate question (strong enough to hold against leakage, weak enough not to fight an active driver)
and this rule does not answer it. The engine cannot read a component's value as a number yet.

**It does not identify strap pins.** The declaration names the nets. On a design whose strap nets are
not named for their function, which is usual, the pins have to be read off the part's datasheet pinout
and written into the declaration by hand. Nothing here can infer them.

### For hardware engineers

A strap is a resistor that parks a pin at a known level so the part can sample it at reset and pick a
configuration: boot source, bus width, clock ratio, or its own address on a shared bus.

Getting one backwards does not look like a wiring fault. The board powers up, the part runs, and it
runs in the wrong mode. It boots from the wrong flash, or it answers on an address another device on
the same bus is already using, and then two parts drive that bus against each other and the whole
segment is unreliable in a way that looks intermittent rather than wired wrong.

### What counts as bias

A resistor on the declared net whose other end reaches a power rail (pull-up, latches high) or a ground
net (pull-down, latches low). The far end may reach its rail through further passives.

A net with **both**, a divider, reports neither. Some parts use a divider for a tri-level strap, and the
level it selects depends on the ratio of two resistances the engine cannot read, so the rule declines to
guess rather than reporting a direction it does not know.

### Declaring it

```yaml
net_properties:
  - {net: BOOT_MODE0, property: strap, value: high}
  - {net: PHYAD1,     property: strap, value: low}
```

The `value` is the intended latched level and is required: without it the rule has nothing to
contradict, so an omitted or misspelled level is rejected at load rather than becoming a rule that
silently never fires.

### Fixing a finding

Either the strap resistor goes to the wrong net, or the declared configuration is wrong. Check the
part's datasheet strap table before moving the resistor. If the declaration is the thing that is wrong,
moving the resistor creates the bug the rule was reporting.

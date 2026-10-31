---
title: "intent/property-strap"
description: "A boot/config strap net is biased to the OPPOSITE level from the one the design intent declares it should latch."
---

### Remedy

Move the strap resistor to the rail that latches the declared level. The board boots either way, configured as something other than what was intended.

### What it checks

A net the design intent declares as a boot/config strap must not be biased to the **opposite** level
from the one it is meant to latch. Declared `high` with a pull-down, or declared `low` with a pull-up,
means the part will come up configured the other way from what the design intended.

### It is not `property-reset-polarity` under another name

The two read the same evidence and ask inverted questions, and a reviewer who assumes they are the
same check will misread both.

`reset-polarity`'s `value` is the level that **asserts** reset. Bias toward it is the defect, bias away
from it is correct, so the rule can only ever catch one of the two ways a reset line goes wrong.

`strap`'s `value` is the level the pin should **latch**. Bias toward it is correct and bias away from
it is the defect, in **either** direction. So this rule catches both mistakes, not one.

### Read this before binding a review item to it

**A pass means "no contradiction found", not "strap confirmed"**, the caveat
`property-reset-polarity` carried before it began reporting an unbiased line as inconclusive.

Strap pins almost always carry an internal pull, and the standard datasheet instruction is to fit an
external resistor **only for the non-default state**. So a design declaring the default level with no
resistor on that net is correct and extremely common. Firing on absent bias would report a non-defect
on the majority of real straps, so the rule stays silent there.

**It checks resistance only against a band you declare.** "Is this strap resistor an appropriate
value" (strong enough to hold against leakage, weak enough not to fight an active driver) is now
checked, but only where the declaration states `min_ohms` / `max_ohms`. There is no built-in band
and there deliberately will not be one, because a CMOS input with nanoamp leakage is happy past
100k, while a strap an active driver has to override wants a few hundred ohms, so any universal
range the engine invented would flag correct boards. The person declaring the strap holds the
datasheet; the band is theirs to state.

Declare neither bound and the value half simply does not run, and the direction half still does.

**It does not identify strap pins.** The declaration names the nets. On a design whose strap nets are
not named for their function, which is usual, the pins have to be read off the part's datasheet pinout
and written into the declaration by hand. Nothing here can infer them.

### For hardware engineers

A strap is a resistor that parks a pin at a known level so the part can sample it at reset and pick a
configuration such as boot source, bus width, clock ratio, or its own address on a shared bus.

Getting one backwards does not look like a wiring fault. The board powers up, the part runs, and it
runs in the wrong mode. It boots from the wrong flash, or it answers on an address another device on
the same bus is already using, and then two parts drive that bus against each other and the whole
segment is unreliable in a way that looks intermittent rather than wired wrong.

### What counts as bias

A resistor on the declared net whose other end reaches a power rail (pull-up, latches high) or a ground
net (pull-down, latches low). The far end may reach its rail through further passives.

A net with **both**, a divider, reports neither. Some parts use a divider for a tri-level strap, and
the level it selects depends on the ratio of the two resistances rather than on either one, so the
rule declines to guess rather than reporting a direction it does not know. A declared band over a
divider reports inconclusive and names both resistors, since neither is "the" strap pull, so judging
one would name an arbitrary part.

### Declaring it

```yaml
intent:
  nets:
    # direction only
    BOOT_MODE0: {strap: high}
    # direction plus an acceptable pull band, in ohms
    PHYAD1: {strap: low, min_ohms: 1000, max_ohms: 100000}
    # one-sided: bound the weak end only
    BOOT_CFG2: {strap: high, max_ohms: 47000}
```

The `strap` level is the intended latched level and is required, because without it the rule has nothing to
contradict, so an omitted or misspelled level is rejected at load rather than becoming a rule that
silently never fires.

`min_ohms` and `max_ohms` are optional and independent, so a one-sided band checks one side only.
Both are rejected at load on a property kind that has no resistance to bound, and a `min_ohms` above
its `max_ohms` is rejected too, because a band nothing can satisfy would compile to a check that can
never pass, which is a declaration error rather than a design finding.

A declared band over a resistor with no value, a value that does not parse, or a value in another
unit reports **inconclusive** and names the resistor. Firing on a number the engine could not read
would report a defect on no evidence, and passing would claim a comparison nobody made, so the
finding says the band was not checked. A wrong-way strap still fails, whatever its value reads as.
With no strap resistor on the net, a declared band has nothing to bound and the rule stays silent,
for the default-state reason above. An unfitted resistor counts as absent.

### Fixing a finding

An inconclusive finding asks for a readable value on the named resistor, or a look at the divider by
hand. Otherwise, either the strap resistor goes to the wrong net, or the declared configuration is wrong. Check the
part's datasheet strap table before moving the resistor. If the declaration is wrong,
moving the resistor creates the bug the rule was reporting.

## property-strap

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

**It checks resistance only against a band you declare.** "Is this strap resistor an appropriate
value" (strong enough to hold against leakage, weak enough not to fight an active driver) is now
checked, but only where the declaration states `min_ohms` / `max_ohms`. There is no built-in band and
there deliberately will not be one: a CMOS input with nanoamp leakage is happy past 100k, while a
strap an active driver has to override wants a few hundred ohms, so any universal range the engine
invented would flag correct boards. The person declaring the strap holds the datasheet; the band is
theirs to state.

Declare neither bound and the value half simply does not run, and the direction half still does.

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

A net with **both**, a divider, reports neither. Some parts use a divider for a tri-level strap, and
the level it selects depends on the ratio of the two resistances rather than on either one, so the
rule declines to guess rather than reporting a direction it does not know. The value check declines
there for the same reason: neither resistor is "the" strap pull, so judging one would name an
arbitrary part.

### Declaring it

```yaml
net_properties:
  # direction only
  - {net: BOOT_MODE0, property: strap, value: high}
  # direction plus an acceptable pull band, in ohms
  - {net: PHYAD1, property: strap, value: low, min_ohms: 1000, max_ohms: 100000}
  # one-sided: bound the weak end only
  - {net: BOOT_CFG2, property: strap, value: high, max_ohms: 47000}
```

The `value` is the intended latched level and is required: without it the rule has nothing to
contradict, so an omitted or misspelled level is rejected at load rather than becoming a rule that
silently never fires.

`min_ohms` and `max_ohms` are optional and independent, so a one-sided band checks one side only.
Both are rejected at load on a property kind that has no resistance to bound, and a `min_ohms` above
its `max_ohms` is rejected too: a band nothing can satisfy would compile to a check that can never
pass, which is a declaration error rather than a design finding.

The value check runs only when a bound is declared AND the pull resistor's value reads as a
resistance. A resistor with no value, an unparseable one, or one stamped in another unit is skipped.
Unreadable is not the same as acceptable, and firing on a number the engine could not read would
report a defect on no evidence.

### Fixing a finding

Either the strap resistor goes to the wrong net, or the declared configuration is wrong. Check the
part's datasheet strap table before moving the resistor. If the declaration is the thing that is wrong,
moving the resistor creates the bug the rule was reporting.

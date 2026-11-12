## property-strap

### What it checks

The design intent can declare a net as a boot/config strap. That net must not be biased to the
**opposite** level from the one it is meant to latch. Declared `high` with a pull-down, or declared
`low` with a pull-up, means the part will come up configured the other way from what the design
intended.

### It is not `property-reset-polarity` under another name

The two read the same evidence and ask inverted questions. If a reviewer assumes they are the same
check, the reviewer will misread both.

`reset-polarity`'s `value` is the level that **asserts** reset. Bias toward it is the defect, bias away
from it is correct, so the rule can only ever catch one of the two ways a reset line goes wrong.

`strap`'s `value` is the level the pin should **latch**. Bias toward it is correct and bias away from
it is the defect, in **either** direction. So this rule catches both mistakes, not one.

### Read this before binding a review item to it

**A pass means "no contradiction found", not "strap confirmed"**. `property-reset-polarity` carried
the same caveat until it began reporting an unbiased line as inconclusive.

Strap pins almost always carry an internal pull, and the standard datasheet instruction is to fit an
external resistor **only for the non-default state**. So a design may declare the default level and
fit no resistor on that net. That design is correct, and the case is extremely common. Firing on
absent bias would report a non-defect on the majority of real straps, so the rule says nothing there.

**It checks resistance only against a band you declare.** The rule now asks whether a strap
resistor has an appropriate value, strong enough to hold against leakage and weak enough not to
fight an active driver. It asks only where the declaration states `min_ohms` / `max_ohms`. There is
no built-in band and there deliberately will not be one. A CMOS input with nanoamp leakage is happy
past 100k. An active driver has to override some straps, and those want a few hundred ohms. So any
universal range the engine invented would flag correct boards. The person declaring the strap holds
the datasheet, so the band is theirs to state.

Declare neither bound and the value half simply does not run, and the direction half still does.

**It does not identify strap pins.** The declaration names the nets. Strap nets are usually not named
for their function. On such a design the pins have to be read off the part's datasheet pinout and
written into the declaration by hand. Nothing here can infer them.

### For hardware engineers

A strap is a resistor that parks a pin at a known level so the part can sample it at reset and pick a
configuration such as boot source, bus width, clock ratio, or its own address on a shared bus.

Getting one backwards does not look like a wiring fault. The board powers up, the part runs, and it
runs in the wrong mode. It boots from the wrong flash. Or it answers on an address that another
device on the same bus already uses, and then two parts drive that bus against each other. The whole
segment is then unreliable in a way that looks intermittent rather than wired wrong.

### What counts as bias

A resistor on the declared net counts as bias when its other end reaches a power rail (pull-up,
latches high) or a ground net (pull-down, latches low). The far end may reach its rail through
further passives.

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

The `strap` level is the intended latched level, and it is required. Without it the rule has nothing
to contradict. So an omitted or misspelled level is rejected at load, rather than becoming a rule
that silently never fires.

`min_ohms` and `max_ohms` are optional and independent, so a one-sided band checks one side only.
Both are rejected at load on a property kind that has no resistance to bound. A `min_ohms` above its
`max_ohms` is rejected too. No value can satisfy that band, so it would compile to a check that can
never pass. That is a declaration error rather than a design finding.

A declared band over a resistor with no value, a value that does not parse, or a value in another
unit reports **inconclusive** and names the resistor. Firing on a number the engine could not read
would report a defect on no evidence, and passing would claim a comparison nobody made, so the
finding says the band was not checked. A wrong-way strap still fails, whatever its value reads as.
With no strap resistor on the net, a declared band has nothing to bound and the rule says nothing,
for the default-state reason above. An unfitted resistor counts as absent.

### Fixing a finding

An inconclusive finding asks for a readable value on the named resistor, or a look at the divider by
hand. Otherwise, either the strap resistor goes to the wrong net, or the declared configuration is
wrong. Check the part's datasheet strap table before moving the resistor. If the declaration is
wrong, moving the resistor creates the bug that the rule was reporting.

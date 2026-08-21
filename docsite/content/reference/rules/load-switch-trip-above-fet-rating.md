---
title: "load-switch-trip-above-fet-rating"
description: "A controller-based load switch trips above the continuous drain rating of its external MOSFET."
---

### Remedy

Lower the switch's current-limit setting below the FET's continuous drain rating, or fit a FET rated above the trip point. As drawn, the limit protects nothing.

### What it checks

A load switch built from a **switch controller** plus an **external MOSFET** plus an **external sense
resistor**, where the current at which the controller trips is higher than the continuous current the
MOSFET is rated to carry.

Both numbers are vendor values and the finding cites the page each came from. The third input, the
sense resistance, comes from the design, because no datasheet knows it.

### For hardware engineers

An integrated load switch is one part. Its current limit and its pass element are specified together
by one vendor, in one document, and they already agree.

A controller-based switch is three parts chosen by three separate decisions:

- The **controller** states a sense threshold, a voltage such as 50mV. That is all it states. It has
  no current limit of its own, because it has no idea what shunt you will put in front of it.
- The **sense resistor** turns that threshold into a current. `ITRIP = V(OCP) / Rsns`. A 50mV
  threshold across 10mΩ trips at 5A; across 50mΩ it trips at 1A. Same controller, five times the
  current.
- The **MOSFET** carries that current, and has its own continuous drain rating.

Nothing in the design flow checks that the third number is bigger than the first two put together.
When it is not, the current limit does nothing useful: the FET reaches its own rating while the
controller is still comfortably below its trip point, so the part the protection existed for is the
part that dies. And a high-side MOSFET that dies usually dies **short**, which puts the whole rail on
the load it was switching.

The effective on-resistance of such a switch is likewise the external FET's RDS(on), not any number
on the controller's sheet. The finding reports it, because "what does the FET dissipate at the trip
current" is the reviewer's next question.

### What derating this does and does not cover

The rule compares against the FET's **printed** rating, which is a 25C figure. Real designs size well
under it. A trip point that sits below the printed rating therefore passes here and may still be
badly derated.

That is the unambiguous half on purpose. "The current limit is set above the vendor's own number" is
a defect nobody argues with. "The current limit is set at 80% of the vendor's number and the part is
in still air behind a connector" is a thermal argument this rule does not attempt.

### How the three parts are found

- **The FET** is a transistor whose **gate** terminal is on exactly one net. The role comes from the
  naming lexicon's role vocabulary, so a house that calls its gate `DRV` declares that in
  `--conventions` rather than patching the engine.
- **The controller** is the single non-transistor part on that gate net whose datasheet states an
  overcurrent threshold. Identified by what its sheet says, not by a device-class keyword: a part that
  declares a sense threshold is a current-limiting controller, whatever its description string reads.
  This also settles integrated-versus-controller for free, since an integrated switch has no external
  gate net to be found from.
- **The sense resistor** is the single resistor **every** one of whose nets the controller also
  touches. That is Kelvin sensing expressed structurally: a shunt is measured by two dedicated pins
  landing on its two terminals. A series part that merely sits nearby has a far terminal the
  controller does not touch.

A feedback or programming divider between a controller's output and its sense pin shares that
structural signature. Magnitude separates them by orders of magnitude, so the resolver accepts only a
resistance of **1Ω or less**: a shunt dropping tens of millivolts at amperes is milliohm-class, a
divider that must not waste current is kilohms, and one ohm sits in the empty middle.

### When it stays silent

Silence is always "I could not tell", never "this is fine".

- **No seeded datasheet set.** `check.Available` gates the rule to not-applicable without `--params`.
- **The gate net carries two candidate controllers**, or the controller has **two candidate shunts**.
  Unresolvable, so no verdict rather than a verdict computed from the wrong part.
- **The FET's gate lands on more than one net**, or it declares no gate pin at all.
- **The controller states no overcurrent threshold**, or states it in a unit the parameter layer does
  not recognize. A millivolt row is reduced to volts and compared, which is how real controller sheets
  print this row; a unit with no entry in the conversion table is skipped rather than scaled by a
  guess, so the rule still fails toward silence instead of toward a current a thousand times too large.
- **The FET is unseeded** or states no continuous drain rating. Pulsed drain current is deliberately
  not accepted in its place.
- **The shunt's value is not stated in ohms in the design.** A component whose value the reader never
  normalized, or normalized without a unit, is no evidence that it is a milliohm shunt. This is the
  live limit on formats: KiCad, IPC-2581 and gEDA normalize the value attribute at ingestion; EDIF and
  xschem do not, so an EDIF design's shunt commonly carries no readable number and the rule reports
  nothing there.
- **A zero-ohm shunt.** Dividing by it gives infinity, which every comparison downstream would read as
  an enormous current and report as a defect.

### For software engineers

Two joins that had no prior arithmetic between them. `check.ExternalFetLoadSwitches` resolves the
topology and the trip current; the rule adds the rating comparison.

The trip current is the first calculation anywhere in the engine that crosses two units. It goes
through `check.OhmsLawCurrent(volts, ohms) (amps, ok)`, a named physical operation rather than a
general dimension algebra: every other consumer compares within one unit, which the accessors already
gate, so a dimension system would be a lot of machinery with no second caller. The signature states
the unit contract, and `ok` is false for a non-positive or non-finite resistance.

`DatasheetProv` carries exactly two citations, the FET's rating first (the endangered part) and the
controller's threshold second. The on-resistance is quoted in the message with an inline citation but
is **not** in `DatasheetProv`, because the verdict does not rest on it and the review's data-trust gate
rates a finding by its weakest citation.

### Fixing a finding

Three levers, and they are not equivalent:

1. **Raise the shunt's resistance** to bring the trip current down. Cheapest, and it costs a little
   more dissipation in the shunt.
2. **Choose a FET with a higher continuous rating.** Right answer when the load genuinely needs the
   current.
3. **Choose a controller with a lower threshold.** Rarely the lever, since the threshold is usually
   fixed by the part.

Check which of the three numbers was the intended one before changing any of them. The rule reports
the FET because the FET carries the rating being exceeded, not because the FET is necessarily the
wrong part.

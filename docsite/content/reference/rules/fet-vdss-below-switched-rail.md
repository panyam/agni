---
title: "fet-vdss-below-switched-rail"
description: "A MOSFET sits on a rail at or above its datasheet drain-source breakdown voltage."
---

### What it checks

A MOSFET sitting on a power rail whose voltage is at or above the part's datasheet **drain-source
breakdown** rating (VDSS). Both numbers are vendor values where the rail's voltage comes from a
datasheet, and the finding cites each one it used.

### For hardware engineers

VDSS is the voltage a FET can block across drain and source while it is off. Past it the part stops
being a switch. It avalanches, conducts when it is meant to block, and often fails **short**.

The short is what makes this worth an error rather than a warning. An open failure disconnects the
load, which is visible and usually harmless. A shorted high-side switch hands the full rail straight
to whatever it was protecting, so a part chosen to protect the load becomes the thing that destroys
it. A 30V-rated FET on a 48V rail is not marginal; it is outside the envelope the vendor will stand
behind.

Rating a switch above the rail is also not the whole story in a real design — inductive kickback and
hot-plug transients push the drain well above the nominal rail, which is why designers derate. This
rule only catches the case where the part is under-rated against the **steady-state** rail, which is
the unambiguous half.

### Where the rail voltage comes from, and why the message says so

Two sources, preferred in this order:

1. **A driving part's datasheet output.** If something on the rail declares an output voltage in its
   spec, that is a vendor value. It earns its own citation, and the review's data-trust gate weighs
   it alongside the FET's.
2. **The net's name.** `+5V` means 5V by convention. That is a design convention, not a document, so
   it gets no citation.

The message names which was used. `5V` read from a datasheet and `5V` inferred from a net name are
not equally trustworthy, and a report that flattened them would be overstating what it knows.

### Precision limit worth knowing

VDSS is a rating on the **drain-source** pair specifically, but the engine's pin-role vocabulary has
no drain or source — only power, ground, and the two diode terminals. So the rule cannot tell which of
the FET's nets actually sits across those terminals.

It therefore compares against every **rail** the part touches, and reports the highest. Rails are the
right filter rather than every net: a gate-drive net is not a rail, so the obvious false pairing is
excluded structurally rather than by luck.

The residual case is a gate deliberately tied to a rail (an always-on FET, or a P-FET gate pulled to
its source). There the binding limit is VGSS, not VDSS, and VGSS is usually far lower — so the
condition is typically still a defect, but this rule would name the wrong parameter for it. WS3-117
(FET pin roles in the naming lexicon) is what makes this exact.

### For software engineers

A join between two projections that were read independently before: the part's breakdown rows
(`FetBreakdownLimits`) and the rail's voltage, resolved from either a driving part's
`OutputVoltageLimits` or `RailMaxVoltage`'s name-derived nominal.

Where several rows are comparable the rule takes the **lowest** breakdown (a part is endangered at
its weakest rating) against the **highest** rail voltage. Any other pairing under-reports.

`DatasheetProv` carries one citation or two depending on the rail's evidence, which is what the plural
citation field from WS3-028 is for: the data-trust gate rates a finding by its weakest citation, so a
rail voltage from a low-confidence extraction correctly drags the whole finding to Provisional.

### When it stays silent

- **No seeded datasheet set.** The rule reads the params tier, so `check.Available` gates it to
  not-applicable without `--params`. Unevaluable, never clean.
- **The FET is unseeded**, or its spec carries no breakdown row — skip, not pass.
- **The rail's voltage is unknown**: no driving part declares an output and the name carries no
  voltage token. A rail named `VSYS` yields no number, and the rule does not guess one.
- **Ground.** Excluded explicitly: it is a rail by the engine's definition but carries no voltage to
  compare.

### Fixing a finding

Either the FET is under-rated for where it sits, or it is on the wrong rail. Check the intent before
swapping the part: the rule reports the FET because that is the part with the rating, but a switch
wired onto a rail it was never meant to see is a netlist error, not a part-selection one.

---
title: "regulator-output-exceeds-abs-max"
description: "A regulator's datasheet output voltage exceeds the absolute-maximum supply rating of a part it feeds."
---

### Remedy

Reprogram the regulator's output to a voltage the downstream part is rated for, or move that part to a rail that already is. Both numbers come from vendor documents, so a design change settles this without needing a measurement.

### What it checks

A regulator's datasheet **output** voltage against the **absolute-maximum supply** rating of a part it
feeds. Both numbers come from vendor documents, and the finding cites both.

It is the first rule that compares a parameter on one part against a parameter on another part across
the net joining them. Every datasheet rule before it read one spec against one rail.

### For hardware engineers

A regulator makes a rail. Everything on that rail is exposed to whatever the regulator puts out. If a
3.3V part sits on a rail a 5V regulator drives, the part is outside its absolute maximum from the
moment power comes up, which is outside the envelope the vendor will stand behind rather than merely
marginal.

Absolute maximum is a stress limit, not an operating range. Exceeding it can destroy the part
immediately, or damage it enough to fail months later in the field, and the second costs far more.

The check follows the supply one series element out, so a ferrite bead or a series resistor between
the regulator and its load does not hide the connection. It deliberately does not follow further,
because unlike a surge, voltage does not fall off along a supply path, so a wider walk would make
every part on the board look connected to every regulator and the rule would start pairing parts
that share no supply at all.

### Why it is not the same as supply-exceeds-abs-max

`supply-exceeds-abs-max` asks a similar question, but it takes the rail voltage from the **net name**, where
`+5V` means 5V. That is a naming convention, and a convention can be silent or wrong.
A rail named `VOUT_A` carries no voltage in its name at all, and a rail named `+5V` that became
3.3V in a respin still reads as 5V.

This rule reads the number off the regulator's own datasheet instead, asking the same question on
evidence rather than convention.

Neither replaces the other. A rail fed by something with no seeded datasheet, a connector or an
unseeded module, still has only its name to go on, so the older rule keeps its job.

### For software engineers

The rule joins two projections that were previously read independently, the part's supply-limit rows
(`SupplyAbsMaxLimits`) and the driving part's output rows (`OutputVoltageLimits`), connected by net
membership within `check.SupplyPathReachHops`.

Where several rows are comparable the rule picks the extreme in the endangering direction, meaning the
**highest** output the source can present, against the **lowest** absolute maximum the load declares.
An adjustable regulator endangers its load at the top of its range, and a part is endangered at its
weakest rating, so any other pairing would under-report.

The finding carries **both** citations, and `Finding.DatasheetProv` is a slice because the review's
data-trust gate rates a finding by its weakest citation, and with a single slot a conclusion resting
half on a low-confidence extraction would still have counted as a hard Fail.

### When it does not compare

- **The rule needs a seeded datasheet set.** It reads the params tier, so `check.Available` gates
  it to not-applicable without `--params`. An unseeded design reads unevaluable, never clean.
- **Both parts need a seeded spec.** A load with no spec, or with no absolute-maximum supply row, is
  reported not-considered with the reason. A regulator with no spec is not treated as a source.
- **Under-specified rows are not compared.** A parameter with text-only conditions or no max bound is
  skipped rather than coerced (see `architecture/datasheet-layer.md`). A row printed in a prefixed unit
  (mV, kV) is not in that category since agni issue 148, because the parameter layer reduces it
  to volts through its one conversion table, and only a unit that table does not recognize is skipped.

### Fixing a finding

Either the wrong regulator is feeding the rail, or the wrong part is on it. Check which of the two the
design intended before changing a value: the rule reports the load as the subject because that is the
part at risk, but the defect is as often the supply.

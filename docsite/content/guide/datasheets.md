---
title: "Datasheets"
description: "Give the tool a part's real limits as data, and it checks every design that uses the part against them."
---

Most rules only need your design. A few can also check it against a **part's real limits**
from its datasheet, once you give the tool those limits as data. This page turns that on.

Transcribe a part's limits once (the Absolute Maximum Ratings, the operating range) into a
small file, and the tool compares every design that uses the part against them. See
"datasheets as checkable data" in [Concepts](../concepts/).

## Load a parameter set

A parameter set is a directory of small text files, one per part, each holding that part's
specs. Point `check` at it with `--params`:

```
agni check regulator.fires.kicad_sch --params ./params/ --rule supply-exceeds-abs-max
```

```
findings by rule:
  supply-exceeds-abs-max 1

first 1:
  [error] supply-exceeds-abs-max: U1 (power-input pin 1 on rail "+24V": nominal 24V
  exceeds absolute-maximum VIN 20V — datasheet "SNOS412Q - FEBRUARY 2000 - REVISED
  JANUARY 2023" page 4, "7.1 Absolute Maximum Ratings" (hand, confidence 1))
```

Read that finding: the design drives a `+24V` rail into `U1` pin 1, whose datasheet caps
VIN at 20V absolute maximum. The message carries a **dual citation**, your design side
(the rail and pin) and the datasheet side (document, page, and the exact table). You can
open the datasheet to page 4 and confirm it.

For the tool to match a datasheet to a placed part, the design has to name the part: an MPN
on the BOM line, or the MPN/Manufacturer properties a schematic symbol carries.

## What the tool will and will not auto-compare

The tool is deliberately conservative about when it compares a number automatically.

- A limit stated as a plain number the tool can act on (VIN abs-max = 20V) is
  **machine-comparable** and can fire a finding.
- A limit that only holds under a **text condition** the tool cannot evaluate ("20V at 25°C
  ambient, derate above") is shown to a human rather than auto-compared. The tool will not
  pretend to a certainty the datasheet did not give it.
- A part whose spec is missing the fields a rule needs is **under-specified** and is skipped,
  not guessed.

This is why an empty or partial parameter set makes datasheet rules go quiet rather than
wrong: no data means the rule had nothing to compare, the same tier logic as everywhere else.

## Confidence and provenance

Each spec value records where it came from and how much to trust it. A limit typed in by a
person reads as the highest confidence. A value extracted automatically from a PDF carries a
lower confidence and its own page/table citation. The finding message surfaces this (the
`(hand, confidence 1)` tail above), so a reviewer can weigh a machine-extracted limit
differently from a hand-verified one.

## Where the specs come from

You author a parameter set by transcribing the limits you care about (facts from a datasheet
are not copyrightable, so cite the document revision and page). There is also a pipeline that
extracts specs from a PDF automatically, which is a separate tool covered in the developer
docs. Either way the result is the same small per-part files this page loads.

## Where to go next

- [Checks and reports](../checks-and-reports/): the general report-reading flow these
  findings appear in.
- [CLI reference](../cli-reference/): `--params` and the other flags.

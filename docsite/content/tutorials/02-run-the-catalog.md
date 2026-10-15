---
title: "2. Run the catalog"
description: "The built-in rules, how to read a finding, and how to fail a build on one."
---

`agni check` runs the shipped rule catalog over one design. These are the general rules,
true of most boards regardless of whose board it is. Your team's own rules come later, in
rungs 4 through 8. Run it from the board's folder, as in [rung 1](../01-read-a-design/).

## Run it

{{ agniRun "content/tutorials/runs/02-check-jetson.yaml" }}

## Reading one finding

Take `[error] i2c-pull-up: /SoM_IO/I2C0_SCL_1V8 (I2C net has no pull-up resistor to a rail)`. It has
four parts.

The **severity** is `error`. The **rule** is `i2c-pull-up`. The **subject** is the net
`/SoM_IO/I2C0_SCL_1V8`, naming the specific thing on your board the rule is talking about, with the
sheet it was drawn on as a prefix. The **reason** in parentheses says what is wrong in plain language.

Severity is a policy signal, not a confidence signal. An `error` is something that will almost
certainly not work, such as an I2C bus with no {{ explainable "pull-up" }}, which cannot signal at all
because the parts on it can only pull the line down and nothing pulls it back up. A `warning` is
something that usually indicates a defect. An `info` is worth a look. None of them is a statement
about how sure the tool is.

Now look at the net itself. It joins three pins, one on the module connector J1, one on R357, and
one on U1, an NTS0104 level translator. R357 is a 2.2 kOhm pull-up, and the schematic marks it
do-not-populate, so the rule does not count it. The translator carries 10 kOhm pull-ups of its own
inside the package, which is a fact from its datasheet and appears nowhere in the netlist. So the
finding is right about what the drawing shows, and the board most likely works. Closing that gap is
what [rung 6](../06-part-limits/) is for, where a datasheet fact becomes something a check can read.

Most findings on a real board read like this one. The rule is correct about the netlist, and
whether it matters depends on something the netlist does not carry.

## Where a finding came from

The text form is a summary. `--format json` gives you the whole thing:

```
agni check . --format json
```

```json
{
  "rule": "i2c-pull-up",
  "severity": "error",
  "subject": {
    "kind": "net",
    "ref": "/SoM_IO/I2C0_SCL_1V8",
    "pin": "",
    "netId": "93b92cc31d78",
    "busId": ""
  },
  "message": "I2C net has no pull-up resistor to a rail",
  "inconclusive": false,
  "provenance": {
    "sourceFile": "jetson-agx-thor-baseboard.kicad_sch",
    "span": null,
    "nativeId": "",
    "nativeIdKind": ""
  },
  "sheets": [
    "/SoM_IO"
  ],
  "locateReason": "LOCATE_REASON_UNSPECIFIED",
  "datasheets": [],
  "context": []
}
```

`provenance` points back at the file the design was read from, and `sheets` names the sheet the net
is drawn on, which is how you get from a finding to the thing in your CAD tool. `datasheets` is
empty here and carries the page and table a datasheet-backed finding rests on, which rung 6 covers.
`inconclusive` marks a check that ran but could not decide, which matters enough to get its own rung
later.

Two other formats are useful early. `--format markdown` organizes by severity, worst first, for
pasting into a review. `--format report` is the long form with each rule's full explanation.

## Narrowing

Run one rule while you work on it:

```
agni check . --rule i2c-pull-up
```

Or a whole category:

```
agni check . --tag category=power
```

## Failing a build

`--fail-on` makes `check` exit non-zero when anything at or above a severity is present, which is
all you need to put it in CI:

{{ agniRun "content/tutorials/runs/02-gate-fails.yaml" }}

The board has thirty `error` findings, all of them `i2c-pull-up`, so the command fails. A rule with
no errors passes the same gate:

{{ agniRun "content/tutorials/runs/02-gate-passes.yaml" }}

Starting at `--fail-on error` is the practical choice. It gates on the things that will not work at
all, which almost nobody argues with, and it lets you tighten to `warning` later once the backlog is
clear. On a board like this one the first job is reading those thirty, because as the finding above
showed, an error can be right about the netlist and still not be a defect.

## What the run says it looked at

Look at the last line of that first run again. Every run ends with it, whether or not it found
anything:

```
24118 subject(s) considered by 35 rule(s), 625 not considered (--verdicts for the detail)
```

This is the half a findings list cannot give you. A run that finds nothing and a run whose rules all
examined the wrong thing produce an identical list of findings, namely none, so the findings alone
can never tell you which one you are holding.

Read the three numbers separately. **24118 considered** is how many subjects were actually judged.
**35 rules** is how many were willing to say what they looked at, which is not the same as how many
ran, because most of the catalog has no subject in scope on any given board, and a rule with nothing
to say is not a gap. **625 not considered** is the one worth reading closely, and it gets its own
look below.

A second line can appear under that one:

```
2 rule(s) reported violations without stating what they examined, so silence from those is not evidence of anything
```

It counts rules that found something and never said what they looked at, so silence from them means
nothing and the coverage number does not cover them. It is printed whenever the count is above zero.
On this board it reads zero and is omitted, and so it does for the sub-sheet read in rung 1. You will
see it on a run that includes rules of your own written without a considered set, and when it
appears it is the limit of the coverage claim above it.

That line is the claim. `--verdicts` is the evidence, one row per subject with passes included:

{{ agniRun "content/tutorials/runs/02-verdicts.yaml" }}

Now the pass is checkable. It names C10 on `+3V3` and C241 on `VCC`, so you can open the schematic
and confirm that those capacitors really are on those rails. Delete one next revision and this output
changes, where the findings-only view would print the same nothing before and after.

The one fail is `Net-(D1-C)`, a net KiCad named after a pin on it, diode D1's cathode, because nobody
labelled it. It is the module's
backup-battery input, fed through D1 and R6, and the module decouples it on its own side of the
connector. That is the same lesson as the I2C finding, met from the other direction.

Across the whole catalog that is a much larger table than the findings list, which is why the summary
is the default and the rows are a flag:

{{ agniRun "content/tutorials/runs/02-verdicts-summary.yaml" }}

`not-considered` is the third outcome, and a findings list has no counterpart to it. It means the
rule was willing to judge that subject and something stopped it, so it says what stopped it rather
than passing on incomplete evidence.

On this board most of the 625 are one of three kinds. 319 are `netclass-via-drill` declining nets
that carry no via, which have no drill to compare. 156 are `floating-input` declining a net that
carries a passive part, because a resistor on a net might be the pull that fixes it, might be a series
element with the driver on the far side, or might be a footprint nobody stuffed, and a netlist cannot
tell those apart. 111 are `rail-not-classified` declining a net whose name declares a voltage while no
power-input pin on it confirms the net is a rail. None of those is something a datasheet fills,
because they mark where the check's reach ends.

That distinction between "checked and fine", "never checked" and "could not tell" runs through the
whole tool, and [rung 9](../09-read-the-verdicts/) is about reading it.

## Next

[See it](../03-see-it/), because a list of net names is not how anyone thinks about a board.

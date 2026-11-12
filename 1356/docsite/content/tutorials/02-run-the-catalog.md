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
`/SoM_IO/I2C0_SCL_1V8`. It names the specific thing on your board that the rule is about, and its
prefix names the sheet where the net is drawn. The **reason** in parentheses says what is wrong in
plain language.

Severity is a policy signal, not a confidence signal. An `error` is something that will almost
certainly not work, such as an I2C bus with no {{ explainable "pull-up" }}, which cannot signal at all
because the parts on it can only pull the line down and nothing pulls it back up. A `warning` is
something that usually indicates a defect. An `info` is worth a look. None of them is a statement
about how sure the tool is.

Now look at the net itself. It joins three pins, one on the module connector J1, one on R357, and
one on U1, an NTS0104 level translator. R357 is a 2.2 kOhm pull-up, and the schematic marks it
do-not-populate, so the rule does not count it. The translator carries 10 kOhm pull-ups of its own
inside the package. That fact comes from its datasheet and appears nowhere in the netlist. So the
finding is right about what the drawing shows, and the board most likely works.
[Rung 6](../06-part-limits/) closes that gap by turning a datasheet fact into something a check can
read.

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

`provenance` points back at the design's source file, and `sheets` names the sheet where the net is
drawn. Together they take you from a finding to the thing in your CAD tool. `datasheets` is empty
here. On a datasheet-backed finding it carries the cited page and table, and rung 6 covers it.
`inconclusive` marks a check that ran but could not decide. That matters enough to get its own rung
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

`--fail-on` makes `check` exit non-zero when anything at or above a severity is present. That flag
is all you need to put it in CI:

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
24117 subject(s) considered by 35 rule(s), 615 not considered (--verdicts for the detail)
```

A findings list cannot give you this half. Picture two runs. One finds nothing. In the other, every
rule examined the wrong thing. Both produce an identical list of findings, namely none, so the
findings alone can never tell you which one you are holding.

Read the three numbers separately. **24117 considered** is how many subjects were actually judged.
**35 rules** is how many were willing to say what they looked at. That is not the same as how many
ran. Most of the catalog has no subject in scope on any given board, and a rule with nothing to say
is not a gap. **615 not considered** is the one worth reading closely, and it gets its own
look below.

A second line can appear under that one:

```
2 rule(s) reported violations without stating what they examined, so silence from those is not evidence of anything
```

It counts rules that found something and never said what they looked at, so silence from them means
nothing and the coverage number does not cover them. It is printed whenever the count is above zero.
On this board it reads zero and is omitted, and the same happens for the sub-sheet read in rung 1.
You will see it on a run that includes rules of your own written without a considered set. When it
appears, it marks the limit of the coverage claim above it.

That line is the claim. `--verdicts` is the evidence, one row per subject with passes included:

{{ agniRun "content/tutorials/runs/02-verdicts.yaml" }}

Now the pass is checkable. It names C10 on `+3V3` and C241 on `VCC`, so you can open the schematic
and confirm that those capacitors really are on those rails. Delete one next revision and this output
changes, where the findings-only view would print the same nothing before and after.

Every row passes here, so the fail column reads nothing. A net whose only supply pin is on a
connector would have no row at all, because the rule leaves it out rather than passing it, and the
count of rows tells the two apart.

Across the whole catalog, that table is much larger than the findings list. So the summary is the
default and the rows are a flag:

{{ agniRun "content/tutorials/runs/02-verdicts-summary.yaml" }}

`not-considered` is the third outcome, and a findings list has no counterpart to it. It means the
rule was willing to judge that subject and something stopped it, so it says what stopped it rather
than passing on incomplete evidence.

On this board most of the 615 are one of three kinds. 319 are `netclass-via-drill` declining nets
that carry no via, since such a net has no drill to compare. 154 are `floating-input` declining a
net that carries a passive part, because a resistor on a net might be the pull that fixes it, might
be a series element with the driver on the far side, or might be a footprint nobody stuffed, and a
netlist cannot tell those apart. 111 are `rail-not-classified` declining a net whose name declares a
voltage while no power-input pin on it confirms the net is a rail. A datasheet fills none of those,
because they mark where the check's reach ends.

The whole tool separates "checked and fine", "never checked" and "could not tell", and
[rung 9](../09-read-the-verdicts/) is about reading that distinction.

## Next

[See it](../03-see-it/), because a list of net names is not how anyone thinks about a board.

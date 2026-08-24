---
title: "2. Run the catalog"
description: "The built-in rules, how to read a finding, and how to fail a build on one."
---

`agni check` runs the shipped rule catalog over one design. These are the general rules, the ones
that are true of most boards regardless of whose board it is. Your team's own rules come later, in
rungs 4 through 8.

## Run it

{{ agniRun "content/tutorials/runs/02-check-designs-gateway-gateway-edn.yaml" }}

## Reading one finding

Take `[error] i2c-pull-up: I2C_SCL (I2C net has no pull-up resistor to a rail)`. It has four parts.

The **severity** is `error`. The **rule** is `i2c-pull-up`. The **subject** is the net `I2C_SCL`,
naming the specific thing on your board the rule is talking about. The **reason** in parentheses
says what is wrong in plain language.

Severity is a policy signal, not a confidence signal. An `error` is something that will almost
certainly not work: an I2C bus with no {{ explainable "pull-up" }} cannot signal at all, because the parts on it can
only pull the line down and nothing pulls it back up. A `warning` is something that usually
indicates a defect. An `info` is worth a look. None of them is a statement about how sure the tool
is.

`profile/can-esd-missing` fires alongside `esd-protection` on the same two nets. That is not a
duplicate. The general rule notices any externally exposed signal with no protection. The CAN
profile knows those two nets are a CAN bus and applies what CAN specifically requires. Rung 5 is
about that second kind.

## Where a finding came from

The text form is a summary. `--format json` gives you the whole thing:

```
agni check designs/gateway/gateway.edn --format json
```

```json
{
  "rule": "i2c-pull-up",
  "severity": "error",
  "subject": {
    "kind": "net",
    "ref": "I2C_SCL",
    "netId": "601209543ef5"
  },
  "message": "I2C net has no pull-up resistor to a rail",
  "inconclusive": false,
  "provenance": {
    "sourceFile": "designs/gateway/gateway.edn",
    "nativeId": "I2C_SCL",
    "nativeIdKind": "edif-rename-id"
  },
  "sheets": ["graph"],
  "datasheets": []
}
```

`provenance` points back at the file and at the identifier the source file itself used, which is how
you get from a finding to the thing in your CAD tool. `datasheets` is empty here and carries the
page and table a datasheet-backed finding rests on, which rung 6 covers. `inconclusive` marks a
check that ran but could not decide, which matters enough to get its own rung later.

Two other formats are useful early. `--format markdown` organizes by severity, worst first, for
pasting into a review. `--format report` is the long form with each rule's full explanation.

## Narrowing

Run one rule while you work on it:

```
agni check designs/gateway/gateway.edn --rule i2c-pull-up
```

Or a whole category:

```
agni check designs/gateway/gateway.edn --tag category=power
```

## Failing a build

`--fail-on` makes `check` exit non-zero when anything at or above a severity is present, which is
all you need to put it in CI:

{{ agniRun "content/tutorials/runs/02-gate-fails.yaml" }}

The board has two `error` findings, so the command fails. With those gone it passes:

{{ agniRun "content/tutorials/runs/02-gate-passes.yaml" }}

Starting at `--fail-on error` is the practical choice. It gates on the things that will not work at
all, which almost nobody argues with, and it lets you tighten to `warning` later once the backlog is
clear.

## What the run says it looked at

Look at the last two lines of that first run again. Every run ends with them, whether or not it found
anything:

```
187 subject(s) considered by 27 rule(s), 7 not considered (--verdicts for the detail)
2 rule(s) reported violations without stating what they examined, so silence from those is not evidence of anything
```

This is the half a findings list cannot give you. A run that finds nothing and a run whose rules all
examined the wrong thing produce an identical list of findings, namely none, so the findings alone
can never tell you which one you are holding.

Read the three numbers separately. **187 considered** is how many subjects were actually judged.
**27 rules** is how many were willing to say what they looked at, which is not the same as how many
ran: most of the catalog has no subject in scope on any given board, and a rule with nothing to say is
not a gap. **7 not considered** is the one worth reading closely, and it gets its own look below.

The second line is the honest edge of the claim. Those 2 rules found something and never said what
they examined, so silence from them means nothing at all, and the coverage number above does not
cover them.

That number falls as the catalog converts. It was 3 while the design-intent rules still reported
violations only, which is worth noticing: the rules you write for your own board were the last ones
whose silence meant nothing, and they are the ones you most want a considered set from.

That line is the claim. `--verdicts` is the evidence, one row per subject with passes included:

{{ agniRun "content/tutorials/runs/02-verdicts.yaml" }}

Now the pass is checkable. It names C1 and C3, so you can open the schematic and confirm that those
capacitors really are on those rails. Delete C1 next revision and this output changes, where the
findings-only view would print the same nothing before and after.

Across the whole catalog that is a much larger table than the findings list, which is why the summary
is the default and the rows are a flag:

{{ agniRun "content/tutorials/runs/02-verdicts-summary.yaml" }}

`not-considered` is the third outcome and the one with no counterpart in a findings list: the rule
was willing to judge that subject and something stopped it, so it says what stopped it rather than
passing on incomplete evidence.

On this board only one of them wants a datasheet value of the kind you seed in
[rung 6](../06-part-limits/). The rest are the more interesting sort. Four are `floating-input`
declining a net that carries a passive part, because a resistor on a net might be the pull that fixes
it, might be a series element with the driver on the far side, or might be a footprint nobody stuffed,
and a netlist cannot tell those apart. Two are `esd-clamp-not-tvs` handing a bare net to
`esd-protection`, which is the rule that reports it. Neither is a gap you fill by seeding anything.
They are the check telling you where its reach ends.

That distinction between "checked and fine", "never checked" and "could not tell" runs through the
whole tool, and [rung 9](../09-read-the-verdicts/) is entirely about reading it.

## Next

[See it](../03-see-it/), because a list of net names is not how anyone thinks about a board.

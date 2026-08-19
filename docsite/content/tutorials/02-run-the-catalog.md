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
certainly not work: an I2C bus with no pull-up cannot signal at all, because the parts on it can
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

## What a clean run looks like

A run that finds nothing prints how many rules it ran:

```
no findings (29 rule(s) run)
```

The count is the important half. It tells you the check actually exercised 29 rules rather than
staying quiet because it had nothing to work with. If you load only a schematic and no board file,
the copper rules do not appear in that count at all, because there is no copper for them to look at.
That distinction between "checked and fine" and "never checked" runs through the whole tool, and
rung 9 is entirely about it.

## Next

[See it](../03-see-it/), because a list of net names is not how anyone thinks about a board.

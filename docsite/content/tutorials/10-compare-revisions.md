---
title: "10. Compare revisions"
description: "What actually changed between rev A and rev B, structurally rather than textually."
---

After the first review, the question is rarely "is this board good". It is "what changed since the
one we already reviewed, and do I need to look at all of it again".

A text diff of two schematic files cannot answer that. Export the same unmodified design twice and
the files usually differ, because coordinates shift, identifiers get reassigned, and ordering is not
stable. Everything is a change, so nothing is.

`agni diff` compares the designs rather than the files.

## Two revisions

The tutorial project bundles a second revision. Someone read the first review and fixed two of the
findings.

```
make diff
```

```
Components: +2  -0  ~0
Nets:       new 0  deleted 0  renamed 2  hard 3  soft 0

Components added (2):
  R4
  R5

Nets changed (5):
  [hard]    I2C_SCL: +[R4.2] -[]
  [hard]    I2C_SDA: +[R5.2] -[]
  [hard]    PMIC_CORE_3V3: +[R4.1 R5.1] -[]
  [renamed] XTAL_IN -> CLK_IN
  [renamed] XTAL_OUT -> CLK_OUT
```

Two resistors added, three nets gained connections, two nets renamed. That is the whole change, and
it is five lines rather than a few hundred.

## Renamed is not deleted plus added

`XTAL_IN -> CLK_IN` is the interesting classification. Textually that is one net vanishing and
another appearing. Structurally it is the same net with a new name, and the tool says so because the
set of pins on it did not change.

That distinction is most of the value. A rename touching forty nets is a naming sweep and needs
skimming. Forty nets genuinely deleted and forty added is a redesign and needs reviewing. They look
identical in a text diff.

## Hard and soft

A **hard** change alters what is connected: a pin joined or removed. A **soft** change alters
something recorded about the net without changing connectivity.

Hard changes are where electrical risk lives. When a diff is large, read the hard changes first and
treat the soft ones as background.

## The review delta

The structural diff says what moved. Running the checklist on both says whether it helped.

Rev A:

```
**3 pass, 8 fail, 1 n/a, 2 not-automated, 1 provisional (of 15)**
```

Rev B:

```
**5 pass, 6 fail, 1 n/a, 2 not-automated, 1 provisional (of 15)**
```

Two failures became passes, and the items say which:

```
| I4 | every I2C bus has pull-ups | pass |  |
| H1 | net names follow house convention | pass |  |
```

The two added resistors are the I2C pull-ups. The two renames bring the clock nets onto house
convention. Every structural change in the diff is accounted for by an item that improved, and
nothing else moved.

That last clause is the one to check on a real revision. A change that fixes what it intended and
also flips something unrelated is the normal way a fix causes a regression, and comparing the two
summaries is how you notice.

## Comparing across formats

The diff runs over the internal representation rather than the file, so the two sides do not have to
be the same format. A design exported as EDIF last quarter and as KiCad this quarter compares
cleanly, and a diff that comes back empty is a real statement that the two describe the same
netlist. That is also how you verify a CAD migration did not quietly change the design.

## Next

[Archive and gate](../11-archive-and-gate/), the last rung, which is about keeping a result and
making it block a merge.

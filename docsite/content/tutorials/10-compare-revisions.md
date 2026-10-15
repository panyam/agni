---
title: "10. Compare revisions"
description: "What actually changed between rev A and rev B, structurally rather than textually."
---

After the first review, the question becomes "what changed since the
one we already reviewed, and do I need to look at all of it again".

A text diff of two schematic files cannot answer that. Export the same unmodified design twice and
the files usually differ, because coordinates shift, identifiers get reassigned, and ordering is not
stable. Everything is a change, so nothing is.

`agni diff` compares the designs rather than the files.

## Two revisions

The Jetson board upstream is a single snapshot, so the project makes its second revision itself, by a
documented edit. `tools/make_rev_b.py` copies rev A and changes two things a reviewer would plausibly
ask for after the first review: it fits R357 and R358, two I2C pull-ups the schematic marked
do-not-populate, and renames the two interrupt nets spelled with a trailing `n` to KiCad's overbar.
Rev B is a design folder of its own, `designs/jetson-rev-b`, with a descriptor matching rev A's, so the
two read the same way and differ only by the edit.

```
make rev-b
```

Then compare them:

{{ agniRun "content/tutorials/runs/10-diff.yaml" }}

Two parts changed and two nets were renamed, which accounts for the whole edit. A text diff of these
two folders would be short too, because a script made the change. A revision KiCad saved after a day
of real work moves coordinates, reassigns identifiers and reorders the file, and this diff reads the
same either way.

## Renamed is not deleted plus added

`PDC_I2C1_IRQn -> ~{PDC_I2C1_IRQ_R}` is the interesting classification. Textually that is one net
vanishing and another appearing. Structurally it is the same net with a new name, and the tool says so
because the set of pins on it did not change.

{{ includeFile "figures/net-rename.svg" }}

That distinction is most of the value. A rename touching forty nets is a naming sweep and needs
skimming. Forty nets genuinely deleted and forty added is a redesign and needs reviewing. They look
identical in a text diff.

The new name carries a lesson of its own. The obvious spelling, `~{PDC_I2C1_IRQ}`, already names the
net on the far side of the series resistor R110, so a label spelled that way would have JOINED the two
nets, and the diff would have shown a hard change where a rename was meant. The script refuses a name
that is already in use, and the board's own convention for the near side of a series resistor is the
`_R` suffix, as in `~{USBC2_INT_R}`.

## Hard and soft

A **hard** change alters what is connected, by joining or removing a pin. A **soft** change alters
something recorded about the net without changing connectivity.

Hard changes are where electrical risk lives. When a diff is large, read the hard changes first and
treat the soft ones as background. This edit has none. Fitting a part changes what is BUILT rather
than what is drawn, so it shows as a component change, `dnp: "yes" -> "no"`.

## The review delta

The structural diff says what moved. Running the checklist on both says whether it helped:

{{ agniRun "content/tutorials/runs/10-review-delta.yaml" }}

One failure became a pass. H1 passes because both trailing-`n` names are gone. I4, every I2C bus has
pull-ups, still fails, because the board has other undriven buses, but it fails on two nets fewer:

{{ agniRun "content/tutorials/runs/10-verdicts-rev-b.yaml" }}

Read the subjects. The two fitted resistors pull up `I2C0_SCL_1V8` and `I2C3_SDA_1V8`, the clock of one
bus and the data line of another, which their neighbouring reference designators would not suggest. The diff told you which parts changed. The verdicts told you
which buses that helped, and on a real revision that is the half to check, because a fix that lands on
a different net than the one intended is how a fix causes a regression.

## Comparing across formats

The diff runs over the internal representation rather than the file, so the two sides do not have to
be the same format. A design exported as EDIF last quarter and as KiCad this quarter compares
cleanly, and a diff that comes back empty is a real statement that the two describe the same
netlist. That is also how you verify a CAD migration did not quietly change the design.

## Next

[Archive and gate](../11-archive-and-gate/), which is about keeping a result and making it
block a merge.

---
title: "Beside an older comparison tool"
description: "Why a semantic diff and a netlist-export spreadsheet can disagree on the same two revisions, and which one is reading the design."
---

A team moving to `agni diff` and its audit tables usually has an older comparison beside it: a netlist
export run through a spreadsheet, or a script that diffs two exports line by line. The two will
disagree on some rows of the same pair of revisions, and most of those rows come from the export, not
the design. This page goes through the kinds we have seen, each on a small synthetic fixture, so you
can tell a real change from an artifact of how the old tool read the file.

agni does not reproduce an export's mistakes to make the numbers match. A row it reports is a fact
about the design, which is what makes the comparison worth having.

## A pin named by a letter

Diodes, transistors and connectors often name pins by letter: `K` and `A` on a diode, `G`, `D` and `S`
on a FET. An export that splits a pin reference with a pattern expecting a digit drops those pins, and
the net they sat on then reads as changed between two revisions that are identical. agni keeps the
pin under its own name, so the same pair diffs clean:

{{ agniRun "content/guide/runs/legacy-letter-pins.yaml" }}

If the older table shows a net as new in one revision and deleted in the other, and every pin on it is
named by a letter, this is the likely cause.

## A net with one pin

A net that reaches a single pin is legitimate. It is a strap that leaves one side of a resistor open,
or a test pad named for what it carries, and some exports leave it out to match another tool's count.
agni lists it, so its net count can be higher than the spreadsheet's by the number of one-pin nets:

{{ agniRun "content/guide/runs/legacy-one-pin-net.yaml" }}

`single-pin-net` reports such a net unless something marks it no-connect, so the extra row is a
question worth asking rather than noise.

## A part named by an EDIF identifier

An EDIF identifier cannot hold a `-`, so a netlist names a cell like `PART-07` by a stand-in such as
`PART_07` and carries the real name in `(rename PART_07 "PART-07")`. A tool keying its parts table on
the identifier lists `PART_07`, and one keying on the part number property lists `PART-07`, so the same
part can appear under two keys. agni reads the part number from the property's string and names the
part type by the rename, never by the identifier:

{{ agniRun "content/guide/runs/legacy-edif-rename.yaml" }}

## Two package lines sharing a footprint

A Telesis netlist lists part numbers in `$PACKAGES`, one line per part number, and two lines can name
the same footprint. agni gives each part the part number of its own line (agni issue 835). An export
that drops a block's last line loses that line's parts, which a parts table then shows as removed:

{{ agniRun "content/guide/runs/legacy-telesis-footprint.yaml" }}

## Reading a disagreement

When the two tables disagree on a row, check the row against the design itself with one query, as each
section above does, rather than against the other table. If agni's answer matches the file, the row is
an artifact of the export, and recording which of these kinds it is saves the next reviewer the same
work. If it does not, that is a defect in agni's reading of the file, and worth an issue with the
smallest file that shows it.

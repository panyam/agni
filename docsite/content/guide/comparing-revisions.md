---
title: "Comparing revisions"
description: "Diff two versions of a design and read what changed, computed on the connectivity."
---

`agni diff` compares two versions of a design and reports what changed. Think of it as a
redline between rev A and rev B, but computed on the connectivity rather than eyeballed off
two prints.

## Run a diff

Give it two files, old first:

```
agni diff rev-a.edn rev-b.edn
```

```
diff rev-a.edn -> rev-b.edn

Components: +1  -0  ~0
Nets:       new 1  deleted 1  renamed 1  hard 1  soft 0

Components added (1):
  R4

Nets changed (4):
  [deleted] OLD
  [hard]    CLK: +[U1.6] -[]
  [new]     NEW
  [renamed] SIG -> DATA
```

## Read the change taxonomy

The summary counts, then lists, each kind of change:

- **Components**: added (`+`), removed (`-`), or modified (`~`, e.g. a value change). Here
  `R4` was added.
- **Nets** fall into five kinds:
  - **new / deleted**: a net that appears only in the new or only in the old revision
    (`NEW`, `OLD`).
  - **renamed**: a net that kept the *same connections* under a new name (`SIG -> DATA`).
    The tool detects this by connectivity, so a pure rename does not read as a delete plus
    an unrelated add.
  - **hard**: the connections themselves changed. `CLK: +[U1.6] -[]` means `U1` pin 6
    joined net `CLK`.
  - **soft**: a change the tool judges cosmetic (it did not alter connectivity).

The `+[...] -[...]` notation on a changed net lists the pin connections gained and lost.

## Why rename detection matters

Renaming a net is one of the most common revision edits, and a naive diff reports it as the
worst possible change (a whole net deleted, a whole net appeared). By matching on
connectivity, `agni diff` calls it what it is, a rename, so your review focuses on the edits
that actually moved a wire.

For rename detection to fire, the net has to keep identical connections under the new name.
A net that was both renamed *and* rewired shows up as a hard change, not a rename.

## In the viewer

`agni serve` renders the same diff visually: open two revisions and changed entities are
tinted by kind. On faithful-geometry formats (KiCad) the two revisions can be overlaid,
because the author coordinates are preserved between them. Netlist-only formats render via
auto-layout, where node positions shift when the node set changes, so those revisions are
compared side by side rather than overlaid. The web tour in the developer docs walks the
visual diff in detail.

## Where to go next

- [Checks and reports](../checks-and-reports/): run the rule catalog on either revision.
- [CLI reference](../cli-reference/): `diff` and the other commands.

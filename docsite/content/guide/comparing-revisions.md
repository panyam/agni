---
title: "Comparing revisions"
description: "Diff two versions of a design and read what changed, computed on the connectivity."
---

`agni diff` compares two versions of a design and reports what changed, like a redline between
rev A and rev B computed on the connectivity rather than eyeballed off two prints.

## Run a diff

Give it two files, old first:

{{ agniRun "content/guide/runs/diff-revisions.yaml" }}

## Read the change taxonomy

The summary counts, then lists, each kind of change:

- **Components** can be added (`+`), removed (`-`), or modified (`~`, e.g. a value change). Here
  `R4` was added.
- **Nets** fall into five kinds:
  - A **new** or **deleted** net appears only in the new or only in the old revision
    (`NEW`, `OLD`).
  - A **renamed** net kept the *same connections* under a new name (`SIG -> DATA`).
  - A **hard** change alters the connections themselves. `CLK: +[U1.6] -[]` means `U1` pin 6
    joined net `CLK`.
  - A **soft** change is one the tool judges cosmetic (it did not alter connectivity).

The `+[...] -[...]` notation on a changed net lists the pin connections gained and lost.

## Why rename detection matters

Renaming a net is one of the most common revision edits, and a naive diff reports it as the
worst possible change (a whole net deleted, a whole net appeared). By matching on
connectivity, `agni diff` reports it as a rename, so your review focuses on the edits
that actually moved a wire.

{{ includeFile "figures/net-rename.svg" }}

For rename detection to fire, the net has to keep identical connections under the new name.
A net that was both renamed *and* rewired shows up as a new net plus a deleted one, unless you
pass `--rename-approx`. The tutorial board's revision C renames the regulator enable net and adds a
pull-up to it:

{{ agniRun "content/guide/runs/diff-rename-approx.yaml" }}

The pairing is marked `renamed?` because it is the best match among candidates rather than a fact the
connectivity proves, which is why the pass is off unless you ask for it. The API takes the same
option as `rename_approx` on `DiffDesignsRequest`, and the Python client as
`diff_designs(..., rename_approx=True)` over either transport.

## In the viewer

`agni serve` renders the same diff visually, so when you open two revisions the changed
entities are tinted by kind. On faithful-geometry formats (KiCad) the two revisions can be overlaid,
because the author coordinates are preserved between them. {{ explainableCap "netlist" }}-only
formats render via auto-layout, where node positions shift when the node set changes, so
those revisions are compared side by side rather than overlaid.
[The web app](../../architecture/web-app/) page covers the visual diff in detail.

## Where to go next

- [Checks and reports](../checks-and-reports/) runs the rule catalog on either revision.
- [CLI reference](../cli-reference/) documents `diff` and the other commands.

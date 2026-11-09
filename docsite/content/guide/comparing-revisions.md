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
  - A **soft** change did not alter connectivity, and the tool judges it cosmetic.

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

The pairing is marked `renamed?`. It is the best match among candidates, and the connectivity does
not prove it, so the pass is off unless you ask for it. The API takes the same option as
`near_renames` on `DiffDesignsRequest`. An empty message runs the calibrated thresholds. Setting a
field, such as `min_new_coverage_significant`, overrides that one threshold. The
pass matches by connectivity and never by name, so the thresholds are fractions of shared endpoints.
The Python client sends it as `diff_designs(..., near_renames={})`; overriding a threshold needs a
server, since the CLI runs the defaults only.

## Listing what did not change

A diff reports changes, so a net that is the same in both revisions does not appear. Some reports
list every net under its outcome, a revision workbook for instance. Such a report asks for the
unchanged nets too with `--include-equal`. They come back as kind `equal` in `--format json`, as
`net-equal` rows in `--format csv`, and as a count in the text summary:

{{ agniRun "content/guide/runs/diff-include-equal.yaml" }}

Every net of both revisions is then accounted for once: an unchanged net as `equal`, a changed one
under its kind, and a rename as one entry covering its old and new names. The API takes it as
`include_equal` on `DiffDesignsRequest`.

## Declared revisions

A later revision usually sits in the same folder as the first, with a board of its own. Declare it
in the design's `design.yaml`, under `revisions`, with its own entry and companions:

```yaml
name: gateway
extends: ../../project.yaml
entry: gateway.edn
companions: [gateway.kicad_sch, gateway.kicad_pcb]
revisions:
  - entry: gateway-rev-b.edn
    companions: [gateway-rev-b.kicad_pcb]
```

When you name a declared revision's entry or one of its companions, agni reads that revision with
its own views, so rev B's board-tier rules run against rev B's copper. A file belongs to one
revision at most, so a board is never borrowed by a netlist it was not drawn for.

A revision you have not declared reads exactly as named, with no board. Its connectivity diffs as
well as a declared one's, but a check of it runs no board-tier rule, and `check` says on stderr which
ones did not run. Without that note a revision with fewer findings could be read as a cleaner board
when its copper was never read. The tutorial project keeps `gateway-rev-c.edn` undeclared to show
the difference:

```
agni check examples/tutorial-project/designs/gateway/gateway-rev-b.edn
agni check examples/tutorial-project/designs/gateway/gateway-rev-c.edn
```

[Projects and designs](../projects/) covers the rest of `design.yaml`, and
[the architecture page](../../architecture/projects-and-designs/) says why companions and revisions
are declared file by file rather than inferred.

## In the viewer

`agni serve` renders the same diff visually, so when you open two revisions the changed
entities are tinted by kind. On faithful-geometry formats (KiCad) the two revisions can be overlaid,
because the author coordinates are preserved between them. {{ explainableCap "netlist" }}-only
formats render via auto-layout, where node positions shift when the node set changes, so
those revisions are compared side by side rather than overlaid. The comparison draws in the
renderer the canvas draws in, SVG or WebGL, so a large board compares at the speed it pans.
[The web app](../../architecture/web-app/) page covers the visual diff in detail.

While a comparison is open, the **Tables** panel offers it as a table, a row per change in the
taxonomy above. `agni diff --format csv` writes the same rows. The engine lays them out (the
`Tabulate` rpc) and the panel only shows them. Tick **near-renames** for `--rename-approx` and
**unchanged nets** for `--include-equal`. Clicking a column heading sorts the table. The panel asks
the engine for that order, so a ref-des column puts R2 before R10. A net or a part in the table is a
link that finds it on the drawing, and **Save CSV** downloads the table as the CLI would write it.
The same panel also runs the checks over either revision and shows each run's findings and verdicts.

A comparison has an address too. `agni diff <old> <new> --server <url>` prints a `look at it:` link to
stderr, and following it opens the old revision with Compare already started against the new one,
as if you had picked it in the Compare picker. The link is the old revision's view, with
`?against=<mount>/<path>` naming the new one. It carries the question rather than an answer, so it
compares the two files as they are when you open it. It is printed only when both revisions can be
linked on that server. [Checks and reports](../checks-and-reports/) gives the rules for any link,
and `--server self` serves both from the same process. An MCP `diff` answer carries the same
link (agni issue 1126). A diff that paired near-renames (`--rename-approx`) adds `rename_approx=1`, and
the comparison's tables open with near-renames ticked, so they show what the command showed.

The browser-only demo opens a comparison too, from its Compare picker (agni issue 978) or from an
`?against=` link, with the engine in the page holding both revisions' files.

## Where to go next

- [Checks and reports](../checks-and-reports/) runs the rule catalog on either revision.
- [CLI reference](../cli-reference/) documents `diff` and the other commands.
- [Beside an older comparison tool](../older-comparison-tools/) explains the rows where this diff and a
  netlist-export spreadsheet disagree.

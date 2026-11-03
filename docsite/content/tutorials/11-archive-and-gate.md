---
title: "11. Archive and gate"
description: "Keep the result, re-read it a year later, and make it block a merge."
---

A review that exists only as terminal output is gone the moment the window closes. To be evidence it
has to outlive the run, and to change behavior it has to be able to stop a merge. This rung does both.

## Write the run down

`--results-out` writes the run to a file as it happens:

```
agni review designs/jetson --results-out review.results.json
```

The file is a self-contained check-result document. It carries what produced it and at what build,
which design and a content hash of that design, which tiers were attached, which rules actually ran,
and every finding and outcome.

Two results documents are only comparable once you know which
build made each. The content hash is the revision identity, so a stale document cannot be silently
read against a design that has since changed. And the list of rules that ran separates a clean
design from a run that checked nothing. Rung 9 was about the same distinction.

## Read it back without the design

{{ agniRun "content/tutorials/runs/11-results-reread.yaml" }}

This works with the design gone. Copy the JSON to a machine that has
never seen the board, has no parameter corpus, and no profiles, and it renders the same report.

So the document is archival. A year from now, when the design file has moved and the tool has
moved on several versions, the document still says what was checked and what was found.

## A report a person can read

The results document is for machines and for your future self. For a reviewer who wants to read the
run today, render it as HTML:

{{ agniRun "content/tutorials/runs/11-report-html.yaml" }}

That writes a self-contained page: failures first, then each rule with what it examined, why the
rule exists, and what to do about a failure. On this board it is about 8 MB, because it carries every
net name and reference designator, so treat it like the design itself. Keep it out of any
repository where the design could not go. It needs no server and no assets, so it attaches to a
review ticket or a release folder as one file.

Give it `--server self` and each subject becomes a link into a viewer, which opens the board with
that verdict's proof drawn:

```
agni check --format html --server self designs/jetson > review.html
```

`--server self` makes this one command rather than two, for the same reason the CLI can withhold a
link. A link promises the reader something they can follow, so the CLI emits one only when it knows
the server serves the design the CLI just read. `self` starts a viewer on a free port over
THIS run's own mount table and blocks until Ctrl-C, so the two cannot disagree by construction.

Point it at a server someone else is running and the promise has to be checked instead:

```
agni check --format html --server http://localhost:8080 \
  --mount work=. designs/jetson > review.html
```

The report needs that `--mount` to carry links. The CLI asks that server whether it serves the
same mounts from the same roots. If you did not NAME a mount, the CLI minted it for this run alone,
and it means nothing on anybody's server. The report then renders plain text subjects instead of
links, and says why, rather than emitting a URL that resolves on nobody's machine.

For one design and a quick look, [`agni open`](../03-see-it/) serves it and prints a ready-made check
command with the mount already filled in. That is the same trade in the other order. Use `open`
when you want the viewer open anyway, and `--server self` when you want the report.

The page is not the archival artifact, because the results document replays without the
design and the page does not. And a rule that reports violations without stating what it examined is
labelled "findings only" here, with its rows captioned so nobody reads silence from it as a clean
bill.

## Gate a merge

{{ agniRun "content/tutorials/runs/11-gate-merge.yaml" }}

Exit `2` is a tripped gate, so CI fails. Put that one line in your pipeline and a board with an
error-severity finding cannot merge. On this board that is the twenty-eight `i2c-pull-up` errors.

## The gate reads severity, not verdict

Rev B fitted two of the missing pull-ups and fixed the naming, and its review went from 7 failures to
6. Run the gate on it:

{{ agniRun "content/tutorials/runs/11-gate-rev-b.yaml" }}

Still failing, and correctly, because twenty-six I2C errors remain. A gate trips on a set of
findings, and a revision that fixes some of them does not open the gate. The review delta in rung 10
shows the progress that the exit code cannot show.

The subtler case is evidence. In rung 6 the checklist scored the first-pass draft of the NTS0102 as
`provisional`. Put that draft back, and gate on the rule it feeds:

{{ agniRun "content/tutorials/runs/11-gate-provisional.yaml" }}

`--fail-on` operates on **finding severity**. Severity is a statement about consequence.
`provisional` is a statement about evidence quality. They are different axes. A finding can be
severe and poorly evidenced at the same time, and this one is both.

So a provisional finding still gates. Whether it should is your call. If you leave it gating, the
block becomes pressure to go transcribe the real value. That is usually the right instinct, and here
it would have found the draft's mistake. You can also defend dropping the parameter tier out of the
gate until the corpus is trustworthy, as long as you accept that those checks are not gating yet.

Do not lower the severity of the rule to make the gate pass. That changes what the tool claims
about consequence to change an exit code, and every future reader of that
finding inherits the lie.

## Gate on the checklist too

The gate above cannot see one whole class of regression. Before you rely on it, run the review
with a floor under how many items it has to answer:

{{ agniRun "content/tutorials/runs/gate-answered-holds.yaml" }}

Thirteen of the fifteen items get answered, so the floor holds. Now drop the `params` line from
`project.yaml`, as a merge resolving a conflict in that file eventually will, and run exactly the same
two commands you have been gating with:

{{ agniRun "content/tutorials/runs/gate-corpus-moved-coverage.yaml" }}

**Covered did not move.** It is still 14 of 15. The item that used to check a part against its
datasheet now reads `not-applicable`, because its rule is still in the catalog and merely has nothing
to read, and `not-applicable` counts as covered. Nothing in the failure count says so either.

`--min-answered` is the number that moved, and it trips:

{{ agniRun "content/tutorials/runs/gate-corpus-moved-trips.yaml" }}

`review` therefore needs a gate of its own. `check --fail-on` asks how bad the answers
were; this asks whether the questions were answered. When a checklist quietly answers fewer of its
own items, every other number you have looks identical to a clean board.

Moving the `params/` folder itself fails differently, and louder. A descriptor names its corpus by
path, so a path that names nothing stops every command at load with the path in the message, and
only a line that is gone reads as a project that never had a corpus.

Put the line back before continuing:

```
mv project.yaml.orig project.yaml
```

A `provisional` does not trip `--fail-on-outcome fail`, because it is a failure based on placeholder
data. A pipeline that goes red on data quality is a pipeline somebody switches off. Ask for it by
name when you want it: `--fail-on-outcome fail,provisional`. And a tripped gate exits `2` where a
broken run exits `1`, so a script can tell a bad board from a bad tool.

## Where to start gating

`--fail-on error` first. Errors are things that will not work at all. Almost nobody argues with
those, and the initial list is usually short.

Tighten to `--fail-on warning` once that list is empty run after run on its own. Going
straight there on an existing board produces a wall of failures on day one, and somebody
reliably turns the gate off.

Run the full `review` in CI alongside the gate and publish the results document as a build artifact.
The gate answers whether this can merge. The document answers what was checked. In six months
somebody will ask whether a particular question was ever considered, and you will want the document
then.

## Next

[Reconcile with the tools you already run](../12-reconcile-existing-tools/), which brings your
existing DRC report into the same model.

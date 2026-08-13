---
title: "11. Archive and gate"
description: "Keep the result, re-read it a year later, and make it block a merge."
---

A review that exists only as terminal output is gone the moment the window closes. To be evidence it
has to outlive the run, and to change behavior it has to be able to stop a merge. This rung is both.

## Write the run down

```
make report
```

```
wrote reports/gateway/review.results.json
```

That is a self-contained check-result document. It carries what produced it and at what build, which
design and a content hash of that design, which tiers were attached, which rules actually ran, and
every finding and outcome.

Each of those exists for a reason. Two results documents are only comparable once you know which
build made each. The content hash is the revision identity, so a stale document cannot be silently
read against a design that has since changed. And the list of rules that ran is what separates a
clean design from a run that checked nothing, which is the same distinction rung 9 was about.

## Read it back without the design

```
agni results reports/gateway/review.results.json --format markdown
```

```
# Review: Gateway ECU design review

Design: `designs/gateway/gateway.edn`

**3 pass, 8 fail, 1 n/a, 2 not-automated, 1 provisional (of 15)**
```

The useful property is that this works with the design gone. Copy the JSON to a machine that has
never seen the board, has no parameter corpus, and no profiles, and it renders the same report.

That is what makes it archival. A year from now, when the design file has moved and the tool has
moved on several versions, the document still says what was checked and what was found.

## Gate a merge

```console verify
$ agni check designs/gateway/gateway.edn --conventions conventions.yaml --params params --fail-on error > /dev/null
$ echo $?
2
```

Exit `2` is a tripped gate, so CI fails. Put that one line in your pipeline and a board with an error-severity finding
cannot merge.

## The gate reads severity, not verdict

Here is the part that surprises people. Rev B fixed the I2C pull-ups and the naming, and its review
went from 8 failures to 6. Run the gate on it:

```console verify
$ agni check designs/gateway/gateway-rev-b.edn --conventions conventions.yaml --params params --fail-on error > /dev/null
$ echo $?
2
```

Still failing. The remaining error is the datasheet finding on U2, the one the review reported as
`provisional` because it rests on placeholder data.

`--fail-on` operates on **finding severity**, which is a statement about consequence. `provisional`
is a statement about evidence quality. They are different axes, and a finding can be severe and
poorly evidenced at the same time, which is exactly what this one is.

So a provisional finding still gates. Whether it should is your call, and there are two honest
answers. Leave it gating and treat the block as pressure to go transcribe the real datasheet value,
which is usually the right instinct. Or drop the parameter tier out of the gate command until the
corpus is trustworthy, and accept that those checks are not gating yet:

```console verify
$ agni check designs/gateway/gateway-rev-b.edn --conventions conventions.yaml --fail-on error > /dev/null
$ echo $?
2
```

Still `2`, and the reason is worth stopping on: dropping the parameter tier removed the *datasheet*
error, and a different one was underneath it — a CAN host declaring the interface without its `STB`
signal. Narrowing what a gate can see does not make a board pass, it only changes which failure you
are looking at. If you want to know what remains, run without `--fail-on` and read the list.

What you should not do is lower the severity of the rule to make the gate pass. That changes what
the tool claims about consequence in order to change an exit code, and every future reader of that
finding inherits the lie.

## Gate on the checklist too

The gate above cannot see one whole class of regression, and it is worth meeting before you rely on
it. Run the review with a floor under how many items it has to answer:

{{ agniRun "content/tutorials/runs/gate-answered-holds.yaml" }}

Thirteen of the fifteen items get answered, so the floor holds. Now move the parameter corpus out of
the way, as somebody reorganising a repository eventually will, and run exactly the same two commands
you have been gating with:

{{ agniRun "content/tutorials/runs/gate-corpus-moved-coverage.yaml" }}

**Covered did not move.** It is still 13 of 15. The item that used to check a part against its
datasheet now reads `not-applicable`, because its rule is still in the catalog and merely has nothing
to read, and `not-applicable` counts as covered. Nothing in the failure count says so either.

`--min-answered` is the number that moved, and it trips:

{{ agniRun "content/tutorials/runs/gate-corpus-moved-trips.yaml" }}

That is the whole reason `review` has a gate of its own. `check --fail-on` asks how bad the answers
were; this asks whether the questions were answered. A checklist quietly answering fewer of its own
items looks identical to a clean board on every other number you have.

Put `params` back before continuing:

```
mv params-old params
```

Two notes on using it. A `provisional` does not trip `--fail-on-outcome fail`, because it is a failure
resting on placeholder data and a pipeline that goes red on data quality is a pipeline somebody
switches off. Ask for it by name when you want it: `--fail-on-outcome fail,provisional`. And a tripped
gate exits `2` where a broken run exits `1`, so a script can tell a bad board from a bad tool.

## Where to start gating

`--fail-on error` first. Errors are things that will not work at all, which almost nobody argues
with, and the initial list is usually short.

Tighten to `--fail-on warning` once that list stays empty on its own. Going straight there on an
existing board produces a wall of failures on day one, and the reliable outcome is that somebody
turns the gate off.

Run the full `review` in CI alongside the gate and publish the results document as a build artifact.
The gate answers whether this can merge. The document answers what was checked, which is the thing
you will want in six months when somebody asks whether a particular question was ever considered.

## That is the ladder

You now have a board that is read correctly, checked against general rules and your team's own, with
a checklist whose every item reports honestly, comparable revision to revision, archived, and
gating.

The two things worth revisiting periodically are coverage and the parameter corpus. Coverage tells
you how much of your checklist is really being decided. The corpus is usually the cheapest way to
move it.

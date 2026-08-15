---
title: Evidence
---

# Evidence: measuring, testing, and trusting a result

Most of the expensive mistakes in this repo have not been wrong code. They have been a correct-looking
*result* that nobody could have falsified: a sweep that found nothing because the detector was broken,
a rate nobody checked for precision, a green test that could not have gone red. This page is the
accumulated set of those, and the habits that catch them.

Everything here is one question in different clothes: **what would this have looked like if it were
wrong?** When the answer is "the same", the measurement is not evidence yet.

## Measuring, and trusting a measurement

**A NEGATIVE RESULT NEEDS A POSITIVE CONTROL.** "Zero hits across 62 documents" is a claim about the
instrument until you show the instrument can find a known instance. Three separate absence claims in
this repo turned out to be artifacts of the detector rather than facts about the data: a table shape
the heuristic could not see, a regex that could not match `V CC` so an arity gate silently dropped the
one sentence being looked for, and a corpus sweep that ran a different code path from the one shipping.
Before reporting an absence, plant a known instance and confirm it is found. The fixtures usually
already contain one.

Two habits that make this cheap. **Instrument the gates**: count and sample what each filter REJECTED,
not only what it matched, which is the silence-never-reads-as-coverage discipline applied to your own
tooling. And **exercise the shipped configuration**: a sweep run with different flags from the ones
the feature ships with has validated a different program.

**A detector that FIRES is a claim about the instrument too.** The rule above is about absence; the
mirror case is a positive rate nobody checked for precision. A sweep for "does this document print a
document number" reported 86% coverage, and the regex behind it accepted `TPS22918` and `TCAN1145`,
which are PART numbers, while missing `SLVSAG5`, which is a real document number. The headline would
have justified building an extractor on a signal that was wrong in both directions. Before quoting a
rate, run the detector against a handful of known positives AND known negatives by hand; when it
cannot tell the two apart, that is the finding, and it is usually more useful than the rate.

**Reading the code is not running it, and the gap is where the expensive bugs live.** Three passes
through the same read path did not reveal that no producer fills `SourceDoc.title` as its contract
specifies; opening the app and looking at the field showed it immediately, and invalidated part of a
PR merged an hour earlier. Code review catches a layer that is wrong. It does not catch layers that
are each internally consistent and collectively wrong, because every file reads fine on its own. When
a change has a user-visible surface, drive it before designing on top of it. See `build/overlay.md`
and the web-app page for how to stand the app up.

**When a run contradicts your PREDICTION, the contradiction is the finding — do not adjust the test
to absorb it.** A test written to prove a malformed `project.yaml` fails the run came back saying the
run had SUCCEEDED, against a confident reading that a downstream error check would catch it. The
tempting move is to assume the fixture is wrong. Chasing the gap instead found that the descriptor
was never reachable at all, which turned out to be the actual content of the ticket (agni issue 312)
rather than the two call sites it named. A surprise here is cheap to investigate and has repeatedly
been worth more than the change that surfaced it.

**Anything matching SYMBOL TEXT out of a doc-IR must tolerate an injected space.** Producers flatten
subscripts, so `VCCA` arrives as `V CCA` (~850 such occurrences in one corpus). This has bitten three
times in unrelated places: a prose sweep, the derive pin path where it would have produced pin ids no
symbol library could match, and in-document search. Assume the space is there.

**A feature premised on a house CONVENTION needs its base rate measured on real designs first, and
the fixtures cannot tell you.** Every fixture in this repo names things the way the built-in
vocabulary expects, because that is what made them work, so a convention-shaped feature always looks
well-founded against them. Measured on two real boards, the endpoint-encoding net-name convention an
issue was written around covered 1.1% and 0.06% of nets, while the shipped tutorial project does not
use it at all. The same measurement then found a live silent bug in the opposite direction. Count the
shape on a real design before designing to it; it is one query and it has twice changed what was
worth building.

**A feature no fixture EXERCISES cannot fail a test, and that reads exactly like working.** The
datasheet role tier shipped against a corpus where not one seeded spec declared pins, so the pass had
no evidence to read: every test passed, the real boards were unchanged, and nothing could have gone
red if it were wrong. The fix was to give the shipped fixture the data the feature consumes, which is
what turned it from unfalsifiable into demonstrable (0 rails to 3 on the tutorial netlist). Before
believing a green run, check that some committed fixture actually carries the input.

**`prototext` output varies its whitespace ON PURPOSE, so never grep it for a count.** Go's prototext
marshaller inserts an unstable extra space to discourage byte-comparison, so `function:  X` and
`function: X` are the same run on different days. A before/after table built with a one-space pattern
read ZERO for every "before" and was nearly shipped; the tell was an internal contradiction (a file
showing 0 typed pins and 2 supply pins at once), not the number itself. Match with `: +` or parse the
proto, and distrust any count whose parts do not add up.

**A long-lived ticket's PREMISE erodes silently, so re-verify it against the code before planning.**
Three substantial issues this month had aged out before anyone picked them up: one was mostly shipped
already, one rested on a convention the only real boards contradicted, and one had landed in pieces
under other work. Nothing was wrong with any of them when filed; adjacent work moved underneath and
the ticket text kept asserting the old world. Read the comment thread, not just the body, and check
the claims against the tree. It costs minutes and has now saved three wasted PRs.
## Trusting a test

A new test is a measurement too, and the red-check is its positive control: neutralise the behaviour
the test is meant to catch and confirm the test fails. Stashing ONE tracked file
(`git stash push -- path/to/file.go`, run the test, pop) is the cheap way to do it. Two outcomes are
worth knowing before you see them.

**That move fails when the stashed file also carries something the test needs to compile** — a new
field, type, or exported helper the test references. The stash removes both the behaviour and the
declaration, so the run comes back `[build failed]`, which proves nothing and looks like it did. Both
times this bit, the fix was to revert the BEHAVIOUR in place (flip the branch to `if false`, drop the
one assignment) and leave the declarations, then restore. Read the red output before believing it: a
compile error is not a failing assertion.

**The other outcome is that it stays GREEN, which means the test asserts nothing.** A render test
written around a gap heuristic ("the two notes must be at least N apart") passed with the fix
removed, because another mechanism shrank the fixture enough that the gap held either way. The
assertion could not fail, and only the red-check revealed it. When a test survives its own fix being
neutralized, replace the heuristic with the PROPERTY: that one added a marker at the same anchor and
asserted the two render at the same y, which then failed with the actual defect named. Run the
red-check on every new test, not just ones you doubt.

**A test that calls the PRODUCTION predicate to decide what counts as a failure cannot fail when
that predicate is what broke.** Two assertions written as `if skipRefDes(x) { t.Error(...) }` read
as real checks and went green under a deliberately broken `skipRefDes`, while their siblings written
against literals went red. It is the same defect as the heuristic above wearing better clothes: the
test and the code agree by construction. Assert against literals, or against a set the production
path produced, and let only the red-check tell you which kind you wrote.
---
title: "9. Read the verdicts"
description: "Why a question nobody answered must not score as a pass, and how to audit one that did."
---

Every checking tool most people have used reports two things: a violation, or silence. That works
only if silence reliably means "checked, and fine". It does not. Silence is also what you get from a
rule that had nothing to work with, an interface that is not on the board, a datasheet nobody seeded,
and a design whose intent was never declared.

Collapsing all of those into "pass" is how a review comes back green on a board nobody checked. The
outcome vocabulary exists to keep them apart.

## The vocabulary

| Outcome | What it means |
|---|---|
| `pass` | the check ran and found nothing wrong |
| `fail` | the check ran and found something wrong |
| `provisional` | it found something, resting on data below the trust floor |
| `not-applicable` | the question does not apply to what was loaded |
| `needs-data` | it would apply, but a datasheet parameter is missing |
| `needs-design-intent` | it would apply, but nothing declared the intent |
| `computed-n/a` | the design itself rules the question out |
| `inconclusive` | the check ran and could not decide |
| `not-automated` | nothing automated is bound to this item |

Only the first two are verdicts about your board. Everything else is a verdict about the *check*,
and the distinction is the point. `pass` is a claim, and a claim needs someone to have looked.

## Coverage before results

{{ agniRun "content/tutorials/runs/coverage-rollup.yaml" }}

Read the two leading numbers before reading anything else. A board with zero failures and a coverage
of 4 of 15 has not been reviewed, and the failure count alone would have told you it was perfect.

**Covered** is how many items a mechanism exists for. **Answered** is how many the run actually
decided, and it is the stricter of the two. They are equal here, and the case where they separate is
the one worth knowing about: a rule can be in the catalog, selected, and still unable to run because
the facts it reads are absent. That item reads `not-applicable` and still counts as covered. Rung 11
gates a pipeline on the answered number for exactly that reason.

Coverage is also the number that tells you where to invest. Interfaces sits at 3 of 4 because of an
absent bus, which is fine and will never improve. House style sits at 2 of 3 because of a genuinely
manual item, which is also fine. Neither is a gap worth working on. A column filling up with
`needs-data` is, because seeding a few parts converts it directly into coverage.

## Eight failures is a good sign

The tutorial board reports 8 fail out of 15. That looks alarming and it is the healthiest state on
offer, because every one of them is a real defect with a named subject and a reason.

Compare against the run before the tiers were added, which reported far fewer failures. The board
was not better then. Fewer questions were being asked.

This is the counterintuitive part of adopting the tool. Failures going **up** as you add tiers is
the system working. Failures going up while coverage stays flat is the number to worry about.

## Auditing a pass

A pass is the outcome that deserves suspicion, because it is the one that ends the conversation. Two
specific ways a pass can be hollow.

**An inverted query.** An item bound to a query that matches the healthy case rather than the
violation passes exactly when it should fail. Read the `match:` of every query item and confirm it
describes something being wrong.

**A rule that had no members.** An item bound to a rule that quantifies over a device class passes
when no component is in that class, which can mean the class was never resolved rather than that
nothing was in it. Rung 7's false module-absence findings are this shape seen from the other side.

The general form: for every pass, you should be able to name the thing it looked at. If you cannot,
the pass is a guess.

`agni check --verdicts` is how you make a rule answer that, one row per subject with the evidence
the pass rests on:

{{ agniRun "content/tutorials/runs/09-audit-a-pass.yaml" }}

Read the middle column as the claim and the right column as the reason to believe it. A pass whose
reason you cannot check against the design is the one to go and look at.

Two limits worth knowing before you lean on it. A rule only appears here if it STATES what it
considered, so a rule missing from the output is declining to answer rather than reporting that it
considered nothing, and the HTML report labels those "findings only" for exactly that reason. And a
pass can only cite a value the rule actually measured, so a few rules still pass with the shape of
their test rather than a number.

## When a check cannot decide

`inconclusive` is separate from every other non-pass outcome, and it is the subtlest. The check ran,
had its inputs, and still could not reach a verdict, usually because the design is ambiguous in a
way the rule cannot resolve. It is a real answer of "I looked and I cannot tell" rather than a gap
in your data or your checklist, sometimes the honest answer, and it belongs in front of a human
rather than being rounded to a pass.

## What is not on this list

There is no `waived`. A finding you have decided to accept is still a finding, and the decision to
accept it lives in your process rather than in the tool's verdict. A tool that lets a report be
edited into agreement stops being evidence.

## Next

[Compare revisions](../10-compare-revisions/), because the question is usually not "is this board
good" but "what changed since the last one".

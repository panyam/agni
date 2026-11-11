---
title: "9. Read the verdicts"
description: "Why a question nobody answered must not score as a pass, and how to audit one that did."
---

Every checking tool most people have used reports two things: a violation, or silence. That works
only if silence reliably means "checked, and fine". It does not. You also get silence from a rule
that had nothing to work with, an interface that is not on the board, a datasheet nobody seeded, and
a design whose intent was never declared.

Collapse all of those into "pass", and a review comes back green on a board nobody checked. The
outcome vocabulary exists to keep them apart.

## The vocabulary

| Outcome | What it means |
|---|---|
| `pass` | the check ran and found nothing wrong |
| `fail` | the check ran and found something wrong |
| `provisional` | it found something, but the data behind it sits below the trust floor |
| `not-applicable` | the question does not apply to what was loaded |
| `needs-data` | it would apply, but a datasheet parameter is missing |
| `needs-design-intent` | it would apply, but nothing declared the intent |
| `computed-n/a` | the design itself rules the question out |
| `inconclusive` | the check ran and could not decide |
| `not-automated` | nothing automated is bound to this item |
| `covered-externally` | another tool's report shows no violation ([rung 12](../12-reconcile-existing-tools/)) |

Only the first two are verdicts about your board. Everything else is a verdict about the *check*.
`pass` is a claim, and a claim needs someone to have looked.

## Coverage before results

{{ agniRun "content/tutorials/runs/coverage-rollup.yaml" }}

Read the two leading numbers before reading anything else. A board with zero failures and a coverage
of 4 of 15 has not been reviewed, and the failure count alone would have told you it was perfect.

**Covered** is how many items a mechanism exists for. **Answered** is how many the run actually
decided, and it is the stricter of the two. They differ here by one item, I2, the CAN interface. A
profile is bound to it, so it is covered, and the board carries no CAN, so the run had nothing to
decide and the item reads `not-applicable`. A catalog rule ends up in the same place when the facts
it reads are absent and it cannot run. Rung 11 gates a pipeline on the answered number for exactly
that reason.

{{ includeFile "figures/covered-vs-answered.svg" }}

Coverage also tells you where to invest. Interfaces carries one `n/a` for an
absent bus, which is fine and will never improve. House style sits at 2 of 3 because of a genuinely
manual item, which is also fine. Neither is a gap worth working on. A column filling up with
`needs-data` is a gap worth working on, because seeding a few parts converts it directly into
coverage.

## Seven failures is a good sign

This board reports 7 fail out of 15. That looks alarming, but it is the healthiest state on offer.
Every one of them names its subjects and its reason, so a reviewer can now finish the conversation
about each one. Rung 2's I2C errors are among them, and so are the test points on nets nobody
named, which no shipped rule would ever have raised.

Compare against the board before the tiers were added. Fewer questions were being asked, so some
failures could not appear, and others, like the ESD findings on internal connectors, were noise
burying the ones that mattered.

If failures go **up** as you add tiers, the system is working. If failures go up while coverage
does not move, worry about that number.

## Provisional

Rung 6 left one state to show. Put its first-pass draft of the NTS0102 back over the transcribed
file and score the checklist:

{{ agniRun "content/tutorials/runs/09-review-provisional.yaml" }}

P4 reads **`provisional`**, not `fail`. The finding is shown in full, and the engine declines to call
it a defect, because every limit behind it carries a confidence below the trust floor.
`--ratified-floor` sets that floor, 0.9 by default.

## Auditing a pass

A pass deserves suspicion, because a pass ends the conversation.
There are two specific ways a pass can be hollow.

Some items are bound to an **inverted query**, one that matches the healthy case rather than the
violation. Such an item passes exactly when it should fail. Read the `match:` of every query item
and confirm it describes something being wrong.

A rule can also have **no members**. Take an item bound to a rule that quantifies over a device
class. It passes when no component is in that class. That can mean the class was never resolved,
rather than that nothing was in it. Rung 7's note on declaring modules by class is this shape seen
from the other side.

In general, you should be able to name the thing every pass looked at. If you cannot,
the pass is a guess.

`agni check --verdicts` makes a rule answer that, with one row per subject and the evidence behind
each pass:

{{ agniRun "content/tutorials/runs/09-audit-a-pass.yaml" }}

Rows sit under the rule that produced them, worst first. Read the subject as the claim and the
sentence beside it as the reason to believe it. If you cannot check a pass's reason against the
design, go and look at that pass.

`--verdicts` has two limits worth knowing before you rely on it. Some rules report violations
without stating what they examined. Such a rule is labelled "reports violations only", and its rows
are what it FOUND rather than what it checked. So absence under such a heading is not evidence of
correctness. And a pass can only cite a
value the rule actually measured, so a few rules still pass with a description of their test rather
than a number.

## When a check cannot decide

`inconclusive` is separate from every other non-pass outcome, and it is the subtlest. The check ran,
had its inputs, and still could not reach a verdict, usually because the design is ambiguous in a
way the rule cannot resolve. It is a real answer, "I looked and I cannot tell", rather than a gap
in your data or your checklist. Sometimes it is the only accurate answer. It belongs in front of a
human rather than being rounded to a pass.

## What is not on this list

There is no `waived`. A finding you have decided to accept is still a finding. The decision to
accept it belongs in your process rather than in the tool's verdict. If a tool lets you edit a report
into agreement, its reports stop being evidence.

## Next

[Compare revisions](../10-compare-revisions/), because the question is usually not "is this board
good" but "what changed since the last one".

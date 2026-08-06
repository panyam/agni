---
title: "Checks contract"
description: "The check-result document: what a run produced, in a form that outlives the run."
---

A check run produces evidence about a design. Until now that evidence only existed as terminal
output or as an RPC response, which means it could not be archived, mailed, diffed against last
week's run, or read by anyone who did not have the design and this build of the engine. The checks
contract is the schema that makes it an artifact.

It is the engine's fourth contract, and it has the same shape as the other three.

| Contract | The claim |
|---|---|
| `agni.v1.ir` | One design IR, N format readers |
| `agni.v1.geom` | One geometry sidecar, N producers and N renderers |
| `agni.v1.param` | One parameter shape, N datasheet extractors |
| `agni.v1.checks` | One check-result document, N checkers and N consumers |

## Where the messages live, and why they moved

`Finding`, `Subject`, `DatasheetCitation`, `CheckReport`, and the per-item review outcomes used to
be declared in the web API package, next to request messages carrying a mount and a path. They were
already the canonical shapes: the CLI's `check --format json` emits the same `Finding` the browser
receives. But "canonical" was a convention held up by one conversion function, and a document meant
to be written to disk had nowhere to be defined except a package full of transport types.

They now live in `agni.v1.checks`, which declares no service and imports no transport. The web API
references them. `service.FindingProto` is still the single place a `check.Finding` becomes its wire
form, so there is still exactly one conversion site, and now it produces a type that does not belong
to any one caller.

## The document

`CheckResults` is deliberately self-contained. It carries:

- **`meta`**: what produced it, at what build, when. A results document is only comparable to
  another once you know which tool and which build made each.
- **`design`**: the source it was about and a content hash of that source. The hash is the revision
  identity, which is what makes a findings diff between two revisions meaningful and what stops a
  stale document from being silently read against a design that has since changed.
- **`run`**: which overlay tiers were attached. Without this a reader cannot tell a design with no
  datasheet violations from a run that had no datasheet corpus.
- **`catalog`**: the rules that actually ran. This is what distinguishes a clean design from a run
  that checked nothing.
- **`findings`**, and for a review run **`areas`** of per-item outcomes.

Self-containment is an acceptance test rather than an aspiration. Writing a run and re-rendering it
from the document alone reproduces the original output byte for byte, and the test proves it by
deleting the design first.

```
agni check design.kicad_sch --format markdown --results-out run.results.json
rm design.kicad_sch
agni results run.results.json --format markdown   # identical
```

The parity is structural, not asserted: `agni results` renders through the same writers `agni check`
and `agni review` use, and the severity pivot lives in one function both a live run and a reloaded
document call. Two writers held equal by a test drift the first time someone edits one of them.

## The outcome vocabulary is the interesting part

`pass` and `fail` are the two verdicts a flat violation list can express, and that is all any
incumbent DRC or ERC report carries. The review layer adds verdicts for every distinct way a
question went unanswered: `not-applicable`, `not-automated`, `needs-data`, `needs-design-intent`,
`computed-n/a`, and `provisional` for an answer resting on data not yet trustworthy. Each of those
exists because a check that did not evaluate had been scoring as a pass.

That asymmetry is a feature of the schema, not a gap in it. When a foreign tool's results are
imported, they must arrive visibly weaker than a native run rather than having a coverage axis
manufactured for them. `outcome` is a string rather than an enum for the same reason `severity` is:
a new honest verdict should be a review-layer change, not a schema migration.

## Versioning

`meta.schema` is checked on read, and an unrecognized version is an error rather than a best-effort
parse. Half-reading a future document would yield a findings list shorter than the run that produced
it, with nothing to say so — the same silence-reads-as-coverage failure the outcome vocabulary
exists to prevent. Unknown *fields* within a known schema are tolerated, because additive fields do
not change what an older reader understands.

## What is not here yet

The rule-DEFINITION half of this contract — the declarative source a rule compiles from, so that
`check.Spec`, a datalog query, an interface profile, and a foreign rule deck become four front-ends
onto one form — is the sibling piece and is not in this package yet. It is deliberately absent
rather than sketched: a declared message nothing populates is the same failure class as a review
binding that silently resolves to zero rules.

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
  identity. It makes a findings diff between two revisions meaningful, and it stops a stale document
  from being silently read against a design that has since changed.
- **`run`**: which overlay tiers were attached. Without this a reader cannot tell a design with no
  datasheet violations from a run that had no datasheet corpus. It is derived from the RESOLVED
  overlay, never from the caller's flags, because those are two different questions once a design
  belongs to a project: the run composes the project's config whether or not anyone passed
  `--params`. Building it from the caller's own flags is a bug this document already shipped, and it
  failed in the reassuring direction (see [Provenance is read off the resolved
  overlay](#provenance-is-read-off-the-resolved-overlay)).
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

## A flat findings list says which rules could not run

`CheckDesignResponse` carries `skipped`: the selected rules `check.Available` gated on this design,
each with the reason the rule itself gave.

It exists because silence reads as coverage, one tier below where the outcome vocabulary fixes it. A
board rule on a netlist, or a datasheet rule with no corpus, is gated before it evaluates and
contributes no findings, and a findings list has no way to distinguish "checked and clean" from
"never ran". That lands on the viewer's default-open panel, so it is the first thing most people see
and the last thing they would think to doubt.

It is deliberately NOT the outcome vocabulary below. A flat rule sweep has no checklist item to
score, so it reports which rules were gated and why, and nothing more. `check.Available` is asked here
with the MODEL, where `ListRules` asks it with a nil one: "can this rule ever run" and "did it run on
this design" are different questions, and only the second can tell a reader their result is narrower
than their selection.

The results document carries it too, beside `catalog`, so `agni check --format json` and an
`agni results` re-render both show it and self-containment holds. An IMPORTED vendor report leaves it
empty, on the same terms `meta.coverage_axis` is false for one: a foreign checker has no notion of a
rule it declined to run, and manufacturing the field would give an import a property it does not have.

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

### Covered and answered are two numbers, and the gap between them is the point

`Tally` derives two counts over those outcomes, and a reader who treats them as one will draw a wrong
conclusion from a real report.

`Covered()` is `Total - NotAutomated`: how many items a MECHANISM exists for. It moves when a rule
leaves the catalog, as a moved profiles directory or a renamed conventions file makes it do.

`Answered()` is `Pass + Fail + Provisional + ComputedNA`: how many items the run actually DECIDED. It
moves for a second reason, and that reason is invisible to the first count. A rule can be present,
selected, and unable to run, because `check.Available` gates it on a fact tier the model does not
carry. That item reads `not-applicable`, which `Covered()` counts as covered.

The split matters because the two failure modes look identical in a findings list and only one of them
is visible in the coverage number. Measured on `examples/tutorial-project/`, removing the `params/`
directory moves `Covered()` by **zero** while a datasheet-backed item stops being answered:

```
with params/     **13 of 15 covered**, **13 answered** — 3 pass,  9 fail, 0 n/a; 2 not-automated
without          **13 of 15 covered**, **12 answered** — 2 pass, 10 fail, 1 n/a; 2 not-automated
```

The assignment worth arguing about puts `ComputedNA` on the answered side and leaves `NotApplicable`
off it. They are opposite events wearing the same word. `computed-n/a` is the DESIGN settling the
question (no crystal on this board, so the crystal rule does not apply), the branch a human reviewer
takes and a real determination. `not-applicable` is the rule's inputs being absent, so the question
goes unasked. `NeedsData`, `NeedsDesignIntent` and `Inconclusive` are on the unanswered side for the
same reason.

Both numbers are rendered, and `agni review` gates on the second (`--min-answered`). Gating on
`Covered()` would have shipped a flag that cannot see the case it was built for.

### Provenance is read off the resolved overlay

`run` records what the run HAD, so it has to be derived from the same value the run used. That is
`service.Overlay`, after the project's config and the request's own have been composed onto the
deployment's. It is not the caller's flags, and the two stopped agreeing the moment a project could
supply config: a design under a project declaring `conventions.yaml`, `profiles/` and `params/` is
scored against all three by `agni check designs/gateway` with no flags at all.

`Overlay.Provenance` is the one place that value is computed and `RunConfigProto` the one place it
becomes the message, so the CLI's check path and the service's review path cannot describe one run
differently. It UNIONS the deployment's tiers with the overlay's, because a server started with
`--profile-path` serving a project that also declares profiles genuinely ran both. The naming
convention is the exception and does not union: a request-supplied one has already REPLACED whatever
was in place by then, so the deployment's name is only the fallback.

Which tier a rule source came from is not recoverable after composition, since a compiled interface
profile and a compiled intent declaration are both just rules in a catalog. That is why the flags
travel on `ProjectConfig` rather than being derived from `Overlay.Sources`.

The failure direction is why this is worth stating. Recording `false` for a corpus that WAS attached
makes a clean report read as better founded than it is, and nothing in the document contradicts it
except the `catalog` snapshot, which nobody cross-reads. It is the same silence-reads-as-coverage
shape the outcome vocabulary exists to remove, one layer up.

## The considered set: what a rule looked at, not only what failed

A `Finding` is a violation, so a pass emits nothing and the set of subjects a rule EXAMINED is
recorded nowhere. That answers "what is wrong with this board" and cannot answer "prove this pin is
fine", which is what a reviewer asks of a design they are signing off. `CheckDesignResponse.verdicts`
carries the second answer.

A `Verdict` is what one rule concluded about ONE subject, including the subjects it could not judge,
so the verdict list IS the considered set. There is no separate coverage structure to keep in step.

**Two things share the word "verdict" and are not the same thing.** The section above uses it for the
REVIEW layer's per-item outcomes (`not-applicable`, `needs-data`, and the rest), which answer "did we
get an answer to this question" and are mostly decided by preconditions around a rule. `check.Verdict`
answers "what did this rule conclude about this thing", decided inside it. The two vocabularies
deliberately do not reuse each other's spellings, and a consumer must not map one onto the other.

The check-layer outcomes:

| Outcome | Means |
|---|---|
| `PASS` / `FAIL` | the comparison was made, and which side the design is on |
| `NO_LIMIT` | the comparison was reached and nothing stated a bound, so nothing was checked |
| `NOT_CONSIDERED` | the rule never reached a comparison; `reason` names the step that stopped it |
| `INCONCLUSIVE` | the rule reached its decision and could not decide |

`NO_LIMIT` and `NOT_CONSIDERED` are the two that did not exist before, and both are the same failure
this document keeps returning to. A datasheet row stating no maximum used to be indistinguishable
from a design sitting comfortably under a real limit, and an enumerator that dropped a subject
reported the same nothing as a rule that never looked at it.

`NO_LIMIT` is not a datasheet-only outcome, even though the datasheet rules are where it started. The
question it answers is "was there a bound at all", and a project's own net-class definitions raise it
in the same shape: `netclass-track-width` reaches its comparison over a net whose classes declare no
width and has nothing to compare against, which is not a pass and is not a subject it failed to reach.
Anywhere a rule compares a measurement against a limit somebody else stated, the limit can be absent.

`INCONCLUSIVE` is the outcome form of `Finding.inconclusive`, which already shipped, and carries that
field's contract: **a consumer must not count it as a failure.**

### A witness is what makes a pass evidence

`Witness.statement` is the line a person reads. What it rests on splits in two, and the split is
load-bearing rather than bookkeeping:

- **`Witness.terms`** are labelled VALUES ("absolute maximum" = "3.6 V"). A term's value is a bare
  string, so nothing can resolve it to something drawable.
- **`Verdict.context`** are typed ENTITIES, carrying the `Subject` a highlight joins on.

The test is whether clicking it should light something up. A proof that is entirely a path (a pull-up
reaching a rail through a resistor) therefore carries no terms at all, which is correct rather than a
gap. `context` excludes the subject, which `subject` already names, so a consumer draws
subject-as-figure over context-as-ground.

### `Verdict.id` is derived, never assigned

`"<rule>:<kind>:<ref>"`, computed from the verdict rather than handed to it, so a CLI run and a
server run name the same verdict without talking to each other. It is the `mount://` parity argument
one level down.

Built from the rule, the kind and the kind's own reference, and from nothing else: not run order, not
the message text, and **not the outcome**. Leaving the outcome out is what lets a link filed against a
passing check survive the answer flipping, which is when someone most wants to follow it. The ref's
grammar belongs to the kind rather than being a positional tuple of every kind's fields, because
`Subject` is already a widening union and a positional key would change format every time a kind is
added.

Rule names are kebab-case and kind is a closed vocabulary, so neither contains a colon: split on the
FIRST TWO colons and take the remainder verbatim. That handles `symbol:Library:Symbol`, whose colon
`KindSymbol` documents as a real spelling.

Known limit: two nets sharing a name share an id, because using the net id instead would make the id
unconstructible. That matches how `Subject` already behaves on the wire.

### Findings are unchanged

`findings` carries exactly what it always did. A verdict list is a different answer to a different
question, and folding passes into the violations list would make every consumer that counts rows
start counting passes as defects. The CLI keeps them apart the same way: `check --verdicts` is a
separate table, not extra rows.

Only rules that STATE a considered set contribute. A rule absent from `verdicts` is declining to say,
not reporting that it considered nothing, and a consumer must not read those the same way. That is
the distinction `skipped` draws one layer up.

Nine rules decline, each for a reason recorded beside its `Eval`, and the reasons group into three
(agni issue 391). Five have a subject that is a RELATION between two entities, which `Verdict.id` and
`Subject` have no grammar for. Three read a reader diagnostic that holds only the offenders, so there
is no set to map over. One cannot separate a pass from a missing datasheet inside its own body. The
first group is the one that would change this contract: a ref naming two entities means a new field on
`Subject`, which `TestVerdictFieldCensus` exists to keep a decision rather than a drift.

## Versioning

`meta.schema` is checked on read, and an unrecognized version is an error rather than a best-effort
parse. Half-reading a future document would yield a findings list shorter than the run that produced
it, with nothing to say so, the same silence-reads-as-coverage failure the outcome vocabulary
exists to prevent. Unknown *fields* within a known schema are tolerated, because additive fields do
not change what an older reader understands.

## Importing another tool's results

A results document is only a contract if more than one tool can produce one. `agni import-results`
reads a `kicad-cli pcb drc --format json` or `sch erc --format json` report into the same document
shape, so `agni results` renders it with no special case.

```
agni import-results erc.json --design board.kicad_pro -o theirs.json
agni results ours.json --compare theirs.json
```

Three things about it are deliberate.

**It is not a `formats` reader.** Every capability on that registry answers a question about a design
file: give me its netlist, its geometry, its board. A results file describes a design it does not
contain and cannot answer any of them. Registering it would make the capability set mean two
different things, so the import is a separate path. The Loader's job is producing a model, and this
produces evidence *about* one.

**The imported document is visibly weaker, and says so.** A vendor report is a flat violation list.
It has no not-applicable, no needs-data, no coverage axis, and no per-item traceability, because
those came out of the review work and no incumbent has them. So `meta.coverage_axis` is false, and
every report that shows an imported document labels it. The difference is invisible in the data,
since an import and a clean native run both look like "few findings", so it has to be declared
rather than inferred.

**The residue is reported, not dropped.** A foreign checker names entities in free text ("Pad 1
[VCC] of R1 on B.Cu"), so attaching a violation to our model is a parse, and a parse has a residue: a
schematic wire's description carries only its orientation and length. Unattached findings are kept
and counted by class in `import_summary`, because a consumer seeing 40 imported findings has to be
able to tell "the tool found 40 things" from "the tool found 60 and we understood 40". A parsed
ref-des that names no component in the loaded design leaves the finding unattached rather than
inventing a subject. A wrong join attaches a real violation to an innocent part, and that is worse
than no join.

### The oracle becomes a harness

Verifying rule semantics against kicad-cli was already standing practice, and it has repeatedly paid:
it is what caught mid-span labels, endpoint-only pin connections, and the brace escapes when unit
tests did not. Every one of those was a person reading two outputs side by side. `--compare` makes it
a gate.

The split is keyed on the **entity** each tool flagged, not on rule names. Two tools have two rule
vocabularies, and a table asserting "our `track-width` means their `track_width`" would be an
unverified mapping that rots, the same objection that killed identifying an interface host by an MPN
prefix list. What can be said without asserting anything is: here is the set of entities we flagged,
here is theirs, here is the overlap. Rule co-occurrence is then reported as an *observation*, so an
equivalence can be discovered from evidence instead of declared up front.

A pin finding keys to its component, because one tool flagging "R1 pin 2" and another flagging "R1"
is a difference in reporting granularity, not a disagreement.

## The other half: rule definitions

A results document says what a run found. The rule-definition half says what a rule *is*, in a form
that is data rather than code.

`check.Rule` is not that form and must not become it: a Rule carries an `Eval` closure, and a Go func
has no wire form. Reaching for one would mean shipping code as data or amputating the escape hatch
that makes the catalog practical. The serializable artifact is the rule's **source**, and compiling is
exactly the step that produces the non-serializable part. The engine already made that split in three
places, and `ruledef.proto` is the union of their inputs.

| Declarative source | Compiler | Runtime, never serialized |
|---|---|---|
| `check.Spec` | `Spec.Rule()` | `*check.Rule` with derived `Reads`/`Primitives` |
| `query.Query` | `query.RuleFromQuery` | `*check.Rule` registered as `dl/…` |
| `profiles.Profile` | `profiles.Compile` | one `*check.Rule` per requirement, `profile/…` |

So the layering is three tiers, not two: a rule definition, the `RuleInfo` catalog projection of a
compiled rule, and the Go runtime object. A rule with a hand-written Go `Eval` and no declarative twin
is **outside this contract by design**, not a gap in it.

A `RuleDef` compiles to one rule *or more*. A spec and a query each yield one; an interface profile
yields one per requirement. That asymmetry is the profile mechanism working, with one declaration
standing in for a family of near-identical checks, so the signature admits it rather than making
every caller pretend otherwise.

Each of those rules records which requirement produced it, in a `requirement` tag alongside the
`profile` tag naming the interface. The pair is what lets a consumer address one ask of an interface
rather than all of them: a review item binding `profile: CAN` selects the union of everything CAN
compiles, while `profile: CAN` plus `requirement: esd` selects the one rule that answers that ask.

The reason to record it as a tag rather than read it off the rule name is that a profile has to be
able to GROW. Under union semantics alone, adding a requirement re-scores every item already bound to
that profile: they all start reporting a defect none of them describes, so an interface's checks
become effectively frozen once more than one item shares it. Selecting narrows the item to its own
requirement without giving up the profile's presence gate. A bare rule binding gives that gate up,
and an absent interface then reads as a hollow pass instead of not-applicable. A requirement the
profile does not declare resolves to no rule and reads not-automated, on the same discipline as
everything else here.

### The FFI boundary is what keeps this honest

A spec needing behavior the vocabulary cannot express calls a registered function **by name**. The
name is data; the Go function behind it is not. So a rule definition serializes to a closed vocabulary
plus named references into a registry, the same posture the vendor-rule survey takes on arbitrary
scripted checks. The escape hatch exists, it is bounded, and covering general code verbatim is a
non-goal, because a scripted check is exactly as reviewable as the code inside it.

### Everything that cannot run is rejected when it is read

An unknown entity set, an unknown fact, an unbound variable, an unregistered function, an unknown
relation, an unknown requirement type, a requirement whose params cannot produce the check its type
promises, an over-broad signal matcher, a completeness requirement with no anchor. Each of those
would otherwise compile to a rule that never fires, and a rule that never fires is indistinguishable
from a design with nothing wrong with it. A deck stops at the first bad
definition rather than loading partially, for the same reason: a catalog missing one rule looks
exactly like a catalog that ran it and found nothing.

Where the check can teach, it does. An unknown relation names the closest one in the catalog.

## Reading a foreign rule deck: `.kicad_dru` on paper

The point of a neutral definition form is that a vendor's rule file becomes a front-end rather than a
parallel path. `.kicad_dru` is the honest test case: it is the one incumbent rule language that is
open, documented, and licensed for study. Mapping it is worth doing on paper *before* building the
front-end, because the interesting result is which layer has to change.

Measured against the licensed 31-rule JLCPCB deck in the private rule corpus:

| `.kicad_dru` construct | Our form | Verdict |
|---|---|---|
| `(rule "<name>")`, `(severity …)` | `RuleMeta.name`, `.severity` | direct |
| `(condition "<expr>")` | `SpecBody.where` | direct in shape |
| `(constraint <kind> (min X))` | `SpecCmp` over the kind's fact | direct where the fact exists |
| `A.Type`, `A.Pad_Type`, `A.isPlated()`, `A.NetClass` | — | no fact |
| `(layer "F.Cu")` | — | no layer scope |
| Anything naming `B` | — | no second entity in scope |

Of the 31 rules, **17 are single-item** and **14 are pairwise** (they reference a second item `B`:
6 `clearance`, 4 `hole_to_hole`, 3 `hole_clearance`, 1 `silk_clearance`). The pairwise half does not
map at all, and that is not an oversight. A `Spec` binds exactly one entity, so `copper-clearance`
ended up the one board rule with a hand-written Go `Eval`. A pairwise spatial join has
not yet earned AST nodes.

Of the 17 single-item rules, the constraint kinds with a shipped fact are `track_width` (2, via
`segment.width`), `annular_width` (4, via `via.annular`), and the via subset of `hole_size` (via
`via.drill`). The rest name item properties we do not model: pad geometry and plating for the pad
`hole_size` rules, silkscreen text metrics for `text_thickness` and `text_height`, board-edge distance
for `edge_clearance`, and blind/buried/micro-via predicates for the one `assertion`.

**The conclusion is the useful part: the definition schema does not need to change.** The shape of a
`.kicad_dru` rule, which names a rule, states a condition, and makes a parametric comparison, is
already `RuleMeta` plus `SpecBody.where` plus a `SpecCmp`. What is missing is the **fact vocabulary** (pad, layer, text, and
board-edge facts) and **a two-entity scope**. Both are additions to the spec language rather than to
this contract, and both have to clear the same bar every fact does: model the concept, not one
vendor's spelling of it, and promote only when more than one source needs it.

The corollary is a warning. A deck whose constraint kinds have no shipped counterpart must not
evaluate clean. Loading 31 rules and silently running 6 of them would report a fab-capability pass
that was never checked, the same false-pass failure the review outcomes exist to prevent. So
load-time rejection is total rather than best-effort.

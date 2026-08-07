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
different things, so the import is a separate path — the Loader's job is producing a model, and this
produces evidence *about* one.

**The imported document is visibly weaker, and says so.** A vendor report is a flat violation list.
It has no not-applicable, no needs-data, no coverage axis, and no per-item traceability, because
those came out of the review work and no incumbent has them. So `meta.coverage_axis` is false, and
every report that shows an imported document labels it. The difference is invisible in the data —
an import and a clean native run both look like "few findings" — which is exactly why it has to be
declared rather than inferred.

**The residue is reported, not dropped.** A foreign checker names entities in free text ("Pad 1
[VCC] of R1 on B.Cu"), so attaching a violation to our model is a parse, and a parse has a residue: a
schematic wire's description carries only its orientation and length. Unattached findings are kept
and counted by class in `import_summary`, because a consumer seeing 40 imported findings has to be
able to tell "the tool found 40 things" from "the tool found 60 and we understood 40". A parsed
ref-des that names no component in the loaded design leaves the finding unattached rather than
inventing a subject — a wrong join attaches a real violation to an innocent part, which is worse than
no join.

### The oracle becomes a harness

Verifying rule semantics against kicad-cli was already standing practice, and it has repeatedly paid:
it is what caught mid-span labels, endpoint-only pin connections, and the brace escapes when unit
tests did not. Every one of those was a person reading two outputs side by side. `--compare` makes it
a gate.

The split is keyed on the **entity** each tool flagged, not on rule names. Two tools have two rule
vocabularies, and a table asserting "our `track-width` means their `track_width`" would be an
unverified mapping that rots — the same objection that killed identifying an interface host by an MPN
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
yields one per requirement. That asymmetry is the profile mechanism working — one declaration standing
in for a family of near-identical checks — so the signature admits it rather than making every caller
pretend otherwise.

### The FFI boundary is what keeps this honest

A spec needing behavior the vocabulary cannot express calls a registered function **by name**. The
name is data; the Go function behind it is not. So a rule definition serializes to a closed vocabulary
plus named references into a registry — the same posture the vendor-rule survey takes on arbitrary
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

Where the check can teach, it does — an unknown relation names the closest one in the catalog.

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
map at all, and that is not an oversight: a `Spec` binds exactly one entity, which is precisely why
`copper-clearance` is the one board rule with a hand-written Go `Eval`. A pairwise spatial join has
not yet earned AST nodes.

Of the 17 single-item rules, the constraint kinds with a shipped fact are `track_width` (2, via
`segment.width`), `annular_width` (4, via `via.annular`), and the via subset of `hole_size` (via
`via.drill`). The rest name item properties we do not model: pad geometry and plating for the pad
`hole_size` rules, silkscreen text metrics for `text_thickness` and `text_height`, board-edge distance
for `edge_clearance`, and blind/buried/micro-via predicates for the one `assertion`.

**The conclusion is the useful part: the definition schema does not need to change.** The shape of a
`.kicad_dru` rule — a named rule, a condition, a parametric comparison — is already `RuleMeta` plus
`SpecBody.where` plus a `SpecCmp`. What is missing is the **fact vocabulary** (pad, layer, text, and
board-edge facts) and **a two-entity scope**. Both are additions to the spec language rather than to
this contract, and both have to clear the same bar every fact does: model the concept, not one
vendor's spelling of it, and promote only when more than one source needs it.

The corollary is a warning. A deck whose constraint kinds have no shipped counterpart must not
evaluate clean. Loading 31 rules and silently running 6 of them would report a fab-capability pass
that was never checked, which is the same false-pass failure the review outcomes exist to prevent —
which is why load-time rejection is total rather than best-effort.

---
title: "Checks contract"
description: "The check-result document: what a run produced, in a form that outlives the run."
---

A check run produces evidence about a design. Before this contract, that evidence existed only as
terminal output or as an RPC response. Nobody could archive it, mail it, diff it against last
week's run, or read it without the design and this build of the engine. The checks contract defines
a schema for it, so the evidence becomes an artifact.

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
already the canonical shapes, since the CLI's `check --format json` and the browser receive the same
`Finding`. But only one conversion function held that convention up. A document meant to be written
to disk had nowhere to be defined except a package full of transport types.

They now live in `agni.v1.checks`, which declares no service and imports no transport. The web API
references them. A `check.Finding` still becomes its wire form in one place, `service.FindingProto`,
so there is still exactly one conversion site. Now that site produces a type that does not belong to
any one caller.

## The document

`CheckResults` is deliberately self-contained. It carries:

- **`meta`** records what produced it, at what build, and when. A results document is only comparable to
  another once you know which tool and which build made each.
- **`design`** names the source it was about and carries a content hash of that source. The hash
  is the revision identity. It makes a findings diff between two revisions meaningful, and it stops a
  stale document from being silently read against a design that has since changed.
- **`run`** records which overlay tiers were attached. Without it a reader cannot tell a design with no
  datasheet violations from a run that had no datasheet corpus. It is derived from the RESOLVED
  overlay, never from the caller's flags, because the two differ once a design belongs to a project.
  The run composes the project's config whether or not anyone passed `--params`. This document once
  shipped a `run` built from the caller's own flags, and that bug failed in the reassuring direction
  (see [Provenance is read off the resolved overlay](#provenance-is-read-off-the-resolved-overlay)).
- **`catalog`** lists the rules that actually ran. It separates a clean design from a run that
  checked nothing.
- **`findings`** holds the findings, and for a review run **`areas`** holds the per-item outcomes.

Self-containment is an acceptance test rather than an aspiration. Writing a run and re-rendering it
from the document alone reproduces the original output byte for byte, and the test proves it by
deleting the design first.

```
agni check design.kicad_sch --format markdown --results-out run.results.json
rm design.kicad_sch
agni results run.results.json --format markdown   # identical
```

Parity holds by construction. `agni results`, `agni check` and `agni review` render through the
same writers, and a live run and a reloaded document call one function for the severity pivot. Two
writers held equal by a test drift the first time someone edits one of them.

## A flat findings list says which rules could not run

`check.Available` gates some selected rules on a design. `CheckDesignResponse` carries `skipped`,
which lists those rules, each with the reason the rule itself gave.

Picture a board rule on a {{ explainable "netlist" }}, or a datasheet rule with no corpus. Either
one is gated before it evaluates and contributes no findings. A findings list has no way to
distinguish "checked and clean" from "never ran", so the silence reads as coverage. The outcome
vocabulary fixes the same problem one tier up. This one shows on the viewer's default-open panel,
so it is the first thing most people see and the last thing they would think to doubt.

`skipped` is deliberately NOT the outcome vocabulary below. A flat rule sweep has no checklist item
to score, so it reports which rules were gated and why, and nothing more. Here the run asks
`check.Available` with its MODEL. `ListRules` asks the same gate of that model built without its
board. So when `ListRules` calls a rule available, a whole run of the same snapshot does not skip
it (agni issue 950). The viewer selects the available rules and runs those. A listing that
disagreed with the run either hid a rule's findings or offered a checkbox the run would skip. Every
datasheet-tier rule had its findings hidden that way in agni issue 940. The listing reads no board,
since `ListRules` answers on every open (agni issue 943). The board tier's format decides whether
the run attaches a board, the same way it decides the Board sheet. `skipped` still matters when the
listing is right. A run of an explicit rule list can name a rule even though the listing called it
unavailable. A scoped run sets design-wide rules aside, and `skipped` names them.
`TestListRulesAvailableIsWhatARunRuns` holds the agreement on the server and the browser engine
alike.

The results document carries `skipped` too, beside `catalog`, so `agni check --format json` and an
`agni results` re-render both show it and self-containment holds. An IMPORTED vendor report leaves
it empty, and `meta.coverage_axis` is false for one on the same terms. A foreign checker has no
notion of a rule it declined to run. Manufacturing the field would give an import a property it
does not have.

## The outcome vocabulary names every way a question went unanswered

A flat violation list can express two verdicts, `pass` and `fail`, and any incumbent DRC or ERC
report carries only those. The review layer adds verdicts for every distinct way a question went
unanswered. They are `not-applicable`, `not-automated`, `needs-data`, `needs-design-intent`,
`computed-n/a`, and `provisional` for an answer based on data that is not yet trustworthy. Each of
those exists because a check that did not evaluate had been scoring as a pass.

The schema keeps that asymmetry on purpose. When a foreign tool's results are
imported, they must arrive visibly weaker than a native run rather than having a coverage axis
manufactured for them. `outcome` is a string rather than an enum for the same reason `severity` is:
a verdict naming a new way to go unanswered should be a review-layer change, not a schema migration.

### Covered and answered are two numbers

`Tally` derives two counts over those outcomes, and a reader who treats them as one will draw a wrong
conclusion from a real report.

`Covered()` is `Total - NotAutomated`, the count of items that have a MECHANISM. It moves when a
rule leaves the catalog, for example when a profiles directory moves or a conventions section is
deleted.

`Answered()` is `Pass + Fail + Provisional + ComputedNA`, the count of items actually DECIDED in the
run. It
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

One assignment is worth arguing about. `ComputedNA` counts as answered and `NotApplicable` does not.
They are opposite events wearing the same word. In `computed-n/a` the DESIGN settles the question.
The board has no crystal, so the crystal rule does not apply. A human reviewer takes the same
branch, and it is a real determination. In `not-applicable` the rule's inputs are absent, so the
question goes unasked. `NeedsData`, `NeedsDesignIntent` and `Inconclusive` are on the unanswered
side for the same reason.

{{ includeFile "figures/covered-vs-answered.svg" }}

Both numbers are rendered, and `agni review` gates on the second (`--min-answered`). Gating on
`Covered()` would have shipped a flag that cannot see the case it was built for.

### Provenance is read off the resolved overlay

`run` records what the run HAD, so it has to be derived from the value the run used. The run used
`service.Overlay`, after the project's config and the request's own have been composed onto the
deployment's. The caller's flags are a different value, and the two stopped agreeing the moment a
project could supply config. Take a design under a project that declares a `conventions:` section,
`profiles/` and `params/`. `agni check designs/gateway` scores it against all three with no flags
at all.

`Overlay.Provenance` alone computes that value, and `RunConfigProto` alone turns it into the
message. So the CLI's check path and the service's review path cannot describe one run differently.
Picture a server started with `--profile-path`, serving a project that also declares profiles. That
server genuinely ran both, so `Overlay.Provenance` UNIONS the deployment's tiers with the overlay's.
The naming convention is the exception and does not union. By then a request-supplied convention
has already REPLACED whatever was in place, so the deployment's name is only the fallback.

<details>
<summary>Why the flags travel on the config rather than being derived from the sources</summary>

Which tier a rule source came from is not recoverable after composition, since a compiled interface
profile and a compiled intent declaration are both just rules in a catalog. So the flags travel as
`Overlay.Profiles` and `Overlay.Intent` rather than being derived from `Overlay.Sources`.

</details>

Suppose `run` records `false` for a corpus that WAS attached. A clean report then reads as better
founded than it is. Only the `catalog` snapshot contradicts it, and nobody cross-reads that. The
outcome vocabulary exists to remove the same silence-reads-as-coverage shape, one layer up.

## The considered set records what a rule looked at, not only what failed

A `Finding` is a violation, so a pass emits nothing, and nothing records the set of subjects a rule
EXAMINED. The findings answer "what is wrong with this board" and cannot answer "prove this pin
is fine". A reviewer signing off a design asks the second question, and
`CheckDesignResponse.verdicts` carries its answer.

A `Verdict` records what one rule concluded about ONE subject, including the subjects it could not
judge, so the verdict list IS the considered set. There is no separate coverage structure to keep in
step.

{{ includeFile "figures/findings-versus-verdicts.svg" }}

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

`NO_LIMIT` and `NOT_CONSIDERED` are new, and both name the failure that recurs throughout this
document. A datasheet row stating no maximum used to be indistinguishable
from a design sitting comfortably under a real limit, and an enumerator that dropped a subject
reported the same nothing as a rule that never looked at it.

`NO_LIMIT` started in the datasheet rules, but it is not a datasheet-only outcome. It answers "was
there a bound at all", and a project's own net-class definitions raise that question too. Say
`netclass-track-width` reaches its comparison over a net whose classes declare no width. The rule
has nothing to compare against. That is not a pass, and the rule did not fail to reach the subject.
Anywhere a rule compares a measurement against a limit somebody else stated, the limit can be absent.

`INCONCLUSIVE` is the outcome form of `Finding.inconclusive`, which already shipped, and carries that
field's contract: **a consumer must not count it as a failure.**

### A datalog rule declares its domain rather than deriving it

A rule authored as a datalog query (`query.RuleFromQuery`) has a problem the hand-written rules do
not. A goal yields the rows that MATCHED, so a subject that did not match is not in the answer at
all (`unterminated(?h)` produces {{ explainable "termination" "unterminated" }}
{{ explainable "bus" "buses" }} and nothing produces the terminated ones). For a long time every rule
compiled by this bridge reported failures only. That included the whole `profile/` and `dl/`
families.

A `FindingQuery` now carries an optional `Domain`, a second goal over the same program. Its rows
list every subject EXAMINED by the rule. The bridge evaluates both, reports the difference as
passes, and sets `StatesConsideredSet`. Leaving it unset keeps the old shape, so a rule that
declares nothing claims nothing.

**The domain is declared by the author and never inferred from the goal.** A wrong derivation
OVERSTATES coverage, which is worse than stating none, and nothing downstream can act on four right
answers out of six. A declaration fails safe.

<details>
<summary>The three rules that defeat the obvious inference</summary>

The inference looks easy. For the ESD requirement the domain is exactly the goal's body minus its
negated literal. Three other rules break that inference.

- `signal-dangling` ends in a COMPARISON with no negation, so "the body minus its negation" is the
  body, and the rule would report the subjects it faulted as the subjects it considered.
- `signal-missing` carries TWO negated literals. `not has_signal(S)` is the test; the host guard
  `not any_host` is scope, because a design that declares a host is covered by the precise path and
  was never in this rule's domain. Nothing in the syntax separates them.
- `crystal-load-caps` negates its exemption (`not net.external`) beside its test. Sweeping the
  exemption into the domain would report a terminal leaving the board as a PASS, claiming the rule
  confirmed a capacitor it never looked for.

</details>

The same rule applies to a capability gate. `power-pin-mistyped` keeps `design.has_nc_channel` in its
domain, because that predicate asks whether the FORMAT can answer the question rather than testing
the pin. A pin read from EDIF was never judged, and putting it in the considered set would report
every supply pin as verified by a rule that is structurally silent there.

### A witness makes a pass evidence

`Witness.statement` is the line a person reads. Its support splits in two, and only one half can
be drawn.

- **`Witness.terms`** are labelled VALUES ("absolute maximum" = "3.6 V"). A term's value is a bare
  string, so nothing can resolve it to something drawable.
- **`Verdict.context`** are typed ENTITIES, carrying the `Subject` a highlight joins on.

The test is whether clicking it should light something up. Some proofs are entirely a path, such
as a {{ explainable "pull-up" }} reaching a {{ explainable "rail" }} through a resistor. Such a
proof carries no terms at all, and that is correct rather than a gap. `context` excludes the
subject, since `subject` already names it. A consumer draws subject-as-figure over
context-as-ground.

**A FAILING verdict needs a witness too, and forgetting one is silent in-process.** The wire form of
a `Verdict` deliberately carries no `Finding`, because a defect travels once in `findings`. Suppose
a failing verdict has its only sentence on its `Finding`. A consumer holding the Go value renders it
correctly. Every consumer that reads verdicts back from the service renders it BLANK, and the CLI's
own verdict report is one of them. `check.FailuresOnly` omits the witness on purpose, since an
unconverted rule has no proof to show; the moment a rule converts, that omission becomes a bug.
Regenerating the docsite captures caught this bug, and no test did. So converting a rule means
looking at rendered output and not only at a green package.

### A verdict's subject is a TUPLE

`verdict.subjects` lists the entities the rule quantified over, in the rule's own order. Most rules
name one. Some rules ask about a RELATION, and such a rule names every entity in it, because a
relation belongs to none of them alone. A clearance violation is a distance between two nets. A
regulator feeding a part can over-drive it, and only the regulator, the rail and the load together
pin that down. A {{ explainable "strap" }} group is a device plus the N nets encoding its value.
Naming fewer would give several answers one id.

Order is the rule's and is significant, since a tracking bound reads subject-pin minus reference-pin.
A symmetric relation is canonicalised by the rule before it reaches the wire, never by a consumer.

A `Finding.subject` is still SINGULAR and is one of these. The two answer different questions. The
tuple is the verdict's identity. A finding's subject is the one entity a reader has to change. The
finding lists every other entity in its `context`, and a consumer that wants equal standing across
all of them looks there.

**A finding names the verdict it came from in `Finding.verdict_ids`** (agni 485), because the id is not
recoverable from the finding. A clearance verdict is keyed on two nets and its finding on one, a
supply verdict can be keyed on a pin the finding lists only as context, and a profile's verdict is
keyed on a `signal:` element no finding field can hold. The run stamps the id at the point where it
holds both a verdict and the finding projected from it. A consumer holding only findings, such as a
review item, then links to the real proof. A finding from a check run carries one id; the field is a
list so a consumer that folds findings together keeps them all. It is empty for a rule that states
no considered set. That rule publishes no verdicts, so an id would name nothing, and its findings
get no link.

### `Verdict.id` is derived, never assigned

The id has the form `"<rule>:(<kind>:<ref>,...)"`. It is computed from the verdict rather than
handed to it, so a CLI run and a server run name the same verdict without talking to each other.
This is the `mount://` parity argument one level down.

The id is built from the rule, the kind and the kind's own reference, and never from run order,
the message text, or **the outcome**. Leaving the outcome out lets a link filed against a passing
check survive the answer flipping. Someone most wants to follow that link right when the answer
flips. Each kind owns its ref's grammar, and the ref is not a positional tuple of every kind's
fields. `Subject` is already a widening union, so a positional key would change format every time a
kind is added.

The id is GENERATED and never parsed. One function builds it and nothing splits it back, because
the structure travels in `subjects`, where a consumer reads it typed.

<details>
<summary>What never parsing allows, and the one limit it accepts</summary>

Because nothing parses the id, a ref can keep its own colons (`symbol:Library:Symbol`) and its own
commas (an endpoint's `0,0`). Inside a ref, the four characters of the tuple syntax are
percent-escaped. An endpoint's ref has a comma in the delimiter position, and without the escape
`("A,net:B")` and `("A", "B")` are one string.

Two nets sharing a name share an id, and that is the known limit. Using the net id instead would
make the id unconstructible. That matches how `Subject` already behaves on the wire.

</details>

### Findings are unchanged

`findings` carries exactly what it always did. A verdict list is a different answer to a different
question, and folding passes into the violations list would make every consumer that counts rows
start counting passes as defects. The CLI keeps them apart the same way, printing `check --verdicts` as a
separate table rather than as extra rows.

Only rules that STATE a considered set contribute. A rule absent from `verdicts` is declining to say,
not reporting that it considered nothing, and a consumer must not read those the same way.
`skipped` draws the same distinction one layer up.

Four rules decline, each for a reason recorded beside its `Eval` (agni issue 391). Three read a reader
diagnostic that holds only the offenders, so there is no set to map over; one cannot separate a pass
from a missing datasheet inside its own body.

## Versioning

`meta.schema` is checked on read, and an unrecognized version is an error rather than a best-effort
parse. Half-reading a future document would yield a findings list shorter than the run that produced
it, with nothing to say so. The outcome vocabulary exists to prevent the same
silence-reads-as-coverage failure. Unknown *fields* within a known schema are tolerated, because
additive fields do not change what an older reader understands.

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
file, such as its netlist, its geometry or its board. A results file describes a design it does not
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
[VCC] of R1 on B.Cu"), so attaching a violation to our model is a parse, and a parse has a residue (a
schematic wire's description carries only its orientation and length). Unattached findings are kept
and counted by class in `import_summary`, because a consumer seeing 40 imported findings has to be
able to tell "the tool found 40 things" from "the tool found 60 and we understood 40". A parsed
{{ explainable "reference-designator" "ref-des" }} that names no component in the loaded design
leaves the finding unattached rather than inventing a subject. A wrong join attaches a real violation to an innocent part, and that is worse
than no join.

### The oracle becomes a harness

Verifying rule semantics against kicad-cli was already standing practice. It has repeatedly caught
mid-span labels, endpoint-only pin connections, and the brace escapes when unit tests did not. Every
one of those was a person reading two outputs side by side. `--compare` makes it a gate.

The split is keyed on the **entity** each tool flagged, not on rule names. Two tools have two rule
vocabularies, and a table asserting "our `track-width` means their `track_width`" would be an
unverified mapping that rots. The same objection ruled out identifying an interface host by an MPN
prefix list. Without asserting anything, we can say which entities we flagged, which entities they
flagged, and where the two overlap. Rule co-occurrence is then reported as an *observation*, so an
equivalence can be discovered from evidence instead of declared up front.

A pin finding keys to its component, because one tool flagging "R1 pin 2" and another flagging "R1"
is a difference in reporting granularity, not a disagreement.

## The other half: rule definitions

A results document says what a run found. The rule-definition half says what a rule *is*, in a form
that is data rather than code.

`check.Rule` is not that form and must not become it, because a Rule carries an `Eval` closure and a Go func
has no wire form. Reaching for one would mean shipping code as data or dropping the hand-written `Eval`
that makes the catalog practical. The serializable artifact is the rule's **source**, and compiling is
exactly the step that produces the non-serializable part. The engine already made that split in three
places, and `ruledef.proto` is the union of their inputs.

| Declarative source | Compiler | Runtime, never serialized |
|---|---|---|
| `check.Spec` | `Spec.Rule()` | `*check.Rule` with derived `Reads`/`Primitives` |
| `query.Query` | `query.RuleFromQuery` | `*check.Rule` registered as `dl/…` |
| `profiles.Profile` | `profiles.Compile` | one `*check.Rule` per requirement, `profile/…` |

So the layering has three tiers, not two. They are a rule definition, the `RuleInfo` catalog
projection of a compiled rule, and the Go runtime object. A rule with a hand-written Go `Eval` and
no declarative twin is **outside this contract by design**, not a gap in it.

A `RuleDef` compiles to one rule *or more*. A spec and a query each yield one; an interface profile
yields one per requirement. That asymmetry is the profile mechanism working, since one declaration
stands in for a family of near-identical checks. So the signature admits it rather than making
every caller pretend otherwise.

Each of those rules records which requirement produced it, in a `requirement` tag alongside the
`profile` tag naming the interface. The pair lets a consumer address one ask of an interface
rather than all of them. A review item binding `profile: CAN` selects the union of every rule
compiled from CAN. Binding `profile: CAN` plus `requirement: esd` selects the one rule that answers
that ask.

The requirement is recorded as a tag rather than read off the rule name because a profile has to be
able to GROW. Under union semantics alone, adding a requirement re-scores every item already bound
to that profile, and every one of them starts reporting a defect it does not describe. So once more
than one item shares an interface, its checks are effectively frozen. Selecting narrows the item to
its own requirement without giving up the profile's presence gate. A bare rule binding gives that
gate up, and an absent interface then reads as a hollow pass instead of not-applicable. A
requirement the profile does not declare resolves to no rule and reads not-automated, on the same
discipline as everything else here.

### The FFI boundary keeps a rule definition serializable

Sometimes a spec needs behavior beyond the vocabulary. It then calls a registered function **by
name**. The name is data; the Go function behind it is not. So a rule definition serializes to a
closed vocabulary plus named references into a registry. The vendor-rule survey takes the same
posture on arbitrary scripted checks. The registered-function path exists, it is bounded, and
covering general code verbatim is a non-goal, because a scripted check is exactly as reviewable as
the code inside it.

### Everything that cannot run is rejected when it is read

Loading rejects an unknown entity set, an unknown fact, an unbound variable, an unregistered
function, an unknown relation, an unknown requirement type, a requirement whose params cannot produce the check its type
promises, an over-broad signal matcher, a completeness requirement with no anchor. Each of those
would otherwise compile to a rule that never fires, and a rule that never fires is indistinguishable
from a design with nothing wrong with it. A deck stops at the first bad
definition rather than loading partially, because a catalog missing one rule looks
exactly like a catalog that ran it and found nothing.

Where the check can teach, it does. An unknown relation names the closest one in the catalog.

## Reading a `.kicad_dru` rule deck on paper

A neutral definition form lets a vendor's rule file become a front-end rather than a
parallel path. `.kicad_dru` is the test case because it is the one incumbent rule language that is
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
map at all, because a `Spec` binds exactly one entity. For the same reason `copper-clearance` has a
hand-written Go `Eval` rather than a spec. Nothing has yet justified AST nodes for a pairwise
spatial join.

<details>
<summary>Which of the 17 single-item rules have a shipped fact today</summary>

The constraint kinds with one are `track_width` (2, via `segment.width`), `annular_width` (4, via
`via.annular`), and the via subset of `hole_size` (via `via.drill`). The rest name item properties we
do not model: pad geometry and plating for the pad `hole_size` rules, silkscreen text metrics for
`text_thickness` and `text_height`, board-edge distance for `edge_clearance`, and
blind/buried/micro-via predicates for the one `assertion`.

</details>

**The definition schema does not need to change.** A `.kicad_dru` rule names a rule, states a
condition, and makes a parametric comparison. That shape is already `RuleMeta` plus
`SpecBody.where` plus a `SpecCmp`. The spec language is missing the **fact vocabulary** (pad,
layer, text, and board-edge facts) and **a two-entity scope**. Both are additions to the spec
language rather than to this contract, and both have to clear the bar set for every fact. A fact
models the concept rather than one vendor's spelling of it, and it is promoted only when more than
one source needs it.

A deck whose constraint kinds have no shipped counterpart must not evaluate clean. Loading 31 rules
and silently running 6 of them would report a fab-capability pass that was never checked. The
review outcomes exist to prevent the same false-pass failure. So load-time rejection is total
rather than best-effort.

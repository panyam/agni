---
title: "Datasheet layer"
description: "How a part's datasheet becomes data a checker can compare a design against: the parameter and document contracts, the derivation that fills them, the demand-driven scheduling model, and the join into checks."
---

A datasheet is close to a type definition that also carries runtime limits. It states what a part is
(an LDO, an N-channel FET) and the envelope inside which it behaves as specified. This layer turns
that document into data a rule can compare a design against.

![the datasheet layer end to end]({{.Site.PathPrefix}}/static/images/datasheet/layer-stack.svg)

| | What it models | Populated by |
|---|---|---|
| **parameter-IR** | one extracted parameter with its conditions, limit kind, provenance | derivation, or a human |
| **doc-IR** | a source PDF as pages, tables, figures, text blocks | a document parser (docling today) |
| **derivation** | the deterministic function from document to parameter set | `datasheet/derive` |
| **resolution** | which parameters to extract, and when | the scheduler below (not yet built) |
| **the join** | parameter set to design, by part identity | `check.Model`'s params tier |

Each contract follows the pattern used elsewhere in the engine: one schema, many producers, the same
shape [ingestion](../ingestion-and-ir/) uses for format readers. No extractor populates the parameter
schema in production yet, and hand-encoded fixtures validate it today.

## The parameter contract

"RDS(on) = 3.5 Ω" is not a fact about a part. The fact is "RDS(on) max 3.5 Ω at VGS = 10 V,
ID = 0.22 A, TJ = 25 °C, pulse-tested", and an engine comparing a design against datasheet limits is
safe only when the schema makes the shorter form impossible to hold without noticing.

| Feature | What it prevents |
|---|---|
| **`RangeValue` has no bare-scalar form**, only a min/typ/max triple with explicit presence | "max only" (an abs-max row), "typ only" (uncharacterized), and "min/max" (an ensured band) collapsing into a lone number |
| **`LimitKind` is an enum**: `ABSOLUTE_MAX` (stress), `RECOMMENDED_OPERATING` (functional envelope), `CHARACTERISTIC` (measured under stated conditions). `UNSPECIFIED` fails validation | A rule treating a stress rating as an operating limit. The three have different safe uses, and a rule dispatches on this field |
| **`ConditionCoverage` makes under-specification explicit.** Anything not `COMPLETE` or `UNCONDITIONAL` is under-specified, and `param.UnderSpecified` says so | Comparing against a conditions-stripped value, which produces confident wrong findings. Worse than no value |

The first two carry the {{ explainable "absolute-maximum-rating" }} distinction that a design gets
wrong quietly. `Condition` captures an exact point (`eq`), a range, or a one-sided bound, always with
the source text in `raw`, so an unstructured condition is retained rather than dropped.

### Provenance

Every `Parameter` carries a `ParamProvenance`: source document (a `SourceDoc` naming vendor doc
number and revision), page, table or figure as titled, extraction method, and a confidence in the
range (0, 1]. A value an engineer cannot verify against the exact page is not usable for review, so
zero confidence is invalid by construction. Hand-encoded rows use `method: "hand"`, `confidence: 1`.
`SourceDoc.locator` is corpus-local, because the deployment posture keeps a customer's datasheets
inside their own boundary.

**A FIXTURE IS NOT A CORPUS, and the difference is size.** A fixture carries the few rows its tests
need, cited, in `datasheet/param/testdata/`, so `make testall` passes on a clean clone. A seeded
corpus is the part's actual parameter set and lives OUTSIDE this repo with its source PDFs.
Transcribing a whole vendor table because it demonstrates better is how a fixture drifts into being
an extracted parameter document: `txb0104.textproto` reached 389 lines that way and was cut back. If
a new fixture is much larger than its neighbours, that is the signal. Anything user-facing uses
SYNTHETIC parts.

### Verification, and why it expires

Provenance says how a value was PRODUCED, and cannot say whether anyone has since agreed with it.
`Parameter.verification` is the separate record of who checked, when, and against WHICH REVISION.

The revision is why this is a message rather than a boolean. A value checked against rev K is not
thereby true of rev L, and a stale verification is worse than none: an unverified value is honest
about what it is, while a stale-verified one is a confident wrong answer with a person's name
attached, trusted precisely because someone did once check it.

![how a verification state is derived]({{.Site.PathPrefix}}/static/images/datasheet/verification-states.svg)

`SourceDoc.content_hash` records the revision the corpus holds, `Verification.doc_content_hash` the
one that was checked, and `param.VerificationOfIn` compares them on every read. Staleness is derived,
never stored, and `unknown` is deliberately not folded into `verified`, because a caller that cannot
check must not be told the answer is fine. Each parameter is judged against the document IT cites,
since a spec routinely carries a datasheet and an application note and revising one must not
invalidate the other. The hash lives on `SourceDoc` rather than being resolved from `locator` on
demand, because a check must not do I/O to learn whether its evidence is current (C22).

<details>
<summary>Why the printed revision is recorded too, and why it is display-only</summary>

A hash decides staleness correctly and tells a person nothing. `Verification.doc_revision` is the
document's identity AS PRINTED when it was verified, snapshotted from that day's `SourceDoc.title`.
It turns a flag into a task:

> Re-confirm VDD abs-max: verified against SCES650K, corpus now holds SCES650L, page 4.

It sits on `Verification` rather than `SourceDoc` for a reason easy to get backwards. A re-seed
REWRITES `SourceDoc`, hash and title both, and that rewrite is the event that makes a verification
stale, so a revision recorded there would be destroyed by the one thing that makes it worth having.
`param.MarkVerified` takes the `SourceDoc` itself rather than a hash and title separately, so the two
are read from one place and cannot disagree. Splitting them would let a caller pin one revision's
hash beside another's printed name, which goes stale correctly and then names the wrong document to
the person asked to re-confirm.

**The snapshot must never become a comparison input.** Vendors reissue documents without moving the
printed revision, and move the revision without changing content, so two files stamped "Rev K" may
differ and two differing strings may describe identical bytes. It is not orderable either (K/L/M, but
also 1.0/1.1, A/B, bare dates, "Rev K.1"), so "how many revisions behind" has no general answer.

</details>

<details>
<summary>Who writes a verification, and why hand transcription counts as one</summary>

Hand transcription in the workbench. Typing a value off the page IS a confirmation, and the layer had
always said so implicitly by stamping `confidence: 1.0` on a hand-entered row. What it could not say
was WHICH revision, so the claim never expired.

This is the opposite posture from `candidate.Accept`, which refuses to mark a machine proposal
verified, keeping "a machine proposed this" from reading as "a person checked this". Hand
transcription is on the checked side of that seam.

Two consequences. A transcription against a document whose revision the corpus has not recorded saves
UNVERIFIED rather than being refused, because transcribing is still worth doing and claiming a
confirmation nothing can invalidate is not. And `SourceDoc.title` is editable in the workbench,
because producers fill it with a PART number ("LM1117") rather than the document number the field is
specified to carry, and a snapshot of a part number reads identically before and after a reissue.

One workbench behaviour matters here and the rest belongs to [Web app](../web-app/): browsing a
corpus WRITES a `.partspec.json` beside every document you open, because a document with no saved
spec gets one seeded and persisted. Point it at a scratch copy rather than an original.

</details>

### Comparison semantics

Values, units, and symbols are STORED as printed, so the layer meets vendor variety and a seeded row
can be checked against its page by eye. Three rules keep comparison honest, all one posture: when a
comparison cannot be made safely, stay silent rather than improvise.

**Three trust states, not two.** `param.UnderSpecified` says a row's conditions are not trustworthy
at all, so skip it. A row can be fully captured and still carry a condition that exists only as text,
which is honest data a human can evaluate next to its provenance and no machine can.
`param.MachineComparable` names the boundary, and the middle state is surfaced but never
auto-compared.

**A prefixed unit is reduced to its SI base before comparison, in exactly one place.**
`param.InBaseUnit` converts, an unrecognized unit is skipped rather than scaled by a guess, and no
rule contains a scale factor. The lookup is a closed vocabulary flattened at init rather than a
prefix parsed off the front, so every accepted spelling is a key a test can enumerate. It is
case-sensitive with no fallback, because `mΩ` and `MΩ` differ by nine orders of magnitude.

**`core/classify`'s prefix table is deliberately NOT reused.** It parses a component's value text off
a design, where IEC 60062's RKM code reads `M` as MEGA and case is not significant. Correct for a
schematic value field, and it inverts three orders of magnitude on a printed unit symbol.
`TestUnitVocabulariesAgree` holds the two tables to each other without an import, since the datasheet
tier depends on nothing in `core` (C17). Vendor symbols never appear in rule text either: the same
parameter prints as "VDC", "WV", or "Rated Voltage" depending on vendor, so `symbol` is the
per-vendor match key and the lookup lives behind the join as a per-corpus alias map.

<details>
<summary>The refusal that unit conversion replaced, and the five rule families it was silently breaking</summary>

Conversion used to be a refusal. Unlike unit strings were treated as under-specified and skipped, on
the reasoning that conversion at a call site would become a second informal normalization layer. The
refusal shipped a worse failure than the one it avoided.

An extractor that dropped the row left its rule with an empty list, and a rule that compares nothing
reports nothing, which the runner scores as a **pass**. Neither guard caught it: `check.Available`
saw a params tier attached and the `needs-data` gate saw the symbol seeded. Milliamps are the
ordinary spelling for a sub-amp regulator, so a spec transcribed as printed hit this without doing
anything unusual, and five rule families were silently passing designs with genuine defects.

What made conversion safe was location rather than caution, which is why one table lives in
`datasheet/param` beside `UnderSpecified` and `MachineComparable` and every extractor reads through
it. Storage is unchanged: the spec keeps the printed row and the extractor returns a converted copy,
mirroring `ir.Quantity`'s split on the design side.

</details>

### Pin binding

A parameter can name the pins it applies to. Without that, a part printing the same concept on
several terminals with different limits collapses into one, and the commonest review question about a
power or interface net ("does this connection meet what this pin actually requires") has no answer.
`PartSpec` carries `pins` and `packages`, and `Parameter` carries `pin_refs`.

**Inside the spec the binding key is a spec-local `Pin.id`**, neither the name nor a number, because
an opaque local id is unique by construction and lets `param.Validate` reject a parameter bound to a
pin the spec never declared. A dangling binding is worth catching at load precisely because
downstream it does not look like an error: the parameter stops applying to anything and the rule that
wanted it reports nothing.

`pin_refs` is orthogonal to `applies_to` rather than a second spelling of it. `applies_to` narrows
which *variant* a row covers and `pin_refs` narrows which *terminal*, so a row carrying both is the
conjunction, and an empty `pin_refs` keeps its old meaning of a fact about the part as a whole.

A pin is a property of the **part type**, never of a placement. One `PartSpec` describes one MPN and
a design may place fifty instances, so no {{ explainable "reference-designator" }} appears anywhere
in this contract, and fanning a type-level pin fact out across instances belongs to the rule.

![pin resolution precedence]({{.Site.PathPrefix}}/static/images/datasheet/pin-precedence.svg)

Resolution leads with the pin NAME and uses the number only to break ties. **A pin number is a fact
about a package, and a datasheet parameter is a fact about the die**, so the same silicon in a
different body renumbers its terminals. A number-keyed join on a part seeded from one package and
placed in another reports about the wrong terminal, evaluates cleanly, and looks healthy doing it.
The seeded TXB0104 carries the real collision. Pin data is optional throughout, and a spec carrying
none behaves exactly as it did before pin binding existed (C9).

<details>
<summary>What the number is still good for, and the three ways resolution refuses</summary>

The design side reaches a terminal as `Connection{component_ref, pin_ref}` to the section's
`PartType` to an `ir.Pin`, which carries a `name` and a package-relative `designator`. The datasheet
side offers the same two channels.

A name comes off the same pin function table on both sides, so it survives repackaging. Its weakness
is that it is not unique, and the number repairs that. Where no package is identified,
`param.ResolvePin` still uses the number if every declared package agrees (`VCCA` is number 1 in both
the TSSOP and the UQFN) and refuses only where packages genuinely disagree.

Every refusal is a distinct sentinel, so a caller can tell "no pin data at all" (`ErrNoPinData`, the
older-corpus case, fall back to the part-level path) from "ambiguous" (`ErrPinAmbiguous`) from "the
channels contradict" (`ErrPinConflict`).

</details>

### Constraints between two pins

Some requirements hold between two terminals: a supply that must stay at or below another, a
reference that must sit a volt above its companion. A `Parameter` cannot say this, because
`pin_refs` already means "this row applies to these terminals" and a relation there would make it
mean two things depending on the row. `PinRelation` is the separate shape, held on `PartSpec` rather
than on a `Pin`, because a relation is between two terminals and owned by neither.

**The bound is a value, not a comparison operator.** The obvious shape for "VCC(A) <= VCC(B)" is a
subject, an operator and a reference, and it breaks on the next document: read across four vendors,
three of five instances carry a non-zero allowance. So the bound is on the DIFFERENCE, subject minus
reference, reusing `RangeValue` unchanged:

| The document says | `difference` |
|---|---|
| `VCCA <= VCCB` | `max: 0` |
| never exceeds by more than 0.5 V | `max: 0.5` |
| at least 1 V higher | `min: 1` |
| within 0.3 V of | `min: -0.3, max: 0.3` |

Because the bound is signed, the ORDER of the two ends is load-bearing and swapping them inverts the
requirement, so `param.PinRelations` returning relations from either end means a caller must read
`subject_pin_ref` rather than assume the pin it asked about is the subject.

<details>
<summary>Modality and regime, the two fields that stop a bound being misread</summary>

**Modality** is recorded because "shall never exceed" and "should be at least 1 V higher for best
operation" are different claims, and a consumer holding only the numbers cannot tell a defect from a
suboptimality. The vendor's modal verb is the only evidence, and the printed sentence is kept in
`raw` regardless.

**A regime is a `Condition`**, not a new field. Two of the five instances scope their bound, one to
transient behaviour and explicitly not DC, another across power-up, power-down and normal operation.
Those are test conditions in the sense `Condition` already models, down to its `raw` escape hatch. A
bound recorded without its regime is wrong in both directions, over-applying a transient allowance to
steady state and under-applying a limit the vendor extended across power-up.

</details>

### What is deliberately absent

A field earns its place when a second producer would populate it, and for parameters a second
producer is a second vendor's datasheet.

| Absent | Why |
|---|---|
| Canonical parameter ids and a symbol ontology | `canonical_id` exists and stays empty, while `symbol` and `unit` are as printed, so a row stays checkable against its page. Doing the ontology early bakes one vendor's vocabulary in as canonical, and comparison needs none of it |
| Curve and graph data | Derating and SOA curves are real, and their shape should be designed against real extractor output rather than guessed |
| A verification WORKFLOW | Assignment, review queues and approval gates belong to the pipeline. Only the OUTCOME is in the contract, because a fact carries what a reader needs to trust it and nothing about how the checking was organised |
| Package GEOMETRY | `Package` carries an id, the printed name, and the orderable-MPN suffix, all that pin numbering needs. Land patterns join through the design IR's footprint tier, and duplicating them here would create a second source of truth |
| More than one `PinRelationKind` | Only `TRACKING` had more than one producer across four vendors. An unpopulated enum member reads as a form that never occurs, a different and worse claim than one nobody has evidence for |
| Power sequencing | Datasheets state it plainly, and a netlist carries no evidence of order in time, so no rule could evaluate one. The prose survives on `Pin.description` |

### Why proto, and the worked examples

A cross-runtime contract shared by the Go engine, the TypeScript viewer, and future extractors, where
hand-written parallel types are exactly the drift a shared schema prevents. It is `agni.v1.param`
rather than a corner of `ir` because it has different producers, consumers, and lifecycle. Three
fixtures are transcribed by hand from the cited revision, and `param_test.go` and `pins_test.go`
assert all three validate, so the examples are executable rather than prose.

| Fixture | What it demonstrates |
|---|---|
| `lm1117.textproto` (TI LM1117 LDO, SNOS412Q) | The three limit kinds on one part: abs-max VIN 20 V, recommended-operating VIN 15 V, dropout as a conditional characteristic. Its dropout rows show why differing condition sets stay distinct parameters |
| `bss138.textproto` (BSS138 N-FET, rev C(W)) | The canonical conditional parameter. RDS(on) specified three times, a table-header default encoded as an explicit condition rather than dropped, and the pulse-test footnote retained in `attributes` |
| `txb0104.textproto` (TI TXB0104, SCES650K) | Pin binding, where one part covers every case it must survive. Two supply terminals with genuinely different ranges, one die in three bodies where number 11 is a data I/O in one and a supply in another, a row bound to a group of terminals, a name printed on two pins (`NC`), pins absent from one package, and ball designators, so a pin number is a string rather than an integer |

## The document contract

The doc-IR is the intermediate artifact, a source document decomposed into pages, tables, figures and
text blocks, with cell structure and bounding-box provenance. Many parsers produce it. Recipes, LLM
proposal stages, the verification UI and revision diffing consume it, and none of them touch the
source bytes. It is named for documents rather than tables or datasheets on purpose: figures are
provenance targets and the page text layer feeds search, so "table-IR" would mislead, and app notes,
errata and reference manuals decompose identically, so "datasheet-IR" would overfit today's corpus.

| Property | Guarantee |
|---|---|
| **Documents are keyed by the hash of their source bytes** | A revision is a different Document, and nothing is mutated. Content-addressing, the same idea as a git blob |
| **Region ids** (`p2.t1`) | Deterministic within one derivation, and the address for crops and queue items. NOT stable across producer versions, because a detection change renumbers them |
| **Table content hashes** (`doc.TableHash`) | The cross-version identity, covering grid shape and cell position, span, text and footnotes, excluding bboxes, ids, confidence and header flags. Two derivations of the same printed table hash equal even when detection nudges coordinates. The prototype producer replicates the hash byte-for-byte in Python and the Go validator is the referee |
| **Coordinates** | Page-local, top-left origin, y-down, in PDF points. Producers reading PDF-native boxes flip at emit time |

Querying comes in two tiers. **Tier 1** is `datasheet/doc/`, in-process and deterministic, used by
recipes, tests and the revision differ. `TablesMatching(d, regexp)` selects by title pattern rather
than by id, since ids are not version-stable, and it is the recipe primitive, alongside addressed
access (`TableByID`), grid access (`CellAt`, where a merged cell appears once at its top-left), the
full-text source (`PageText`), and `FindTableForProv` to resolve a locator back to a table. **Tier 2**
is deferred to the extraction store, so "not extracted yet" is searchable rather than a dead end.

Running `tools/pdf2doc` (docling 2.x) over two real datasheets taught three things. Structure is
solid, with every table validating for grid consistency and hash match, including a 32x8 table. Table
titles come back empty, because datasheet tables are headed rather than captioned, so title
attachment is recipe-layer work. And symbol text needs normalization, since subscripts arrive
space-split ("V GSS"), which is also a recipe concern because doc-IR stores text as extracted.

Absent by design: no curve data (figures carry caption and bbox so provenance can point at them), no
semantic classification (the recipe layer's output, which keeps doc-IR reusable across recipe
versions), and no cross-document corpus structure.

## How a PartSpec is derived from a document

    PartSpec = f(document, toolchain, recipes, patches)

Every input is pinned, every output reproducible from a run manifest, re-runs incremental. A run has
two outputs, and the second one is the point:

```mermaid
flowchart LR
    R["a derive run"] --> P["PartSpec<br/>what it extracted"]
    R --> G["gap list<br/>what it declined, and why"]
    P --> C["a rule compares<br/>a design against these"]
    G --> W["a worklist for a human"]
```

| Stage | What it does |
|---|---|
| **1. Classification by candidate titles** | Real producers emit datasheet tables untitled, and often fold the section band into the table as a merged header cell. Classification tries candidates in order: the producer-attached title, then embedded band cells, then heading-like blocks above the table (nearest first, with a small overlap tolerance because real detected boxes touch). The first candidate a recipe rule matches becomes the title and limit kind. Unmatched tables land in the gap list rather than being silently skipped |
| **2. Header-row detection and tokenization** | The column-header row is found by recognized names (Symbol, Parameter, Test Conditions, Min, Typ, Max, Units, Ratings), scanning past band rows. TI-shaped tables with an unlabeled row-label column fall back to column 0, leaving symbol empty rather than guessing. Value cells tokenize by shape, conditions parse two structured forms, and everything else is kept raw-only, which the comparison semantics above then correctly exclude |
| **3. Pin function tables** | A separate path yielding terminals rather than values, and the one stage needing the recipe to declare something the document does not say. Below |
| **4. Patches, applied last** | One pinned human correction to one cell of one exact document, keyed by document content hash plus pre-patch table hash, so a new revision or a re-detection stops it matching. Empty text clears a cell and a patch at an empty position inserts, so a producer cell-placement error (the real LM1117 case, where docling put the abs-max 20 under MIN) is corrected by a clear-plus-insert pair. Applying patches last means a verified fix cannot regress |
| **5. Validation and emission** | The emitted PartSpec must pass `param.Validate`, and every parameter carries provenance with `method: "derive/v0"` and confidence 0.9, where only human verification earns 1.0 |

Stage 3's layout comes in two shapes, both common. A flat header has one column labelled `NO.` or
`PIN`. A banded header puts a spanning `PIN` cell above several columns of designators, and those
sub-columns carry no role word, so **the recipe must say what they MEAN, because the document does
not.**

Read off the page, the two possibilities look identical:

| The sub-columns are | What that means |
|---|---|
| package codes (`D, PW` \| `RUT` \| `YZT`) | one die, numbered differently in each body |
| part variants (`ADS1113` \| `ADS1114` \| `ADS1115`) | three parts sharing one pinout |

The evidence does not settle it either. Over a 63-document corpus the banded shape appears 44 times,
of which six are variant columns and one is package columns, so a rule inferring the axis from header
text would be fitted to a single example and would mint packages named after part numbers on the
rest. `PinColumnAxis` is therefore declared, and the safe default extracts the pins while gapping the
columns it declined to read.

<details>
<summary>Four things about real pin-table output, none visible on the printed page</summary>

| What arrives | Why it matters |
|---|---|
| One header cell can name several packages (`D, PW`) | One cell yields a `PinNumber` for each body |
| Designators split on commas only, never whitespace | Producers flatten a footnote marker into the cell, so `8 2` is pin 8 with footnote 2 while `2, 3` is two terminals. Splitting on whitespace invents a pin on every footnoted row |
| The absence marker is an ASCII hyphen, not the printed em-dash | A parser matching the typographic glyph reads every absence as an unparsed cell |
| Subscripts flatten with an injected space (`VCCA` as `V CCA`) | The name is the channel that resolves a design pin to a spec pin, so fragments are rejoined when every one is a short all-caps token. A multi-word label like `Thermal pad` keeps its spaces |

A row carrying several designators per package is **ambiguous by construction**: `GND 2, 5, 7` is one
terminal bonded to three legs, `NC 6, 9` is two terminals sharing a printed name. The table cannot
tell them apart, so the split keys on the one function the document states in words ("No connection.
Not internally connected."), and every such row is gapped whichever way it went. The type column is
otherwise taken at face value, because real tables leave it blank on supply and ground rows and
inferring `POWER_INPUT` from a name like `VCCA` would be this stage inventing a classification the
document declined to make.

</details>

Note what stage 5's gate covers, because it constrains what may be added to it. `param.Validate` is
`Problems` joined, and `Problems` includes the COMPLETENESS half, so a derived spec must be complete
rather than merely well-formed. Any new completeness check therefore fails every `derive` run the
moment it lands, and the run reports it as "a derive bug, not a data gap". That is the right default
and it is also a trap: a thing a first pass genuinely cannot supply must be recorded as a manifest
gap, never as a completeness problem.

<details>
<summary>Document identity, the case that proved the trap real</summary>

A run states the part it derived and the revision it read, and NOT the document's printed identity,
because `SourceDoc.title` is specified as the vendor's document number and a derivation cannot read
that. It used to be filled from the doc-IR's own title, which producers set to the PART number, so
every citation named the part and none named a revision, identically before and after a reissue.

That refusal is now an `unidentified-document` gap carrying the cover page's opening prose, and a
citation with no recorded identity reads `datasheet for LM1117 (revision unrecorded)` rather than
borrowing the part name.

The one wrong state `param.Validate` does report is a title that repeats the MPN, because that is an
assertion rather than an absence. The check is equality, deliberately, and not a guess at what a part
number looks like: the failure being corrected came from a plausible-looking value nobody challenged,
and a heuristic would reject legitimate titles for vendors whose numbering nobody has seen.

</details>

### Trust defaults, the honesty ladder

Rows from tables with **no conditions channel** stay UNSPECIFIED, under-specified until a human
verifies, never UNCONDITIONAL, because a header default this stage cannot prove captured may qualify
every row. Rows with a captured channel are COMPLETE, and raw-only members still make the row
machine-incomparable, which is the intended middle state. Derived confidence is a constant 0.9.

That upgrade is a SECOND signal rather than the record. `param.MarkVerified` raises confidence to 1
alongside writing the `Verification`, so a consumer reading only the older float is not misled. But
confidence cannot expire and a verification can, so anything deciding whether to TRUST a value reads
the verification state. Judging on the float alone rates a verification of a superseded revision as
the most trustworthy data in the system, because the 1.0 stays after the document moves.

### The manifest, and why declining is recorded

Every run emits a `RunManifest`: doc content hash, producer and derive version, part identity,
recipes matched, patches applied, and the gap list (unclassified tables, unparsed rows, raw-kept
conditions, unapplied patches, untyped pins). What the run did not extract is enumerated rather than
implied.

**A narrow classifier plus a recorded decline beats a clever one.** Pin typing is the worked example.
A table routinely leaves its type column dashed on exactly the supply and ground rows and puts the
fact in prose, so the stage reads the description, but only how it OPENS, in a handful of
near-universal phrasings, and refuses everything else.

The temptation is to search the whole sentence for "ground". One real table describes a thermal pad
that "must either be connected to Ground or left electrically open", which is a decision the BOARD
makes, and a greedy matcher would stamp a false ground onto a net every rail rule then quantifies
over. An untyped pin costs nothing, and a wrongly typed one is a confident wrong answer. That trade
is only affordable because declining is recorded, so the narrow path produces a worklist rather than
a silence.

**The gap list is also the diagnostic instrument.** A pin table whose header spanned two rows had
both its type and description columns invisible, so every pin came out named, numbered and untyped, a
result indistinguishable from a datasheet that simply did not say. That is how a silent extractor bug
survives: its output is plausible. It was found in minutes because the manifest said which pins were
declined and why, and the gap detail separates the two causes, since an unknown token is a vocabulary
gap that names its own fix while a missing column is a judgement for a human.

`derive_test.go` asserts that deriving the raw-shaped BSS138 doc-IR reproduces every hand-encoded row.
Any change must keep that agreement or deliberately update the goldens, the same regression discipline
the render golden SVGs use. Deferred: an ensemble or VLM second path with agreement gating, the
verification queue, a persistence store (specs and manifests are files today), realtime on-demand
derivation, curves as data, and where the recipe catalog lives once a second vendor family accumulates.

## How resolution is scheduled

**Decided, not built.** Everything above is contracts and a function over them. What drives
`PartSpec` population, and when, is separate, sits on top, and changes none of them.

docling gives a faithful but semantically flat decomposition, and across vendors that layer has no
reliable global structure, so there is no single pattern to search for. Given that, the obvious plan
is eager extraction, classifying every table in every datasheet before running rules. It has two
defects. Most of the work is wasted, since a datasheet carries hundreds of parameters and a
schematic's use of a part cares about a handful. And eagerly extracted params are unverifiable,
because extracting 300 values with no design in hand gives no signal for which are wrong.

**So resolution starts from the schematic.** The rules being evaluated ask for exactly the parameters
they need, each ask resolves against a cache, and a miss triggers resolution that backfills it. This
mirrors how an engineer uses a datasheet, as a reference consulted with a question rather than a
database memorized in advance.

![the resolution chain]({{.Site.PathPrefix}}/static/images/datasheet/resolution-chain.svg)

The eager/lazy seam sits at doc-IR to PartSpec. **doc-IR is eager**, because decomposing a PDF is a
fixed one-time cost per file that produces no semantic claim. **PartSpec population is lazy.** So
"extract on demand" means classify and transcribe on demand over already-faithful doc-IR, not run
docling on demand, which removes the usual objection to a just-in-time design where a developer
clicks a rule and stalls on a heavy extraction.

The query language and evaluators do not change. What is new is a resolver, a lazy provider behind
the Model's params tier: today that tier is loaded eagerly and a miss means the rule silently skips,
where a miss would instead trigger the chain and answer. Enumerating which params a rule needs is not
a new step either, since when `supply-exceeds-abs-max` reads `abs_max(part, "VIN")` from the Model,
that read IS the query.

Two consequences. `agni query` can answer datasheet questions that today it cannot unless the set was
preloaded, so the resolver is what lets search reach the datasheet at all. And over a corpus the
cache converges toward the eager one in priority order, accreting into a `PartSpec` built from actual
demand, where every parameter has already been used against a real net.

A backend answers one query, `(part, parameter, condition)`, PartSpec-shaped with confidence and
provenance, so the chain composes without any backend knowing about the others and stops at the first
confident answer. **cache** is a prior resolution. **recipe** is per-vendor deterministic
classification, precise and brittle across vendors. **model** is inference, general and fuzzy.
**local few-shot** embeds the region and suggests the nearest labeled prior. **HITL** is a human, and
only a human resolution grants confidence 1.0. The trust predicates already in the parameter-IR are
the gate between auto-use and show-a-human, so a low-confidence or `UnderSpecified` answer is
surfaced as a suggestion rather than committed as a silent fact.

<details>
<summary>The second query shape, recall, and the three flows this produces</summary>

The resolver serves two query shapes over the same doc-IR and the same chain. A **scalar limit**,
`(part, parameter, condition)`, returns a value with its range and limit kind, the shape
`supply-exceeds-abs-max` needs. A **pin table**, `(part)`, returns a pin-function mapping: which pins
carry which interface signals, a bus's CLK/CMD/DAT lines, a memory byte-lane, a
{{ explainable "transceiver" }}'s TXD/RXD. An interface-shape check can then derive its
required-signal list from the host part's datasheet rather than hand-authoring it. A pin table is
already a doc-IR `Table`, so a recipe classifies it exactly as another recipe locates an abs-max
value, and only the result type differs.

Every resolution, automatic or human, is stored keyed by roughly
`(part-family, parameter, vendor, region-shape)`. Nearest-prior over that store yields suggestions,
which is the few-shot idea used as warm start rather than as auto-committed fact: it accelerates
review and seeds the model backends, and auto-commit still requires the chain to clear the trust gate
on its own. Coverage also gets a better grain, since eager extraction reports "we extracted 300 of an
unknowable total" while demand-driven reports "of the parameters your rules queried, N answered
automatically, M needed a human, K unresolved".

- **Check-driven, no human.** `agni check --params` reads `abs_max(LM1117, VIN)`, misses, the TI
  recipe locates the abs-max table and returns 20 V at confidence 0.9, the value materializes, and
  the rule compares the design's +24 V supply against it, fires, and cites both sides.
- **Escalation at the point of demand.** The same walk, but the recipe cannot confidently locate or
  parse the value. The resolver opens the workbench focused on the likely page and region, "confirm
  VIN abs-max for LM1117," with a warm start from recall. Confirming caches the value with
  region-cited provenance and upgrades trust, and the rule then fires.
- **Search-driven.** A developer clicks a pin or runs a datalog search across parts. The same
  resolver runs underneath, so search and checks share one cache and one path.

Four points stay open: the resolver interface signature and where it lives, async UX for the slow
backends, the recall store's key and similarity metric, and whether multi-param whole-part analyses
batch their queries while staying demand-driven. The home for a pin-function mapping is **settled**,
extending the parameter-IR as [pin binding](#pin-binding) describes, so what remains open there is the
extraction rather than the target.

</details>

## How it joins into checks

The join key is part identity, `PartSpec.mpn` plus `PartSpec.manufacturer`, matching `ir.BomLine`.
The dependency points one way: readers and the design IR never import the parameter layer. When a
design carries no BOM or MPN data the join has no key and parameter checks skip, the same
skip-not-false-pass behaviour used for unseeded parts. The Model's params tier
(`check.NewModelWithParams`) is the join, taking the BomLine MPN first, else the component's MPN
attribute, matched case-insensitively and nothing fuzzier.

That join is by part identity only. The finer per-pin join is a property of the contract today and no
rule consumes it yet, since the shipped rules reach a terminal through a vendor-symbol alias table
and so cannot tell two supply pins of one part apart. Pin-level relations and a pin-rating rule are
separate work, both expected to keep the alias path as the fallback.

Two rules use the layer today. **`supply-exceeds-abs-max`** compares a power-input pin's rail nominal
against the spec's machine-comparable abs-max supply rows, carrying the design site in `Prov` and the
datasheet citation in the message. It is already a right-to-left query, so the scheduling model above
generalizes an existing shape rather than introducing a new one. **`cap-voltage`** is the first
spec-authored datasheet rule with no Go twin: its body is a `check.Spec`, the join and the float
compare live behind the `cap_voltage_detail` SpecFunc, and the FFI's declared reads flow into derived
metadata, so `param.cap_rated_voltage`, `net.max_voltage` and `component.mpn` appear as named
relations without hand-maintained lists.

A citation also carries the value's verification state, derived at citation time from the revision its
`SourceDoc` records. That is what lets the review layer tell a fail backed by a confirmed value apart
from one backed by a confirmation of a superseded revision: `isUnratified` treats `stale` and
`unknown` as untrustworthy, so the item reads Provisional (a re-confirm task) rather than a hard Fail.
Deriving it at citation time rather than stamping it on the finding means a re-seed changes every
subsequent answer with nothing to re-stamp.

A user-facing walkthrough is in [the datasheets guide](../../guide/datasheets/), and the map from
software concepts to the hardware nouns used here is in
[the analogy reference](../../reference/analogy/).

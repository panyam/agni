---
title: "Datasheet layer"
description: "How a part's datasheet becomes data a checker can compare a design against: the parameter and document contracts, the derivation that fills them, the demand-driven scheduling model, and the join into checks."
---

A datasheet in software terms is close to a type definition that also carries runtime
limits. It states what a part is (an LDO, an N-channel FET) and the envelope inside which the
part behaves as specified. The datasheet layer turns that document into structured data so a
rule can compare a design against a part's real limits. It is built as three proto contracts
with two processes over them:

- The **parameter-IR** models one extracted parameter with its conditions, its limit kind,
  and its provenance.
- The **doc-IR** models a source PDF decomposed into pages, tables, figures, and text blocks.
- **Derivation** is the deterministic function from a document to a parameter set.
- **Resolution** is the scheduling model that decides which parameters to extract and when.
- A **join by part identity** brings the result into the rule engine.

Each contract follows the pattern used elsewhere in the engine: one schema with many
producers. The design IR is one representation with many format readers (see
[Ingestion and IR](../ingestion-and-ir/)). The parameter-IR is one representation with many
datasheet extractors. No extractor populates the parameter schema in production yet, and it
is validated today by hand-encoded fixtures transcribed from real datasheets.

## The parameter contract

A parameter is not a scalar. "RDS(on) = 3.5 Ω" is not a fact about a part. "RDS(on) max
3.5 Ω at VGS = 10 V, ID = 0.22 A, TJ = 25 °C, pulse-tested" is. A validation engine that
compares design state against datasheet limits is only safe when the schema makes it
impossible to hold the first form without noticing. Three schema features enforce that.

1. **`RangeValue` has no bare-scalar form.** Every value is a min/typ/max triple with
   explicit presence (proto3 `optional`), so "max only" (an absolute-max table row), "typ
   only" (an uncharacterized typical), and "min/max" (an ensured band) stay distinct and none
   collapses into a lone number.
2. **`LimitKind` is a first-class enum** rather than free text: `ABSOLUTE_MAX` (a stress
   rating), `RECOMMENDED_OPERATING` (the vendor's functional envelope), and `CHARACTERISTIC`
   (measured behavior under stated test conditions). A rule that checks a net voltage against
   a limit dispatches on this field, because the three kinds have different meanings and
   different safe uses. `UNSPECIFIED` fails validation, so extractors classify or drop.
3. **`ConditionCoverage` makes under-specification explicit.** A parameter whose condition
   list is not asserted complete (`COMPLETE`) or genuinely condition-free (`UNCONDITIONAL`) is
   under-specified. `param.UnderSpecified` returns true and consumers skip or flag it rather
   than compare against it. A conditions-stripped value produces confident-but-wrong findings,
   which is worse than no value at all.

Conditions themselves (`Condition`) capture an exact point (`eq`), a range (`min`/`max`), or
a one-sided bound, always with the source text in `raw`, so an unstructured condition
("VDS = VGS", a temperature-range phrase) is retained verbatim rather than dropped.

### Provenance

Every `Parameter` carries a `ParamProvenance`: the source document (by reference to a
`SourceDoc` that declares the vendor doc number and revision), the page, the table or figure
as titled, the extraction method, and a confidence in the range (0, 1]. This mirrors the
discipline the check layer enforces on findings, where every finding carries a `Prov`. An
extracted value that an engineer cannot verify against the exact datasheet page is not usable
for review, so zero confidence is invalid by construction and a value nothing stands behind is
never emitted. Hand-encoded values use `method: "hand"`, `confidence: 1`.

`SourceDoc.locator` is corpus-local by design. The deployment posture keeps the customer's
datasheets and the extracted database inside their own boundary, so locators are only
meaningful within one deployment and the schema carries no assumption of a shared global
document store.

### Verification, and why it expires

Provenance says how a value was PRODUCED. It cannot say whether anyone has since agreed with it, and
an extractor can never write that claim on its own behalf. `Parameter.verification` is the separate
record: who checked the value, when, and against WHICH REVISION.

The revision is the whole reason this is a message rather than a boolean. A value checked against
rev K is not thereby true of rev L, and a verification that survives a revision is worse than no
verification at all. An unverified value is honest about what it is. A stale-verified one is a
confident wrong answer with a person's name attached, and it is trusted precisely because someone
did once check it.

So staleness is DERIVED, never stored. `SourceDoc.content_hash` records the revision the corpus
currently holds, `Verification.doc_content_hash` records the one that was checked, and
`param.VerificationOfIn` compares them on every read:

| State | Meaning |
|---|---|
| `unverified` | Nobody has confirmed this value. The ordinary state of anything an extractor produced. |
| `verified` | A person confirmed it against the revision the corpus holds. |
| `stale` | A person confirmed it against a DIFFERENT revision. Needs re-confirming, which is much smaller than finding it again. |
| `unknown` | A verification exists but no current revision is recorded, so drift cannot be ruled out. |

`unknown` is deliberately not folded into `verified`. A caller that cannot check must not be told the
answer is fine, which is the same discipline the outcome vocabulary applies to a check that could not
run.

This is the mechanism `derive.Patch` already uses one layer over. A patch is keyed by content hash,
so a revision invalidates it by construction: it stops matching and the manifest reports it
unapplied. Nothing decays silently because the key stops resolving. Facts now behave the same way,
and the invalidation happens at the moment a re-seed rewrites `SourceDoc.content_hash` rather than
when someone remembers to go looking.

Two consequences worth knowing. Each parameter is judged against the document IT cites, not against
one revision for the whole spec, because a spec routinely carries a datasheet and an application note
and revising one must not invalidate values read from the other. And the hash lives on `SourceDoc`
rather than being resolved from `locator` on demand, because a check must not do I/O to learn whether
its evidence is current (C22) and the engine runs in hosts with no filesystem at all.

### Comparison semantics

Values, units, and symbols are STORED as printed, so the comparison layer meets vendor
variety and a seeded row can be checked against its datasheet page by eye. Three rules keep
comparison honest, all variants of one posture: when a comparison cannot be made safely, stay
silent rather than improvise.

- **Three trust states, not two.** `param.UnderSpecified` says a row's conditions are not
  trustworthy at all, so skip it. A row can be fully captured and still carry a condition that
  exists only as text ("VDS = VGS", a temperature-range phrase). That is honest data a human
  can evaluate next to its provenance and no machine can. `param.MachineComparable` names that
  boundary: only rows whose every condition is structured (`eq` or `min`/`max`) may enter an
  automatic comparison. The middle state, captured but text-only, is surfaced and never
  auto-compared.
- **A prefixed unit is reduced to its SI base before comparison, in exactly one place.** A row
  printed in millivolts, milliamps or milliohms is converted by `param.InBaseUnit` and compared;
  a unit that table does not recognize is skipped rather than scaled by a guess.

  This used to be a refusal: unlike unit strings were treated as under-specified and skipped,
  because conversion logic written at a call site would be a second, informal normalization
  layer. The refusal was the wrong conclusion from a right premise, and it shipped a worse
  failure than the one it avoided. An extractor that dropped the row left its rule with an empty
  list, and a rule that compares nothing reports nothing, which the runner scores as a **pass**.
  Neither guard caught it: `check.Available` saw a params tier attached, and the `needs-data`
  gate saw the symbol seeded. Milliamps are the ordinary spelling for a sub-amp regulator, so a
  spec transcribed as printed hit this without doing anything unusual, and five rule families
  were silently passing designs with genuine defects.

  What made conversion safe was location, not caution. One table lives in `datasheet/param`
  beside `UnderSpecified` and `MachineComparable`, every extractor reads through it, and no rule
  contains a scale factor. The lookup is a closed vocabulary flattened at init rather than a
  prefix parsed off the front, so every accepted spelling exists as a key that a test can
  enumerate. It is case-sensitive with no fallback, because `mΩ` and `MΩ` differ by nine orders
  of magnitude and a case-insensitive retry would resolve that by guessing.

  Storage is unchanged, and that is the point: the spec keeps the printed row, the extractor
  returns a converted copy. This mirrors `ir.Quantity`'s split on the design side, where `value`
  is normalized at ingestion and `input` keeps the source text so the normalization stays
  non-lossy. `RangeValue` has no `input` field, which is why the parameter layer converts at
  read time rather than at seed time.

- **`core/classify`'s prefix table is deliberately NOT reused for this.** It parses a
  component's value text off a design, where IEC 60062's RKM code reads `M` as MEGA and case is
  not significant. That is correct for a schematic value field and inverts three orders of
  magnitude on a printed unit symbol. The two tables agree on the canonical base spellings, and
  `TestUnitVocabulariesAgree` in `core/check` holds them to each other without an import, since
  the datasheet tier depends on nothing in `core` (C17).
- **Vendor symbols never appear in rule text.** The same physical parameter prints as "VDC",
  "WV", or "Rated Voltage" depending on vendor. `symbol` is the per-vendor match key, but the
  lookup lives behind the join and Model layer as a per-corpus alias map, so a rule asks for a
  concept and no rule hardcodes one vendor's spelling.

### Pin binding

A parameter can name the pins it applies to. Without that, a part printing the same concept on
several terminals with different limits collapses into one concept, and the most common review
question about a power or interface net ("does this connection meet what this pin actually
requires") has no answer. `PartSpec` therefore carries `pins` and `packages`, and `Parameter`
carries `pin_refs`.

**Inside the spec the binding key is a spec-local `Pin.id`, which is neither the name nor a
number.** Both of those are ambiguous in ways the spec itself can resolve, and an opaque local id
is unique by construction, which is what lets `param.Validate` reject a parameter bound to a pin
the spec never declared. A dangling binding is worth catching at load precisely because
downstream it does not look like an error: the parameter simply stops applying to anything, and
the rule that wanted it reports nothing.

`pin_refs` is orthogonal to `applies_to`, not a second spelling of it. `applies_to` narrows which
*variant* a row covers; `pin_refs` narrows which *terminal*. A row carrying both is the
conjunction. An empty `pin_refs` means the row is a fact about the part as a whole (a junction
temperature, a storage range), which is also what every spec seeded before pin binding says, so
empty keeps meaning exactly what it always meant.

A pin is a property of the **part type**, never of a placement. One `PartSpec` describes one MPN
and a design may place fifty instances of it, so no reference designator appears anywhere in this
contract. A rule fans a type-level pin fact out across instances, each landing on its own net; the
fan-out belongs to the rule.

#### Why the name leads and the number only breaks ties

The design side reaches a terminal as `Connection{component_ref, pin_ref}` to the section's
`PartType` to an `ir.Pin`, which carries both a `name` and a package-relative `designator`. The
datasheet side offers the same two channels. The precedence is name first, number as a tie-breaker
inside an identified package, refusal when the two disagree.

Leading with the number is the tempting choice and the wrong one. **A pin number is a fact about a
package; a datasheet parameter is a fact about the die.** The same silicon wired into a different
body renumbers its terminals, so a number-keyed join on a part seeded from one package and placed
in another reports about the wrong terminal, evaluates cleanly, and looks healthy doing it. That
is worse than reporting nothing. The seeded TXB0104 carries the real collision: number 11 is the
data I/O `B3` in the TSSOP-14 and the `VCCB` supply in the UQFN-12.

A name comes off the same pin function table on both sides, so it survives repackaging. Its
weakness is that it is not unique, and that is exactly what the number repairs: a name printed on
several terminals is disambiguated by the designator within a package the design is known to
place. Where no package is identified, `param.ResolvePin` still uses the number if every declared
package agrees on it (`VCCA` is number 1 in both the TSSOP and the UQFN, so the body cannot change
that answer) and refuses only where the packages genuinely disagree. Every refusal is a distinct
sentinel, so a caller can tell "this spec has no pin data at all" (`ErrNoPinData`, the older-corpus
case, fall back to the part-level path) from "the evidence is ambiguous" (`ErrPinAmbiguous`) and
from "the two channels contradict each other" (`ErrPinConflict`).

Pin data is optional throughout. A spec that carries none validates and behaves exactly as it did
before pin binding existed, so an older corpus is unaffected (C9).

### Constraints between two pins

Some requirements hold between two terminals rather than about one: a supply that must stay at or
below another, a reference that must sit at least a volt above its companion. A `Parameter` cannot
say this. It states a value about one quantity, and its `pin_refs` already means "this row applies
to these terminals", so expressing a relation there would make `pin_refs` mean two different things
depending on the row. `PinRelation` is the separate shape, held on `PartSpec` rather than on a
`Pin`, because a relation is between two terminals and is owned by neither.

**The bound is a value, not a comparison operator.** The obvious shape for "VCC(A) must be less
than or equal to VCC(B)" is a subject, an operator and a reference, and it breaks on the next
document. Read across four vendors, three of five instances carry a non-zero allowance: one part
requires its analog supply never exceed another by more than 0.5 V, another wants its high-side
reference at least 1 V above the low, a third permits 100 mV of difference but only as a transient.
So the bound is on the DIFFERENCE, subject minus reference, and reuses `RangeValue` unchanged:

| The document says | `difference` |
|---|---|
| `VCCA <= VCCB` | `max: 0` |
| never exceeds by more than 0.5 V | `max: 0.5` |
| at least 1 V higher | `min: 1` |
| within 0.3 V of | `min: -0.3, max: 0.3` |

Because the bound is signed, the ORDER of the two ends is load-bearing and swapping them inverts
the requirement. `param.PinRelations` returns the relations a pin takes part in from either end, so
a caller has to read `subject_pin_ref` rather than assume the pin it asked about is the subject.

**Modality is recorded because it changes what a violation means.** "Shall never exceed" and
"should be at least 1 V higher for best operation" are different claims, and a consumer holding only
the numbers cannot tell a defect from a suboptimality. The vendor's own modal verb is the only
evidence of which one it is, and the printed sentence is kept in `raw` regardless.

**A regime is a `Condition`, not a new field.** Two of the five instances scope their bound: one to
transient behaviour and explicitly not to DC, another across power-up, power-down and normal
operation. Those are test conditions in the sense `Condition` already models, down to its `raw`
escape hatch, so a relation carries `conditions` and nothing new was invented for it. A bound
recorded without its regime is wrong in both directions: it over-applies a transient allowance to
steady state, and under-applies a limit the vendor extended across power-up.

Relations are optional on the same terms as pins. A spec carrying none validates exactly as before.

### What is deliberately absent

A field earns its place only when a second producer would populate it, and for parameters a
"second producer" is a second vendor's datasheet. Several things are left out on that basis.

- **No canonical parameter ids, no canonical symbol ontology.** `Parameter.canonical_id` exists
  but stays empty, and `symbol` is as printed. The ontology is a later phase, and doing it early
  would bake one vendor's vocabulary in as "canonical". `unit` is also stored as printed, and the
  fixtures keep "IOUT = 800 mA" as 800 mA rather than 0.8 A, so a row stays checkable against its
  datasheet page. That is a STORAGE decision only: comparison reduces a prefixed unit to its SI
  base (see comparison semantics above), which needs no ontology because the SI prefixes are
  specified and vendor-independent in a way parameter NAMES are not.
- **No graph or curve data.** Derating and SOA curves are real and valuable, but they are the
  harder extraction and their shape (sampled curves, fitted models) should be designed against
  real extractor output rather than guessed.
- **No verification WORKFLOW** (assignment, review queues, approval gates). That belongs to the
  extraction pipeline and store, and only the workflow's OUTCOME is in the contract: `Verification`
  records that a person stood behind a value against a stated revision (see "Verification, and why it
  expires" above). Who was asked to check it, what state their review is in, and who may approve are
  all workflow, and stay out. The line is that a fact carries what a reader needs in order to decide
  whether to trust it, and nothing about how the checking was organised.
- **No package GEOMETRY.** `Package` carries an id, the name as printed, and the orderable-MPN
  suffix, which is what pin numbering needs and nothing more.
  Land patterns, body dimensions, and package-compatibility checks join through the design IR's
  footprint tier when they arrive; duplicating that here would create a second source of truth.
- **Only one kind of pin relation.** `PinRelationKind` has a single member, `TRACKING`, because
  that is the only form a read across four vendors found more than one producer for. Two others
  are real and recorded on the issue rather than in the schema: a common-net requirement (found
  only in one vendor's documents so far) and a mutual same-state requirement for unused pin pairs
  (found only in two second-source parts whose wording is nearly identical, which is one document
  lineage rather than two producers). An unpopulated enum member reads to a consumer as a form
  that never occurs, which is a different and worse claim than one nobody has evidence for yet.
- **No power-sequencing shape.** Datasheets state sequencing between pins as plainly as they state
  ordering, but a netlist carries no evidence of order in time, so no rule could ever evaluate one.
  The prose survives verbatim on `Pin.description`, which is where a human reads it.

### Why proto

The parameter-IR is a proto package for the same reason the design IR and geometry sidecar
are. It is a cross-runtime contract shared by the Go engine, the TypeScript viewer surfaces,
and future extractors in whatever language the tooling prefers, and hand-written parallel
types are exactly the drift a shared schema prevents. The human-authoring cost is paid in
textproto, which the fixtures use and which stays diffable and comment-friendly. It is a
separate proto package (`agni.v1.param`) rather than a corner of `ir`, because it has
different producers, different consumers, and a different lifecycle.

### Worked examples

Three fixtures are transcribed by hand from the cited datasheet revision, values and units as
printed. They are FIXTURES rather than corpus entries: each carries the few rows its properties
need, not the part's parameter set. A seeded corpus lives with its source documents, outside this
repo, which is what `SourceDoc.locator`'s corpus-local posture already assumes.

- **`datasheet/param/testdata/lm1117.textproto`** (TI LM1117 LDO, SNOS412Q rev Jan 2023) shows the
  three limit kinds on one part: abs-max VIN 20 V, recommended-operating VIN 15 V, and dropout
  voltage as a conditional characteristic. The dropout rows show why rows with different
  condition sets stay distinct parameters: typ 1.2 V holds at TJ = 25 °C and max 1.3 V holds
  over the 0 to 125 °C junction range, both at IOUT = 800 mA.
- **`datasheet/param/testdata/bss138.textproto`** (BSS138 N-FET, Fairchild rev C(W)) is the canonical
  conditional parameter, RDS(on) specified three times (VGS = 10 V, VGS = 4.5 V, and VGS = 10 V
  at TJ = 125 °C), plus a table-header default ("TA = 25 °C unless otherwise noted") encoded as
  an explicit condition rather than silently dropped, and the pulse-test footnote retained in
  `attributes`.

- **`datasheet/param/testdata/txb0104.textproto`** (TI TXB0104 level translator, SCES650K rev Mar
  2025) is the pin-binding example, and one real part covers both cases the binding has to
  survive. `VCCA` and `VCCB` are two supply terminals with genuinely different ranges (recommended
  1.2 to 3.6 V against 1.65 to 5.5 V), which is the collapse pin binding undoes. The same die
  ships in a TSSOP-14, a UQFN-12 and a DSBGA-12, and the renumbering is not a relabelling: number
  11 is the `B3` data I/O in one body and the `VCCB` supply in another, which is the argument for
  the name-first precedence in one line. It also carries a row bound to a group of terminals (one
  continuous-current limit stated for both supplies and ground at once), a name printed on two pins
  (`NC`), pins present in one package and absent from another, and ball designators, which is why a
  pin number is a string rather than an integer.

`datasheet/param/param_test.go` and `datasheet/param/pins_test.go` assert that all three fixtures
validate and that these encodings are present, so the worked examples are executable rather than
prose.

## The document contract

The doc-IR is the intermediate artifact of the extraction pipeline: a source document
(datasheet PDF, app note) decomposed into pages, tables, figures, and text blocks, with cell
structure and bounding-box provenance. It sits between the raw document bytes and the
parameter-IR. Many document parsers produce doc-IR. Recipes, LLM proposal stages, the human
verification UI, and revision diffing consume doc-IR and never touch the source bytes. One
prototype producer (docling) and a hand-authored fixture exercise it today.

The artifact is named for documents rather than tables or datasheets on purpose. It carries
more than tables, since figures are provenance targets, headings and footnotes are
classification context, and the page text layer feeds search, so "table-IR" would mislead.
Nothing in the decomposition is datasheet-specific either, because app notes, errata, and
reference manuals decompose identically, so "datasheet-IR" would overfit the current corpus.
`doc` joins the contract family alongside `ir` for designs, `geom` for geometry, and `param`
for parameters.

### Identity and stability

These are the properties a consumer may rely on.

- **Documents are keyed by the hash of their source bytes.** A datasheet revision is a
  different Document by construction, and nothing is ever mutated. This is content-addressing,
  the same idea as a git blob.
- **Region ids** (`p2.t1`) are deterministic within one derivation and are the address for
  crops, review-queue items, and in-derivation navigation. They are not stable across producer
  versions, because a detection change renumbers them.
- **Table content hashes** (`doc.TableHash` covers grid shape, cell position/span/text, and
  footnotes, excluding bboxes, ids, confidence, and header flags) are the cross-version
  identity. Two derivations of the same printed table hash equal even when detection nudges
  coordinates, and revision diffing skips unchanged tables on this key. `doc.Validate`
  recomputes and enforces stored hashes, so a validated doc-IR's hashes can be trusted without
  recomputation. The prototype producer replicates the hash byte-for-byte in Python, and the Go
  validator is the referee.
- **Coordinates** are page-local, top-left origin, y-down, in PDF points. Producers reading
  PDF-native (bottom-left, y-up) boxes flip at emit time.

### Querying in two tiers

Tier 1 is the `datasheet/doc/` package, in-process and deterministic, used by recipes, tests, and the
revision differ.

- `TablesMatching(d, regexp)` selects tables by title pattern rather than by id, since ids
  are not version-stable. It is the recipe primitive.
- `TableByID` / `FigureByID` give addressed access within a derivation (crops, queue).
- `CellAt` / `CellText` give grid access, where a merged cell appears once, at its top-left.
- `PageText` joins a page's text blocks and is the full-text-search source.
- `FindTableForProv(d, page, label)` resolves a `param.ParamProvenance` locator (page plus
  table label, matched by title equality or containment either way) to a table. A committed
  cross-contract test proves the param fixture's provenance resolves against the doc-IR
  fixture.

Tier 2 is deferred to the extraction store: a Connect `DocService` over the persisted corpus,
addressing a document by hash and a region by id for verification crops, plus full-text search
over an index built from `PageText` and table cells, so "not extracted yet" is searchable
rather than a dead end. The index engine choice is left open, and the schema's obligation to
that tier is already met through stable addressing, content hashes, and retained text.

### What the real producer taught

Running `tools/pdf2doc` (docling 2.x) over the two datasheets surfaced three findings.

- **Structure is solid.** Every table validated for grid consistency and hash match,
  including a 32x8 electrical-characteristics table, and cell text is faithful.
- **Table titles come back empty.** Datasheet tables are headed rather than captioned, and
  the producer does not attach nearby headings. Title attachment is therefore recipe-layer work
  (the nearest heading text block above the table bbox), which is why `Table.title` resolution
  and the recipe tests run against the hand-authored fixture, the post-recipe shape, rather
  than raw producer output.
- **Symbol text needs normalization.** Subscripts arrive space-split ("V GSS"). That is a
  recipe-layer tokenizer concern, and doc-IR stores text as extracted, faithful to the parse.

### What is deliberately absent

- **No curve or graph data.** Figures carry a caption and bbox so provenance can point at
  them and a human can jump there. Extracting curve data waits for real extractor output to
  design against.
- **No semantic classification** (this-table-is-abs-max). That is the recipe layer's output,
  recorded in the parameter-IR. doc-IR stays a faithful decomposition with no interpretation,
  which is what makes it reusable across recipe versions.
- **No cross-document corpus structure** (part to documents). That is the store's join.

## How a PartSpec is derived from a document

The extraction stage takes a document to a PartSpec as a deterministic function:

    PartSpec = f(document, toolchain, recipes, patches)

Every input is pinned, every output is reproducible from a run manifest, and re-runs are
incremental.

### The stages, all deterministic

1. **Classification by candidate titles.** Real producers emit datasheet tables untitled, and
   often fold the section band into the table as a merged header cell. Classification tries
   candidate titles in order: the producer-attached title, then embedded band cells, then
   heading-like text blocks above the table (nearest first, with a small overlap tolerance
   because real detected boxes touch). The first candidate a recipe rule matches becomes the
   table's title and limit kind. A note line sitting between the heading and the table is
   harmless, because it is a candidate that never matches a rule. Unmatched tables land in the
   manifest's gap list rather than being silently skipped.
2. **Header-row detection and tokenization.** The column-header row is found by recognized
   column names (Symbol, Parameter, Test Conditions, Min, Typ, Max, Units, Ratings), scanning
   past band rows. TI-shaped tables with an unlabeled row-label column fall back to column 0 as
   the name column, leaving symbol empty rather than guessing. Value cells tokenize by shape:
   plain numbers, "±N", and "A to B" ranges (spaced signs included). Conditions parse two
   structured forms ("SYM = N UNIT" and "A <= SYM <= B UNIT"). Everything else is kept
   raw-only, which the parameter contract's comparison semantics then correctly exclude from
   automatic comparison. Non-title band text ("TA = 25C unless otherwise noted") becomes a
   table-level condition on every row.
2b. **Pin function tables.** A table a `PinTableRule` claims yields terminals rather than values,
   so it takes a separate path. Its layout comes in two shapes and both are common: a flat header
   with one column labelled `NO.` or `PIN`, and a banded header where a spanning `PIN` cell sits
   above several columns of designators. In the banded shape the sub-columns carry no role word at
   all, so they are identified by position rather than by vocabulary.

   **The recipe must say what those sub-columns MEAN, because the document does not.** A banded
   table looks identical whether its sub-columns are package codes (`D, PW` | `RUT` | `YZT`) or
   part variants (`ADS1113` | `ADS1114` | `ADS1115`), and the two are opposite: per-package
   numbering of one die versus three different parts sharing a pinout. Measured over a
   63-document corpus the banded shape appears 44 times, of which six are variant columns and one
   is package columns, so a rule inferring the axis from header text would be fitted to a single
   example and would mint packages named after part numbers on the rest. `PinColumnAxis` is
   therefore declared, and the safe default extracts the pins while recording a gap naming the
   columns it declined to read.

   Four things about real producer output, none of which are visible on the printed page:

   - **One header cell can name several packages.** `D, PW` heads a single column of designators
     shared by two bodies, so one cell yields a `PinNumber` for each.
   - **Designators split on commas only, never whitespace.** Producers flatten a footnote marker
     into the cell as a trailing token, so `8 2` is pin 8 carrying footnote 2 while `2, 3` is
     genuinely two terminals. Splitting on whitespace invents a pin on every footnoted row.
   - **The absence marker arrives as an ASCII hyphen**, not the em-dash printed on the page. A
     parser matching the typographic glyph reads every absence as an unparsed cell.
   - **Subscripts flatten with an injected space**, so `VCCA` arrives as `V CCA`. The name is the
     channel that resolves a design pin to a spec pin, so it is rejoined when every fragment is a
     short all-caps token. A multi-word label like `Thermal pad` keeps its spaces.

   A row carrying several designators per package is **ambiguous by construction**: `GND 2, 5, 7`
   is one terminal bonded to three legs, and `NC 6, 9` is two terminals sharing a printed name.
   The table cannot tell them apart, so the split is keyed on the one function the document states
   in words ("No connection. Not internally connected."), and every such row is gapped whichever
   way it went. The type column is otherwise taken at face value: real tables leave it blank on
   supply and ground rows, and inferring `POWER_INPUT` from a name like `VCCA` would be this stage
   inventing a classification the document declined to make.
3. **Patches, applied last.** A patch is one pinned human correction to one cell of one exact
   document, keyed by the document content hash plus the pre-patch table content hash, so a new
   revision or a re-detection invalidates it by construction: it stops matching, and the
   manifest reports it unapplied. Empty text clears a cell, and a patch at an empty position
   inserts, so a producer cell-placement error (the real LM1117 case, where docling put the
   abs-max 20 under MIN) is corrected by a clear-plus-insert pair. Applying patches last means
   a verified fix cannot regress.
4. **Validation and emission.** The emitted PartSpec must pass `param.Validate`. Every
   parameter carries provenance: page, attached table title, `method: "derive/v0"`,
   confidence 0.9, where only a human verification earns 1.0.

### Trust defaults, the honesty ladder

- Rows from tables with **no conditions channel** stay `ConditionCoverage` UNSPECIFIED,
  under-specified until a human verifies, never UNCONDITIONAL, because a header default this
  stage cannot prove captured may qualify every row.
- Rows with a captured conditions channel (column and/or band) are COMPLETE. Raw-only members
  still make the row machine-incomparable, the intended middle state that surfaces to a human
  without auto-comparing.
- Derived confidence is a constant 0.9, below the human ceiling. The verification queue
  upgrades confirmed rows to `method: "human-verified"`, confidence 1, and demotions become
  patches.
- That confidence upgrade is a SECOND signal, not the record itself. `param.MarkVerified` writes the
  `Verification` and raises confidence to 1 alongside it, so a consumer reading only the older float
  is not misled while the explicit state becomes available. But confidence cannot expire and a
  verification can, so anything deciding whether to TRUST a value reads the verification state.
  Judging on the float alone rates a verification of a superseded revision as the most trustworthy
  data in the system, for the reason above: the 1.0 stays after the document moves.

### The manifest, coverage accounting

Every run emits a `RunManifest` recording the doc content hash, the doc producer and derive
version (the toolchain pin), part identity, recipes matched, patches applied, and the gap
list: unclassified tables, unparsed rows, raw-kept conditions, and unapplied patches. What the
run did not extract is enumerated rather than implied. Ensemble-agreement fields exist in the
manifest and stay zero until a second extraction path lands.

### The golden gate

The hand-encoded fixtures in `datasheet/param/testdata/` are the first verified golden corpus.
`datasheet/derive/derive_test.go` asserts that deriving the raw-shaped BSS138 doc-IR reproduces every
hand-encoded row (symbol, kind, min/typ/max, unit). Any change to the derive stage must keep
that agreement or deliberately update the goldens, the same regression discipline the render
golden SVGs use, applied to extraction. `Version` is bumped on behavior changes.

### Deliberately deferred

- **Ensemble or VLM second path** and agreement gating. The manifest carries the stats
  fields, and the deterministic path is meant to soak first.
- **Verification queue** (crop-beside-row UI, confidence upgrades). It needs the persistence
  port, and patches are the storage half of it, already in place.
- **Persistence store.** Specs and manifests are files today, and the injected-port store
  arrives with the service tier.
- **Realtime on-demand derivation** (latency budget, where it runs).
- **Curves and graphs** as data. Figures remain provenance targets.
- **Recipe catalog location.** Recipes are shareable engine content (vendor layout
  knowledge), and whether they ship in-repo as a `recipes/` catalog or as packs is open until
  a second vendor family accumulates.

## How resolution is scheduled

Everything above is a set of contracts and a function over them. What drives `PartSpec`
population, and when, is a separate concern. The model below sits on top of the parameter-IR,
the doc-IR, and derivation, and changes none of them. It changes only how they are driven. It
is a decided design direction that is not yet implemented.

### The problem it solves

docling gives a faithful but semantically flat decomposition: pages, tables, figures, text
blocks, bboxes, raw cell grids. Across a corpus of different vendors and part families that
layer has no reliable global structure. Column orders, header wording, and where a condition
lives (a column, a footnote, or the section header "at Ta=25°C unless noted") all differ.
Searching for one pattern over the raw doc-IR is the wrong approach, because there is no single
pattern to find.

Given that, the obvious plan is eager extraction: classify every table in every datasheet into
a full `PartSpec`, then run rules against the design. That plan has two defects.

1. **Most of the work is wasted.** A datasheet carries hundreds of parameters, and a given
   schematic's use of a part cares about a handful: the supply tied to a rail, abs-max on pins
   that reach a net, a logic threshold where a net lands.
2. **Eagerly extracted params are unverifiable.** Extracting 300 values with no design in
   hand gives no signal for which are wrong, because nothing has been used against a real net.

### The decision: right-to-left, demand-driven

Resolution starts from the schematic rather than the document. For each component, net, and
pin, the rules being evaluated ask for exactly the parameters they need. Each ask is resolved
against a cache, and a miss triggers resolution (recipe, model, or human) that backfills the
cache. This mirrors how a hardware engineer uses a datasheet, as a reference consulted with a
specific question rather than a database memorized in advance.

The eager/lazy seam sits at doc-IR to PartSpec, and the two layers put their cost where it is
justified.

- **doc-IR is eager.** Decomposing a PDF into faithful pages, tables, and figures is a fixed
  one-time cost per file, produces no semantic claim, and is OCR-free for born-digital
  datasheets. Run it once per document, offline.
- **PartSpec population is lazy.** There is no whole-document semantic classification up
  front. A parameter enters the `PartSpec` cache only when a real query asks for it.

So "extract on demand" means classify and transcribe on demand over already-faithful doc-IR,
not run docling on demand. The slow pass is already done, and the on-demand step is a fast
locate-and-confirm. That removes the usual objection to a just-in-time design, where a
developer clicks a rule and stalls on a heavy extraction.

### A lazy provider behind the Model params tier

The query language and the evaluators already exist. `query.Naive` over `check.NewModel(d)` is
the datalog surface, and the Spec/Model rule layer is the rule evaluator. Neither changes.
What is new is one layer beneath them.

- **Enumerating which params a rule needs is not a new step.** It falls out of normal
  evaluation. When `supply-exceeds-abs-max` walks each supply net and reads `abs_max(part,
  "VIN")` from the Model, that read is the query. There is no separate planning phase, because
  the planning is implicit in lazy evaluation.
- **The new piece is a resolver, a lazy provider behind the Model's params tier.** Today the
  params tier is loaded eagerly and wholesale (`param.LoadSet` from a seed dir), and a miss
  means the rule silently skips. In the demand-driven model a miss triggers the resolution
  chain, materializes the result into the `PartSpec` cache, and answers. The loop is cache
  miss, side-effecting resolution, backfill, answer.

Rules and datalog queries see the same relation they see today. It materializes on access
rather than having been preloaded. Two consequences follow. The `agni query` search surface
can answer datasheet questions (`abs_max(C, "VIN", V)`) that today it cannot answer unless the
set was preloaded, so the resolver is what lets search reach the datasheet at all. And this
slots into the existing `check.Available` gating, where an empty params tier is silent by
construction, and into the provider story the codebase already earmarks for external
vocabulary.

Over a corpus the demand-driven cache converges toward the eager one, in priority order. The
first design that uses an LM1117 pays to resolve VIN abs-max, and the fiftieth hits cache. The
cache accretes into a real `PartSpec`, but one built from actual demand: exactly the useful
parameters, each already used against a real net, so each carries a confidence signal. The
`param.PartSpec` contract (parameter, conditions, range, limit kind, provenance, trust
predicates) is unchanged, and only the scheduler flips from eager-batch to demand-driven.

### The resolution chain

A resolver backend answers one query, `(part, parameter, condition)`, with a PartSpec-shaped
result plus confidence and provenance. The chain runs in escalation order and stops at the
first confident answer:

    cache → recipe → model → HITL

- **cache** is a prior resolution for this part and param (see Recall below).
- **recipe** is per-vendor deterministic table classification, the same `datasheet/derive/` recipes as
  above. It is precise, brittle across vendors, and cheap to write one vendor at a time.
- **model** is an inference backend (see below). It generalizes across vendors, is fuzzy, and
  needs an eval harness.
- **HITL** is a human, authoritative. Only a human resolution grants verified-comparable
  trust and confidence 1.0.

The trust predicates already in the parameter-IR are the gate between auto-use and
show-a-human. A low-confidence or `UnderSpecified` answer, or one whose condition is text-only
and therefore not `MachineComparable`, is surfaced as a suggestion rather than committed as a
silent fact. Auto-resolved values (recipe or model) keep method-tagged confidence below 1.0
and derive coverage from the conditions channel, the same as `derive/v0` does today.

### Pluggable resolver backends

One interface has several implementations, chosen and stacked per need.

- **Per-vendor recipe** is deterministic and high precision, at the cost of N-vendor
  maintenance.
- **LLM extractor/classifier** generalizes across vendors, is fuzzy, and needs behavioral eval
  and a confidence estimate.
- **Local few-shot / embedding-KNN** embeds the region, finds the nearest labeled prior, and
  suggests its label and value. It is cheap, runs on-device, and improves with every human
  label.
- **HITL** is the authoritative fallback and the trust anchor.

All return the same shape, so the chain composes without any backend knowing about the others.

### Two query shapes: scalar parameters and pin tables

The resolver serves more than one kind of query, both over the same faithful doc-IR and the
same `cache → recipe → model → HITL` chain.

- **Scalar limit**: `(part, parameter, condition)` returns a value plus range plus limit kind.
  This is what a check like `supply-exceeds-abs-max` needs, one number with its conditions. The
  result is a `PartSpec` parameter.
- **Pin table**: `(part)` returns a pin-function mapping, which pins carry which interface
  signals (a bus's CLK/CMD/DAT lines, a memory byte-lane, a transceiver's TXD/RXD). An
  interface-shape check can then derive its required-signal list from the host part's datasheet
  pin table rather than hand-authoring it. A standard bus pinout is generic rather than
  proprietary, so this is a shareable abstraction. The result is a structured pin to
  role/signal mapping, not a scalar.

Both shapes ride the same seam. A pin table is already a doc-IR `Table`, and a recipe
classifies it as a pin-function table and extracts the mapping, exactly as another recipe
locates an abs-max value. Both are trust-gated and recall-able. What differs is only the
result type. The same on-demand extraction that answers a scalar limit also, on a pin-table
query, populates an interface-shape check.

### Recall and suggestion

Every resolution, automatic or human, is stored keyed by roughly `(part-family, parameter,
vendor, region-shape)`. On a new query, nearest-prior over that store yields suggestions: the
same parameter on a sibling part, or a similar region shape on the same vendor. Query history
adds a "because you searched similarly last time" behavior. This is the few-shot idea used as
suggestion and warm start rather than as an auto-committed fact. It accelerates human review
and seeds the model backends. Auto-commit still requires the resolution chain to clear the
trust gate on its own.

### Coverage stays honest, at a better grain

Eager extraction reports "we extracted 300 of an unknowable total." Demand-driven reports "of
the parameters your rules queried, N answered automatically, M needed a human, K unresolved."
That is coverage relative to demand, the same discipline as the derivation run manifest's gap
list, at the grain that matters to a design.

### User flows

**Check-driven, no human.** `agni check --params` evaluates `supply-exceeds-abs-max` over each
supply net. For the LM1117 it reads `abs_max(LM1117, VIN)`. A cache miss follows, the TI recipe
locates the abs-max table and returns 20V at confidence 0.9, and the value materializes into
the cache. The rule compares the design's +24V supply against 20V, fires, and cites both the
design provenance and the datasheet page and table. No human was involved.

**Resolution escalates to a human at the point of demand.** The same walk, but the recipe
cannot confidently locate or parse the value, because docling mis-segmented the cell or the
abs-max condition is text-only and therefore not machine-comparable. The resolver opens the
workbench focused on the likely page and region, "confirm VIN abs-max for LM1117," and the
suggestion panel offers a warm start. Confirming caches the value with region-cited provenance
and upgrades trust to verified-comparable, and the rule then fires. This is human review
motivated by a real query rather than transcribing 30 params with no design in mind.

**Search-driven and exploratory.** A developer clicks a pin, runs a datalog search for a
parameter across parts, or asks what the datasheet says about a net. The same resolver runs
underneath, so search and checks share one cache and one resolution path.

### Open design points

- The resolver interface signature and where it lives (a params-tier provider in the Model
  versus a `check`-level service) is not settled.
- The home for a pin-function mapping is **settled**: it is an extension to the parameter-IR
  (`PartSpec.pins`, `PartSpec.packages`, `Parameter.pin_refs`), not a sibling contract. See
  [pin binding](#pin-binding) above for the shape and for the name-over-number precedence a
  resolver must follow. What remains open is the extraction, not the target.
- Async resolution UX for the slow model and human backends: the fast path resolves inline,
  and a slow path likely returns "pending" and re-answers on completion.
- The recall store's key and similarity metric (exact family/param versus embedding
  nearest-neighbor) is open, starting exact and adding embedding recall with the local backend.
- Multi-param whole-part analyses (a thermal or power budget, a part comparison) issue a small
  batch of queries rather than a singleton but stay demand-driven.

## How it joins into checks

The join key is part identity: `PartSpec.mpn` plus `PartSpec.manufacturer`, matching
`ir.BomLine.mpn` and `ir.BomLine.manufacturer`. The dependency points one way. The readers and
the design IR never import the parameter layer, and the validation join consumes both
contracts. When a design carries no BOM or MPN data (a bare netlist), the join has no key and
parameter checks skip, the same skip-not-false-pass behavior used for unseeded parts.

That join is by part identity only. The finer per-pin join described under
[pin binding](#pin-binding) is a property of the contract today and no rule consumes it yet: the
shipped rules still reach a terminal through a vendor-symbol alias table meeting a pin-type
inference, which is why they cannot tell two supply pins of one part apart. Pin-level query
relations and a pin-rating rule are separate work, and both are expected to keep the alias path as
the fallback for a part with no pin data.

`param.Set` and `param.LoadSet` hold the seeded corpus. The check Model's params tier
(`check.NewModelWithParams`, `Model.PartSpec`) is the join. It takes the BomLine MPN first,
else the component's MPN attribute, since the KiCad reader carries the MPN and Manufacturer
symbol properties into attributes, matched case-insensitively and nothing fuzzier.

Right-to-left resolution is only as good as the map from a schematic component to its
datasheet, MPN to part to documents. The KiCad reader already carries MPN, Manufacturer, and
BomLine into attributes, so the join is partly there, and a weak MPN resolution gives the query
nothing to point at.

The first datasheet-backed rule, `supply-exceeds-abs-max`, compares a power-input pin's rail
nominal (parsed from the net name, refusing ambiguity) against the spec's machine-comparable
absolute-maximum supply rows, resolved through the supply-symbol alias map in the Model layer.
Rule text never names a vendor symbol. Its findings carry the design site in `Prov` and the
datasheet citation (document revision, page, table, method, confidence) in the message. This
rule is already a right-to-left query: it starts from the design's supply net and interrogates
the datasheet, citing both sides. The demand-driven scheduling model above generalizes that one
shape rather than introducing a new one.

A citation also carries the value's verification state, derived at citation time from the revision
its `SourceDoc` records. That is what lets the review layer's ratification axis tell a fail backed by
a confirmed value apart from one backed by a confirmation of a superseded revision: `isUnratified`
treats `stale` and `unknown` as untrustworthy data, so the item reads Provisional (a re-confirm task)
rather than a hard Fail. Deriving it at citation time rather than stamping it on the finding means a
re-seed changes every subsequent answer with nothing to re-stamp. A value nobody verified is
unaffected and is still judged by method and confidence alone.

The second rule, `cap-voltage`, is the first spec-authored datasheet rule with no Go twin (see
[Rules and checks](../rules-and-checks/) for the spec-and-twin model). Its body is a
`check.Spec`, the join and the float compare live behind the `cap_voltage_detail` SpecFunc, and
the FFI's declared reads flow into the rule's derived metadata, so `param.cap_rated_voltage`,
`net.max_voltage`, and `component.mpn` appear as named relations without hand-maintained lists.
Rail voltage is declared through a `max_voltage` net attribute, else the name-derived nominal,
and the assertion is `Vrated >= rail_V x 1.25`, with the derate constant until rule
parameterization lands.

A user-facing walkthrough of running these checks with a parameter set is in
[the datasheets guide](../../guide/datasheets/). The broader map from software concepts to the
hardware nouns used here is in [the analogy reference](../../reference/analogy/).

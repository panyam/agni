# 26 — Parameter resolution (demand-driven, schematic-first)

Status: DECIDED direction, not yet built. This is the scheduling model for the
datasheet layer: what drives `PartSpec` population and when. It sits on top of the
existing contracts (doc-IR [21](21-document-ir.md), parameter-IR
[20](20-parameter-ir.md), derivation [24](24-derivation.md)) and changes none of
them. It changes only how they are driven.

Numbering note: minted as `26` against a docs tree that already has a `23`
collision; expect a possible renumber on merge (see the doc-number-collision
gotcha in the workspace notes).

## The problem this solves

docling gives us a faithful but semantically flat decomposition: pages, tables,
figures, text blocks, bboxes, raw cell grids. Across a corpus of different vendors
and part families that layer has no reliable global structure. Column orders,
header wording, and where a condition lives (a column, a footnote, or the section
header "at Ta=25°C unless noted") all differ. Trying to find one pattern over the
raw doc-IR is the wrong search. There is not one.

Given that, the obvious plan is eager extraction: classify every table in every
datasheet into a full `PartSpec`, then run rules against the design. That plan has
two defects:

1. **Most of the work is wasted.** A datasheet carries hundreds of parameters. A
   given schematic's use of a part cares about a handful (the supply tied to a
   rail, abs-max on pins that reach a net, a logic threshold where a net lands).
2. **Eagerly extracted params are unverifiable.** Extract 300 values with no
   design in hand and there is no signal for which are wrong. Nothing has been
   used against a real net.

## The decision: right-to-left, demand-driven

Start from the schematic, not the document. For each component, net, and pin, the
rules being evaluated ask for exactly the parameters they need. Each ask is
resolved against a cache; a miss triggers resolution (recipe, model, or human) and
backfills the cache. This mirrors how a hardware engineer actually uses a
datasheet: it is a reference consulted with a specific question, not a database
memorized in advance.

### The eager/lazy seam sits at doc-IR ↔ PartSpec

This is the crux. The two layers have their cost put where it is justified:

- **doc-IR is eager.** docling decomposing a PDF into faithful pages/tables/figures
  is a fixed one-time cost per file, produces no semantic claim, and is OCR-free
  for born-digital datasheets. Run it once per document, offline.
- **PartSpec population is lazy.** No whole-document semantic classification up
  front. A parameter enters the `PartSpec` cache only when a real query asks for
  it.

So "extract on demand" means *classify and transcribe on demand over already-faithful
doc-IR*, not *run docling on demand*. The slow pass is already done; the on-demand
step is a fast locate-and-confirm. That is what removes the usual objection to a
just-in-time design, that the developer clicks a rule and stalls on a heavy
extraction.

## Architecture: a lazy provider behind the Model params tier

We already have the query language and the evaluators. `query.Naive` over
`check.NewModel(d)` is the datalog surface; the Spec/Model rule layer is the rule
evaluator. Neither needs to change. What is new is one layer beneath them.

- **Enumerating which params a rule needs is not new.** It falls out of normal
  evaluation. When `supply-exceeds-abs-max` walks each supply net and reads
  `abs_max(part, "VIN")` from the Model, that read *is* the query. There is no
  separate planning phase; the planning is implicit in lazy evaluation.
- **What is new is a resolver: a lazy provider behind the Model's params tier.**
  Today the params tier is loaded eagerly and wholesale (`param.LoadSet` from a
  seed dir), and a miss means the rule silently skips (skip-not-false-pass). In the
  demand-driven model a miss triggers the resolution chain, materializes the result
  into the `PartSpec` cache, and answers. The loop is: cache miss → side-effecting
  resolution → backfill → answer.

The rules and datalog queries see the same relation they see today. It just
materializes on access instead of having been preloaded. Two consequences:

- The `agni query` search surface can answer datasheet questions
  (`abs_max(C, "VIN", V)`) that today it cannot answer unless the set was
  preloaded. The resolver is what lets search reach the datasheet at all.
- This slots into the existing `check.Available` gating (params tier empty is
  silent by construction) and the provider story that the codebase
  already earmarks for external vocabulary.

### Demand-driven converges toward eager, in priority order

The first design that uses an LM1117 pays to resolve VIN abs-max; the fiftieth hits
cache. Over a corpus of designs the cache accretes into a real `PartSpec`, but one
built from actual demand: exactly the useful parameters, each already used against a
real net, so each carries a confidence signal. The demand-driven cache is the eager
`PartSpec`, filled lazily and priority-ordered. The `param.PartSpec` contract
(parameter + conditions + range + limit kind + provenance + trust predicates) is
unchanged; only the scheduler flips from eager-batch to demand-driven.

## The resolution chain

A resolver backend answers one query — `(part, parameter, condition)` — with a
`PartSpec`-shaped result plus confidence and provenance. The chain runs in
escalation order and stops at the first confident answer:

    cache → recipe → model → HITL

- **cache** — a prior resolution for this part/param (see Recall below).
- **recipe** — per-vendor deterministic table classification (the `derive/`
  recipes). Precise, brittle across vendors, cheap to write one vendor at a time.
- **model** — an inference backend (see Model types). Generalizes across vendors,
  fuzzy, needs an eval harness.
- **HITL** — a human, authoritative. Only a human resolution grants
  verified-comparable trust and confidence 1.0.

The trust predicates already in the parameter-IR are the gate between "auto-use" and
"show a human." A low-confidence or `UnderSpecified` answer, or one whose condition
is text-only and therefore not `MachineComparable`, is surfaced as a *suggestion*,
never committed as a silent fact. Auto-resolved values (recipe or model) keep
method-tagged confidence below 1.0 and derive coverage from the conditions channel,
exactly as `derive/v0` does today.

### Model types are pluggable resolver backends

One interface, several implementations, chosen and stacked per need:

- **Per-vendor recipe** — deterministic, high precision, N-vendor maintenance.
- **LLM extractor/classifier** — cross-vendor generalization, fuzzy, needs
  behavioral eval and a confidence estimate.
- **Local few-shot / embedding-KNN** — the map-scanner idea reused: embed the
  region, find the nearest labeled prior, suggest its label/value. Cheap, runs
  on-device, improves with every HITL label.
- **HITL** — the authoritative fallback and the trust anchor.

All return the same shape, so the chain composes without any backend knowing about
the others.

### Two query shapes: scalar parameters and pin tables

The resolver serves more than one kind of query, both over the same faithful doc-IR
and the same `cache → recipe → model → HITL` chain:

- **Scalar limit** — `(part, parameter, condition) → value + range + limit kind`.
  This is what checks like `supply-exceeds-abs-max` need: one number with its
  conditions. The result is a `PartSpec` parameter.
- **Pin table** — `(part) → pin-function mapping`: which pins carry which interface
  signals (a bus's CLK/CMD/DAT lines, a memory byte-lane, a transceiver's TXD/RXD).
  This lets an interface-shape check derive its required-signal list from the host
  part's datasheet pin table instead of hand-authoring it. A standard bus pinout is
  generic, not proprietary, so this is a shareable abstraction. The result is a
  structured pin → role/signal mapping, not a scalar.

Both shapes ride the same seam. The pin table is already a doc-IR `Table`; a recipe
classifies it as a pin-function table and extracts the mapping, exactly as another
recipe locates an abs-max value. Both are trust-gated and recall-able. What differs
is only the **result type**: where a pin-function mapping lives (an extension to the
parameter-IR, or a sibling contract) is an open design point, tracked below. This is
the datasheet layer paying off twice: the same on-demand extraction that answers a
scalar limit also, on a pin-table query, populates an interface-shape check.

## User flows

**A. Check-driven, no human.** `agni check --params` evaluates
`supply-exceeds-abs-max` over each supply net. For the LM1117 it reads
`abs_max(LM1117, VIN)`. Cache miss. The TI recipe locates the abs-max table,
returns 20V at confidence 0.9. The value materializes into the cache. The rule
compares the design's +24V supply against 20V, fires, and cites both the design
provenance and the datasheet page/table. No human was involved.

**B. Resolution escalates to HITL at the point of demand.** Same as A, but the
recipe cannot confidently locate or parse the value (docling mis-segmented the
cell, or the abs-max condition is text-only and therefore not machine-comparable).
The resolver opens the workbench focused on the likely page and region:
"confirm VIN abs-max for LM1117." The suggestion panel offers a warm start (see
Recall). Confirming caches the value with region-cited provenance and upgrades
trust to verified-comparable. The rule now fires. This is HITL motivated by a real
query, not "transcribe 30 params with no design in mind."

**C. Search-driven / exploratory.** The developer clicks a pin, or runs a datalog
search for a parameter across parts, or asks "what does the datasheet say about
this net." The same resolver runs underneath, so search and checks share one cache
and one resolution path.

## Recall and suggestion

Every resolution, automatic or human, is stored keyed by roughly
(part-family, parameter, vendor, region-shape). On a new query, nearest-prior over
that store yields suggestions: the same parameter on a sibling part, or a similar
region shape on the same vendor. Query history adds the "because you searched
similarly last time" behavior. This is the few-shot idea again, used as *suggestion
and warm start*, never as an auto-committed fact. It accelerates HITL and seeds the
model backends. Auto-commit still requires the resolution chain to clear the trust
gate on its own.

## Coverage stays honest, at a better grain

Eager extraction reports "we extracted 300 of an unknowable total." Demand-driven
reports "of the parameters your rules queried, N answered automatically, M needed
HITL, K unresolved." That is coverage relative to demand, which is the same
silence-is-not-coverage discipline as the derivation run manifest's gap list, at
the grain that matters to a design.

## Dependency: design-side part resolution

Right-to-left is only as good as the map from a schematic component to its
datasheet(s): MPN → part → docs. The KiCad reader already carries MPN, Manufacturer,
and BomLine into attributes, so the join is partly there, but it is the load-bearing
prerequisite. A weak MPN resolution gives the query nothing to point at.

## Already validated

`supply-exceeds-abs-max` is already a right-to-left query: it starts from the
design's supply net and interrogates the datasheet, citing both sides. One instance
of this model is shipped and works. The decision here is to make that the general
shape and stop pursuing eager corpus-wide extraction, not to build something new.

## Out of scope / open questions

- The resolver interface signature and where it lives (a params-tier provider in
  the Model, versus a `check`-level service) is a design task, not settled here.
- The home for a pin-function mapping (the pin-table query shape): an extension to
  the parameter-IR versus a sibling contract. Start with the scalar shape; the seam
  must not preclude the pin-table result type.
- Async resolution UX for the slow (model, HITL) backends: the fast path resolves
  inline; a slow path likely returns "pending" and re-answers on completion.
- The recall store's key and similarity metric (exact family/param versus embedding
  nearest-neighbor) is left open; start exact, add embedding recall with the local
  backend.
- Multi-param whole-part analyses (thermal or power budget, part comparison) issue a
  small batch of queries rather than a singleton, but stay demand-driven.

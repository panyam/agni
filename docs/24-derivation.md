# 22 — Derivation (datasheet extraction as a deterministic pipeline)

The extraction stage of the datasheet layer:

    PartSpec = f(document, toolchain, recipes, patches)

Every input pinned, every output reproducible from a run manifest, re-runs
incremental. This is the engineering half of the derivation model; the process
rationale (why point-in-time extraction is brittle: revisions, extractor upgrades,
evaporating corrections) lives in the private research notes. Schema:
[protos/agni/v1/derive/derive.proto](../protos/agni/v1/derive/derive.proto);
implementation: the `derive/` package and `agni derive`; posture:
[CONSTRAINTS C16](../CONSTRAINTS.md).

## The stages (all deterministic)

1. **Classification by candidate titles.** Real producers emit datasheet tables
   untitled, and often fold the section band INTO the table as a merged header cell.
   Classification therefore tries candidate titles in order, the producer-attached
   title, embedded band cells, then heading-like text blocks above the table
   (nearest first, small overlap tolerance: real detected boxes touch), and the
   first candidate a recipe rule matches becomes the table's title and limit kind.
   A note line sitting between the heading and the table is harmless: it is a
   candidate that never matches a rule. Unmatched tables land in the manifest's gap
   list, never silently skipped.
2. **Header-row detection and tokenization.** The column-header row is found by
   recognized column names (Symbol/Parameter/Test Conditions/Min/Typ/Max/Units/
   Ratings), scanning past band rows; TI-shaped tables with an unlabeled row-label
   column fall back to column 0 as the name column (symbol stays empty, honestly).
   Value cells tokenize by shape: plain numbers, "±N", "A to B" ranges (spaced signs
   included). Conditions parse two structured forms ("SYM = N UNIT",
   "A <= SYM <= B UNIT"); everything else is kept raw-only, which the docs/20
   semantics then correctly exclude from automatic comparison. Non-title band text
   ("TA = 25C unless otherwise noted") becomes a table-level condition on every row.
3. **Patches, applied last.** A patch is one pinned human correction to one cell of
   one exact document, keyed by document content hash + PRE-patch table content
   hash, so a new revision or a re-detection invalidates it by construction (it
   stops matching and the manifest reports it unapplied). Empty text clears a cell;
   a patch at an empty position inserts, so a producer cell-placement error (the
   real LM1117 case: docling put the abs-max 20 under MIN) is corrected by a
   clear + insert pair. Applied last means a verified fix can never regress.
4. **Validation and emission.** The emitted PartSpec must pass `param.Validate`;
   every parameter carries provenance (page, attached table title,
   `method: "derive/v0"`, confidence 0.9, only a human verification earns 1.0).

## Trust defaults (the honesty ladder)

- Rows from tables with **no conditions channel** stay `ConditionCoverage`
  UNSPECIFIED: under-specified until a human verifies, never UNCONDITIONAL, because
  header defaults this stage cannot prove captured may qualify every row.
- Rows with a captured conditions channel (column and/or band) are COMPLETE; raw-only
  members still make the row machine-incomparable, the intended middle state
  (surface to a human, no auto-compare).
- Derived confidence is a constant 0.9 below the human ceiling; the verification
  queue (follow-up) upgrades confirmed rows to `method: "human-verified"`,
  confidence 1, and demotions become patches.

## The manifest (coverage accounting)

Every run emits a `RunManifest`: doc content hash, doc producer + derive version (the
toolchain pin), part identity, recipes matched, patches applied, and the gap list:
unclassified tables, unparsed rows, raw-kept conditions, unapplied patches. Silence
never reads as absence: what the run did not extract is enumerated, not implied.
Ensemble agreement fields exist in the manifest and stay zero until a second
extraction path lands.

## The golden gate

The hand-encoded fixtures in `param/testdata/` are the first verified golden corpus:
`derive/derive_test.go` asserts that deriving the raw-shaped BSS138 doc-IR reproduces
every hand-encoded row (symbol, kind, min/typ/max, unit). Any change to the derive
stage must keep that agreement or deliberately update the goldens, the regression
discipline the render golden SVGs established, applied to extraction. Bump `Version`
on behavior changes.

## Deliberately deferred

- **Ensemble/VLM second path** and agreement gating: the manifest carries the stats
  fields; the deterministic path must soak first.
- **Verification queue** (crop-beside-row UI, confidence upgrades): needs the
  persistence port (C13/C16); patches are the storage half of it, already in place.
- **Persistence store**: specs and manifests are files today; the injected-port
  store arrives with the service tier.
- **Realtime on-demand derivation** (latency budget, where it runs).
- **Curves/graphs** as data; figures remain provenance targets.
- **Recipe catalog location**: recipes are shareable engine content (vendor layout
  knowledge, C16); whether they ship in-repo as a `recipes/` catalog or as packs is
  open until a second vendor family accumulates.

# 21 — Doc-IR (source documents as a contract)

Enforceable rules in [/CONSTRAINTS.md](../CONSTRAINTS.md); a `CN` reference (e.g. C9) points to constraint N there.

The doc-IR is the intermediate artifact of the datasheet extraction pipeline: a
source document (datasheet PDF, app note) decomposed into pages, tables, figures,
and text blocks, with cell structure and bounding-box provenance. Schema:
[protos/agni/v1/doc/doc.proto](../protos/agni/v1/doc/doc.proto); loading,
invariants, and the in-process query surface: the `doc/` package; a prototype
producer: `tools/pdf2doc/`. It sits between the raw document bytes and the
parameter-IR ([docs/20](20-parameter-ir.md)): N document parsers produce doc-IR;
recipes, LLM proposal stages, the human verification UI, and revision diffing
consume doc-IR and never touch the source bytes. The derivation model it serves is
documented in the private research notes.

Status: PROVISIONAL, like the parameter-IR: one prototype producer (docling) and a
hand-authored fixture exercise it; names and shape may change until the extraction
pipeline runs end to end.

## Naming

The artifact is deliberately named for documents, not tables or datasheets. It must
carry more than tables (figures are provenance targets; headings and footnotes are
classification context; the page text layer feeds search), so "table-IR" would
mislead. And nothing in the decomposition is datasheet-specific (app notes, errata,
reference manuals decompose identically), so "datasheet-IR" would overfit the
current corpus. `doc` joins the contract family: `ir` models designs, `geom`
geometry, `param` parameters, `doc` source documents.

## Identity and stability (what a consumer may rely on)

- **Documents are keyed by the hash of their source bytes.** A datasheet revision is
  a different Document by construction; nothing is ever mutated.
- **Region ids** (`p2.t1`) are deterministic within one derivation and are the
  address for crops, review-queue items, and in-derivation navigation. They are NOT
  stable across producer versions: a detection change renumbers.
- **Table content hashes** (`doc.TableHash`: grid shape + cell position/span/text +
  footnotes, excluding bboxes, ids, confidence, and header flags) are the
  cross-version identity. Two derivations of the same printed table hash equal even
  when detection nudges coordinates; revision diffing skips unchanged tables on this
  key. `doc.Validate` recomputes and enforces stored hashes, so a validated doc-IR's
  hashes can be trusted without recomputation. The prototype producer replicates the
  hash byte-for-byte in Python; the Go validator is the referee.
- **Coordinates** are page-local, top-left origin, y-down, in PDF points. Producers
  reading PDF-native (bottom-left, y-up) boxes flip at emit time.

## Querying: two tiers

Tier 1, the `doc/` package, in-process and deterministic, what recipes, tests, and
the revision differ use:

- `TablesMatching(d, regexp)`: the recipe primitive: select tables by title
  pattern, never by id (ids are not version-stable).
- `TableByID` / `FigureByID`: addressed access within a derivation (crops, queue).
- `CellAt` / `CellText`: grid access; a merged cell appears once, at its top-left.
- `PageText`: the page's text blocks joined; the full-text-search source.
- `FindTableForProv(d, page, label)`: resolves a `param.ParamProvenance` locator
  (page + table label, matched by title equality or containment either way) to a
  table. The committed cross-contract test proves the param fixture's
  provenance resolves against the doc-IR fixture.

Tier 2, deferred to the extraction store: a Connect `DocService`
(C2/C13) over the persisted corpus, document by hash, region by id (verification
crops), and full-text search over an index built from `PageText` + table cells,
so "not extracted yet" is searchable and never a dead end. The index engine choice
is deliberately open; the schema's obligation to that tier is already paid (stable
addressing, content hashes, text retained).

## What the real producer taught (prototype findings)

Running `tools/pdf2doc` (docling 2.x) over the two datasheets:

- **Structure is solid**: every table validated (grid consistency, hash match),
  including a 32x8 electrical-characteristics table, and cell text is faithful.
- **Table titles come back empty.** Datasheet tables are headed, not captioned, and
  the producer does not attach nearby headings. Title attachment is therefore
  recipe-layer work (nearest heading text block above the table bbox), which is why
  `Table.title` resolution and the recipe tests run against the hand-authored
  fixture (the post-recipe shape), not raw producer output.
- **Symbol text needs normalization**: subscripts arrive space-split ("V GSS").
  A recipe-layer tokenizer concern; doc-IR stores text as
  extracted, faithful to the parse.

## What is deliberately absent

- **No curve/graph data.** Figures carry caption + bbox so provenance can point at
  them and a human can jump there; extracting curve data waits for real extractor
  output to design against.
- **No semantic classification** (this-table-is-abs-max). That is the recipe layer's
  output, recorded in the parameter-IR; doc-IR stays a faithful decomposition with
  no interpretation, which is what makes it reusable across recipe versions.
- **No cross-document corpus structure** (part to documents). That is the store's
  join.

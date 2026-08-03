# Agni engineering docs

Architecture and implementation docs for the EDA tooling engine (ingestion, IR, diff,
rules, geometry, rendering). Enforceable rules live in [/CONSTRAINTS.md](../CONSTRAINTS.md); a `CN` reference in these docs (for example `C9`) points to constraint N there.

Research, market, competitor, and strategy analysis is deliberately kept out of this
repo so it can be shared freely.

**Using the tool rather than building it?** The [user guide](userguide/README.md) is for
hardware engineers running reports, datasheets, and diffs, and opens with a
[software-to-hardware concepts map](userguide/concepts.md). The docs below are the
engineering/internals track (the future `docs/devguide/`).

Writing docs? [STYLE.md](STYLE.md) is the documentation style guide (the two tracks, the
audience-mapping principle, the real-output rule, prose conventions).

New here? [GETTING_STARTED.md](GETTING_STARTED.md): build agni, install the source/golden
EDA tools (KiCad, xschem, gEDA), point `--symbol-path` at symbol libraries, and run a golden
comparison. [NATIVE_VERIFICATION.md](NATIVE_VERIFICATION.md) is the per-format table of native
tools and the `agni native render` / `agni native open` commands that drive them. For the browser viewer, [WEB_WALKTHROUGH.md](WEB_WALKTHROUGH.md) is a ten-minute
guided tour over committed fixtures. Coming from software? [ANALOGY.md](ANALOGY.md) maps every
schema concept to its software analogy (classes, instances, lockfiles, type stubs, source
maps), with circuit examples and diagrams.

## Architecture

- [13 — Ingestion & IR architecture](13-ingestion-ir-architecture.md): readers to one IR, lossless + provenance, emit tiers, legal ingress ordering. Geometry as a keyed sidecar.
- [14 — Stack & architecture decision](14-stack-and-architecture.md): Go engine, proto IR, WASM optional, thin TS view, boundary discipline.
- [15 — Presenter-contract architecture](15-presenter-contract.md): duplex semantic contract, presenter mandatory + per-surface runtime (the shipped viewer runs a TS presenter over Connect; WASM stays the option for offline/in-browser reuse).
- [16 — Geometry & rendering](16-geometry-and-rendering.md): geometry sidecar representation (three tiers: logical proto contract / columnar bytes transport / compute Go form) and the scalable WebGL renderer plan.
- [17 — IR v0](17-ir-v0.md): the first versioned IR: neutral vocabulary, two layers (semantic + fidelity), tiered maturity (frozen netlist tier / provisional physical tier), provenance model, and the promotion-rule safeguard (CONSTRAINTS C9).
- [18 — Semantic diff](18-semantic-diff.md): the diff engine over the IR: identity strategy, the change taxonomy (Equal/Soft/Hard/Renamed/New/Deleted), rename detection, provenance-annotated findings, and the hard cases.
- [19 — Rules & checks](19-rules-dsl.md): the rules layer: rule expressiveness tiers, the rules-assert/analysis-computes boundary, technical prior art, and the phased evaluation model (embedded rules library first, declarative DSL later).
- [20 — Parameter-IR](20-parameter-ir.md): the third contract (one parameter-IR, N datasheet extractors): parameter + test conditions + range + limit kind + provenance, the under-specification guard, and the join to the design IR by part identity. PROVISIONAL.
- [21 — Doc-IR](21-document-ir.md): the extraction pipeline's intermediate: a source document decomposed into pages/tables/figures/text with bbox provenance, content-hash identity for revision diffing, and the two-tier query surface (in-process helpers now, corpus service + full-text search with the store). PROVISIONAL.
- [23 — Authoring a check rule](23-rule-authoring.md): the practical path from checklist item to shipped rule: the sentence-then-guards method, spec-first authoring with the twin discipline, the doc-file requirement, fixture shapes, four-level verification, narrated over a real rule.
- [22 — Net solving & the hierarchy walk](22-net-solving-and-hierarchy.md): how implicit schematic connectivity becomes nets: the shared netgraph solver (point-union, label-union, rank naming), KiCad's connection-point rules pinned against kicad-cli (mid-span labels, endpoint-only pins, name escapes), and the multi-sheet walk (instance scoping as fully-qualified names, coordinate bands, port stitching, the WS1-017 completeness witness).
- [23 — The web app](23-web-app.md): the browser viewer and visual diff over `agni serve`: the mount model, the four Connect services and their contract, the render/highlight contracts both renderers share, the client composition (islands, presenter, dock, router), and the recorded C3 deviation. Runnable tour: [WEB_WALKTHROUGH.md](WEB_WALKTHROUGH.md).
- [24 — Derivation](24-derivation.md): extraction as a deterministic pipeline: PartSpec = f(document, toolchain, recipes, patches); candidate-title classification, tokenizers, patches-last, run manifests with gap lists, the trust ladder (derive/v0 confidence < human), and the golden gate against the hand-encoded corpus. Posture: CONSTRAINTS C16.
- [25 — Open core: engine + overlay](25-open-core.md): the public Apache-2.0 engine and the private overlay that depends on it: the two personas, what lives where (the doc-placement rule extended to code), the two extension seams (`formats.Register`, `check.RegisterSource`), how an overlay requires the engine, and the `examples/overlay` reference. Posture: CONSTRAINTS C18. Practical how-to: [OVERLAY_AUTHORING.md](OVERLAY_AUTHORING.md).
- [26 — Parameter resolution](26-parameter-resolution.md): the scheduling model for the datasheet layer (schematic-first, demand-driven): the eager/lazy seam at doc-IR ↔ PartSpec, the lazy provider behind the Model params tier (cache → recipe → model → HITL), model types as pluggable backends, the check/HITL/search user flows, recall-as-suggestion, and demand-relative coverage. DECIDED direction, not yet built.

## Format references

- [EDIF primer](edif-primer.md): the EDIF 2.0.0 netlist (`.edn`) format we ingest.
- [EDIF schematic primer](edif-schematic-primer.md): the `.eds` SCHEMATIC export: geometry constructs, what else is in the file, orientation math, join keys, grammar.
- [xschem grammar](../xschem/GRAMMAR.md): the xschem `.sch`/`.sym` subset: object stream, netlist assembly, and faithful geometry (symbol artwork + `@name`/`@value` field templates).
- [gEDA grammar](../geda/GRAMMAR.md): the gEDA gschem `.sch`/`.sym` subset: line-oriented objects, geometric netlisting, faithful geometry, and `G` picture → `geom.Image`.

`.sch` is shared by xschem, gEDA, and legacy KiCad; the CLI/tree sniff the header to pick.
Both readers need the symbol library (`--symbol-path`) for pin-level nets and faithful symbol
artwork. Details and the golden-compare setup are in [GETTING_STARTED.md](GETTING_STARTED.md).

## Examples

Runnable demokit walkthroughs of the engine live in [../examples](../examples/README.md),
one per feature, over a shared `common` reuse package. They are living docs: run one live,
step through it in a TUI, or render it to markdown. Each shippable feature ships a runnable
example ([CONSTRAINTS C10](../CONSTRAINTS.md)); the how-to is
[examples/CONVENTIONS.md](../examples/CONVENTIONS.md).

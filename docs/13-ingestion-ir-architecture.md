# Ingestion platform & IR architecture

See [README](README.md). Enforceable rules in [/CONSTRAINTS.md](../CONSTRAINTS.md); a `CN` reference (e.g. C9) points to constraint N there. Builds on the file-formats / lock-in analysis and the project's
moat-path thinking. This is the core platform bet.

## The shape: compiler / Pandoc architecture

Many **readers** (one per format) normalize into a single **IR**; many **writers**
emit back out. This is LLVM (many frontends -> one IR -> many backends) and, closest
to our case, **Pandoc** (universal document converter: every format reads into one
central AST, writes out of it). Pandoc is the cleanest prior art for "ingestion
platform = a set of tools around one IR." Study it.

The "set of tools" is right: the reader layer is heterogeneous by necessity.

| Format class | Examples | Reader approach |
|---|---|---|
| Binary natives | Altium, Cadence Allegro | Wrap vendor extractor (`extracta`) or binary deserializer |
| Text grammars | netlists, SPICE, IBIS, Touchstone, KiCad s-expr | tree-sitter / parser combinators |
| XML | IPC-2581 | standard XML tooling |
| Record/tabular | ODB++ | custom record reader |

All target one IR.

## Fidelity contract per reader

"Lossless" is a per-adapter property, not a vague platform promise. Each reader
**declares what it preserves**: e.g. `IPC-2581 reader: lossless`, `ODB++ reader:
lossless to ODB++`, `extracta reader: lossy, drops X/Y`. If the source tool
(`extracta`) is lossy; that is the ceiling and nothing downstream recovers it; we
accept and document it. The round-trip oracle (below) then applies only where a
reader claims losslessness.

## Emit: tiered, not a universal emitter

We do NOT need every emitter built in. Tier the write path:
1. **Built-in emitters for open formats** we control end to end: IPC-2581, ODB++,
   Gerber, KiCad. Cheap, no dependency.
2. **Drive the native tool to write its own format.** Xpedition writes Xpedition,
   Altium writes Altium, via their automation API or by importing our IPC-2581/ODB++.
   We never emit their binary. This is also the legally clean way to produce a native
   format (using the vendor's tool as intended).
3. **Community/optional emitters** fill gaps over time.

This is Pandoc's posture: a rich-but-not-exhaustive set of writers, not a guarantee
of every target.

## Lossless + annotations (the hard, valuable part)

Prior art PL has already solved:
- **Full-fidelity / lossless syntax trees** (Roslyn red-green trees, rust-analyzer
  `rowan`, tree-sitter CSTs): a concrete layer retains everything (ordering, trivia,
  fields we don't model) alongside the clean semantic view we analyze.
- **Provenance spans / source maps:** every IR node back-points to its origin (file,
  byte offset, record id). Buys three things at once: lossless reconstruction,
  "finding maps to exactly this line/figure" for trust, and **surgical edits**
  (rewrite only the changed region, leave the rest byte-identical).
- **Unknown-field preservation:** carry unmodeled fields opaquely so write-back never
  drops them. Forward-compat hedge against format churn; makes "lossless" survive our
  own incomplete coverage.

**Two-layer IR:** a normalized semantic layer (Altium-net and Cadence-net look the
same -> cross-format analysis + DSL) PLUS a format-fidelity layer (retains quirks for
round-trip). Provenance links the two. Normalization abstracts detail away; the
fidelity layer is how we don't lose it.

**Geometry is a keyed sidecar, not in the core IR.** Render data (symbol shapes,
placements, wire routing, pin coordinates) lives in a separate artifact that references
the core IR by stable keys (`ref_des`, net name, `port_ref`, plus provenance
`source_id`), joined at render time. Diff/rules/sim never carry graphics; the renderer
loads (core IR + geometry sidecar) and joins. This keeps heavy graphics off the hot
paths (C7) and lets geometry come from a different source than connectivity (e.g. the
EDIF `.eds` schematic vs the `.edn` netlist).

## Round-trip as a test oracle (pays for itself)

`parse then emit` = identity for unchanged input. Run the whole real-file corpus
through read-then-write, assert byte-identical (or semantically identical).
Property-based + differential testing. Directly attacks the treadmill's worst burden
(validation): catches parser regressions and silent format-churn breakage
automatically, not when a customer file explodes.

## Two nuances that decide if it works

1. **"Lossless" is relative to the ingestion source, not the native file.** An
   extractor/export (ODB++, `extracta` output) has already dropped info the native
   binary held. So we can be perfectly lossless w.r.t. the interchange format we read
   while still lacking native-only detail. Promise the right fidelity:
   lossless-to-ODB++ is achievable; lossless-to-Altium-native is not unless we read
   the native file directly.
2. **Write-back into the native tool goes through interchange or the API, not by
   emitting their binary.** Round-trip by emitting ODB++/IPC-2581 for the tool to
   import, or by driving its automation API. The "way back" is IR -> open interchange
   -> tool, plus provenance for surgical fidelity. Still genuinely lossless along the
   path we control.

## Ingress ordering: reverse engineering is the last resort (legal)

RE of proprietary binaries is low on the list, mostly for legal reasons (not legal
advice; get counsel on specifics):
- **EULAs typically prohibit reverse engineering.** Breach risks contract claims and
  the vendor relationship we depend on.
- **DMCA anti-circumvention** if any encryption/protection is involved (encrypted
  SPICE, protected files). A separate, worse category.
- **Trade-secret / copyright exposure.** Clean-room interop has some footing but is
  jurisdiction-dependent, expensive, and EULA terms can override it.
- **Enterprise-sales blocker (may matter most).** Aerospace/defense/automotive run
  procurement + legal review. A legally questionable ingestion method gets killed in
  review regardless of actual litigation risk.

Sanctioned ingress ordering:
1. **Official automation API / export** (vendor-blessed).
2. **Open standards / documented formats** (IPC-2581, ODB++ spec, KiCad, Gerber).
3. **Official extractors** (`extracta`).
4. **Community RE parsers** (e.g. Altium open parsers): fine for prototyping, but
   they inherit the legal questions; relying on them commercially imports that risk.
5. **In-house reverse engineering:** last resort, only where no sanctioned path
   exists, and only after a deliberate legal check.

Convergence: this is the SAME ordering the technical argument pointed to (ingest via
export/API). Easier path and legally clean path stack instead of competing, a strong
signal the architecture is right.

## Why this is strategically bigger than it looks

A lossless IR with provenance is a capability tier above AllSpice (read-only diff/
review). It enables **surgical write-back, automated fixes, and transformations**,
the same machinery the design-as-code opportunity
needs to emit designs. The IR becomes the
platform asset that the diff, rules DSL, analysis, and corpus all sit on. Owning a
great neutral hardware-design IR is itself a moat: a competitor must rebuild the
whole thing to reach parity. This is the LLVM-style bet: the IR is the product even
when no single piece around it looks like one. It is the concrete form of the IR-as-moat
bet.

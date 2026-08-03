# 20 — Parameter-IR (datasheet parameters as a contract)

The parameter-IR is the engine's third contract. The design IR is one IR with N format
readers (docs/13, docs/17). The geometry sidecar is one geom proto with N producers
(docs/16). The parameter layer is one parameter-IR with N datasheet extractors. Schema:
[protos/agni/v1/param/param.proto](../protos/agni/v1/param/param.proto). Invariants
and loading: the `param/` package. This document records the design decisions; it
builds on the private research notes on component data.

Status: PROVISIONAL, the same posture as the IR's tier-2 physical messages. No
extractor populates the schema yet; it is validated by two hand-encoded fixtures
(`param/testdata/`) transcribed from real datasheets. Names and shape may change until
the first extractor (WS10-002) lands.

## The core decision: a parameter is not a scalar

"RDS(on) = 3.5 Ω" is not a fact about a part. "RDS(on) max 3.5 Ω at VGS = 10 V,
ID = 0.22 A, TJ = 25 °C, pulse-tested" is. A validation engine that compares design
state against datasheet limits (WS10-003) is only safe if the schema makes it
impossible to hold the first form without noticing. Three schema features enforce
that:

1. **`RangeValue` has no bare-scalar form.** Every value is a min/typ/max triple with
   explicit presence (proto3 `optional`), so "max only" (an absolute-max table row),
   "typ only" (an uncharacterized typical), and "min/max" (an ensured band) are all
   distinct, and none of them collapses into a lone number.
2. **`LimitKind` is a first-class enum**, not free text: `ABSOLUTE_MAX` (stress
   rating), `RECOMMENDED_OPERATING` (the vendor's functional envelope), and
   `CHARACTERISTIC` (measured behavior under stated test conditions). A rule that
   checks a net voltage against a limit dispatches on this field; the three kinds have
   different meanings and different safe uses. `UNSPECIFIED` fails validation:
   extractors classify or drop.
3. **`ConditionCoverage` makes under-specification explicit.** A parameter whose
   condition list is not asserted complete (`COMPLETE`) or genuinely condition-free
   (`UNCONDITIONAL`) is under-specified: `param.UnderSpecified` returns true and
   consumers skip or flag it, never compare against it. A conditions-stripped value is
   worse than no value, because it produces confident-but-wrong findings.

Conditions themselves (`Condition`) capture an exact point (`eq`), a range
(`min`/`max`), or a one-sided bound, always with the source text in `raw` so an
unstructured condition ("VDS = VGS", a temperature-range phrase) is retained verbatim
rather than dropped.

## Provenance is non-negotiable

Every `Parameter` carries a `ParamProvenance`: source document (by reference to a
`SourceDoc` declaring the vendor doc number and revision), page, table/figure as
titled, extraction method, and a confidence in (0, 1]. This mirrors the discipline the
`check/` package already enforces (a `Prov` on every finding) and it is the product
stance, not a nicety: an extracted value an engineer cannot one-click verify against
the exact datasheet page is a liability. Zero confidence is invalid by construction; a
value nobody stands behind is not emitted. Hand-encoded values use `method: "hand"`,
`confidence: 1`.

`SourceDoc.locator` is corpus-local on purpose. The deployment posture (WS10-002) is
internal-seed: the customer's datasheets and the extracted database never leave their
boundary, so locators are only meaningful inside one deployment and the schema carries
no assumption of a shared global document store.

## Join to the design IR

The join key is part identity: `PartSpec.mpn` + `PartSpec.manufacturer`, matching
`ir.BomLine.mpn` / `ir.BomLine.manufacturer`. The dependency points one way: the
readers and the IR never import the parameter layer; the WS10-003 validation join
consumes both contracts. When a design carries no BOM/MPN data (a bare netlist), the
join has no key and parameter checks skip, the same skip-not-false-pass behavior
WS10-003 specifies for unseeded parts.

Implemented (the WS10-003 slice): `param.Set`/`param.LoadSet` hold the seeded corpus;
the check `Model`'s params tier (`check.NewModelWithParams`, `Model.PartSpec`) is the
join, BomLine MPN first, else the component's MPN attribute (the KiCad reader carries
the MPN/Manufacturer symbol properties into attributes), case-insensitive, nothing
fuzzier. The first datasheet-backed rule, `supply-exceeds-abs-max`, compares a
power-input pin's rail nominal (parsed from the net name, refusing ambiguity) against
the spec's machine-comparable absolute-maximum supply rows, resolved through the
supply-symbol alias map in the model layer. Rule text never names a vendor symbol.
Its findings carry the design site in `Prov` and the datasheet citation (document
revision, page, table, method, confidence) in the message.

The second rule, `cap-voltage` (WS10-005), is the first spec-authored datasheet rule
(docs/19 "a rule is a value", no Go twin): the body is a `check.Spec`, the join and
the float compare live behind the `cap_voltage_detail` SpecFunc, and the FFI's
declared reads flow into the rule's derived metadata, `param.cap_rated_voltage`,
`net.max_voltage`, `component.mpn` appear as named relations without hand-maintained
lists (the WS3-004 fact-capture seed). Rail voltage is declared (a `max_voltage` net
attribute, else the name-derived nominal); the assertion is
`Vrated >= rail_V x 1.25`, with the derate constant until rule parameterization lands.

## Comparison semantics before normalization

Values, units, and symbols are as printed, so the comparison layer (WS10-003) meets
vendor variety before WS10-004 normalization exists. Three rules keep it honest, all
variants of the same posture: when a comparison cannot be made safely, stay silent,
never improvise.

- **Three trust states, not two.** `param.UnderSpecified` says a row's conditions are
  not trustworthy at all: skip it. But a row can be fully captured and still carry a
  condition that exists only as text ("VDS = VGS", a temperature-range phrase); it is
  honest data a human can evaluate next to its provenance, and no machine can.
  `param.MachineComparable` names that boundary: only rows whose every condition is
  structured (`eq` or `min`/`max`) may enter an automatic comparison. The middle state
  (captured but text-only) is surfaced, never auto-compared.
- **Unlike unit strings are under-specified for comparison.** Until WS10-004 provides
  canonical units, a comparison between values whose unit strings differ ("mA" vs
  "A", "mOhm" vs "Ohm") is skipped or flagged, never ad-hoc converted. Conversion
  logic written at a call site is a second, informal normalization layer, which is
  exactly the drift WS10-004 exists to prevent.
- **Vendor symbols never appear in rule text.** The same physical parameter prints as
  "VDC", "WV", or "Rated Voltage" depending on vendor. `symbol` is the per-vendor
  match key, but the lookup lives behind the join/Model layer (a per-corpus alias
  map), so a rule asks for a concept and no rule hardcodes one vendor's spelling.
  This precedent matters before the first datasheet-backed rule (WS10-005) lands.

## What is deliberately absent

The C9 discipline (a field earns its place only when a second producer would populate
it) applies here with "vendor datasheet" in place of "format":

- **No canonical parameter ids, no canonical units.** `Parameter.canonical_id` exists
  but stays empty; `symbol` and `unit` are as printed. Normalization and the ontology
  are WS10-004, and doing them prematurely would bake one vendor's vocabulary in as
  "canonical". The fixtures keep "IOUT = 800 mA" as 800 mA, not 0.8 A.
- **No graph/curve data.** Derating and SOA curves are real and valuable, but they are
  the harder extraction (WS10-002 defers them) and their shape (sampled curves? fitted
  models?) should be designed against real extractor output, not guessed.
- **No verification-workflow state** (reviewed-by, approved). The human-in-the-loop
  workflow belongs to the extraction pipeline and store (WS10-002/003); until it
  exists, `method` + `confidence` carry what the schema needs. Anything extra goes in
  `attributes`.
- **No package/pin data.** Package compatibility checks (WS10-003) join through the
  design IR's footprint tier when they arrive; duplicating package data here would
  create a second source of truth.

## Why proto (and not a lighter schema)

The ticket left proto vs a lighter schema (YAML/JSON) open. Proto, for the same
reasons the other two contracts are proto (CONSTRAINTS C2/C8): the parameter-IR is a
cross-runtime contract (Go engine, TS viewer surfaces, future extractors in whatever
language the VLM tooling prefers), and hand-written parallel types are exactly the
drift C2 exists to prevent. The human-authoring cost is paid in textproto, which the
fixtures use and which stays diffable and comment-friendly. It is a separate proto
package (`agni.v1.param`) rather than a corner of `ir`: different producers,
different consumers, different lifecycle.

## Worked examples

Two fixtures, both transcribed by hand from the cited datasheet revision, values and
units as printed:

- **`param/testdata/lm1117.textproto`** (TI LM1117 LDO, SNOS412Q rev Jan 2023): the
  three limit kinds on one part: abs-max VIN 20 V, recommended-operating VIN 15 V,
  and dropout voltage as a conditional characteristic. The dropout rows show why rows
  with different condition sets stay distinct parameters: typ 1.2 V holds at
  TJ = 25 °C, max 1.3 V holds over the 0 to 125 °C junction range, both at
  IOUT = 800 mA.
- **`param/testdata/bss138.textproto`** (BSS138 N-FET, Fairchild rev C(W)): the
  canonical conditional parameter, RDS(on) specified three times (VGS = 10 V,
  VGS = 4.5 V, and VGS = 10 V at TJ = 125 °C), plus a table-header default
  ("TA = 25 °C unless otherwise noted") encoded as an explicit condition rather than
  silently dropped, and the pulse-test footnote retained in `attributes`.

`param/param_test.go` asserts both fixtures validate and that the encodings above are
present, so the worked examples are executable, not prose.

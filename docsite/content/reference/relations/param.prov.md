---
title: "param.prov"
description: "the citation of a datasheet parameter — the SourceDoc title, page, and table/figure it was read from (needs --params)"
---

### What it is

`param.prov(mpn, symbol, doc, page, section)` yields one row per parameter of a datasheet spec that
joined to a part in the design, keyed by manufacturer part number (`mpn`) and the parameter's
datasheet symbol. Where `param` and `param.range` carry the parameter's VALUE, `param.prov` carries
its CITATION: `doc` is the source document's title (vendor document number and revision), `page` is
the page the value was read from, and `section` is the table or figure on that page. It answers "where
did this number come from", so a value and its provenance are both queryable from the same seeded
spec.

This is the datasheet tier of the query surface. It is EMPTY unless `agni` is run with
`--params <dir>` pointing at a seeded `PartSpec` corpus — skip-not-false-pass by construction.

### For hardware engineers

A row is the footnote you would want under any spec-derived claim: the document, the page, and the
table it came from, so a review finding can be traced back to the page a reviewer opens. It does not
carry the extraction `method` or `confidence` as columns (a finding gets those; see below), but the
`doc`/`page`/`section` are enough to locate the value in the PDF. As everywhere in the param tier, a
citation is required for a fact to exist: a value with no provenance is not a verifiable fact.

### For software engineers

`param.prov` is the provenance projection of the same seeded parameters `param`/`param.range` expose,
split off so a query can ask for the citation without the value (or join both). The design-side
identity is still `component.mpn(ref_des, mpn)`, so the join is unchanged. The tuple has room for the
readable `doc` title, the `page`, and the `section`; the extraction `method` and `confidence` are not
columns (the fact tuple has no slot for them). A datalog-authored RULE that wants the FULL citation
on its findings — including confidence, which flags a value that should be verified before it is
trusted — declares `param_symbol` on its query binding; `RuleFromQuery` then resolves the complete
citation from the subject component's spec via `check.DatasheetProvFor` and attaches it to
`Finding.DatasheetProv`, which the review report renders.

### Go projector

`paramProvFacts` in `check/facts.go` iterates `Model.Components()`, reads each component's MPN via
`Model.ComponentMPN` and its spec via `Model.PartSpec`, dedupes by MPN, and emits one row per
`spec.Parameters` entry. `Value` is the SourceDoc title resolved from the parameter's `doc_ref`, `Num`
is the page, and `Conditions` is the table/figure. It shares the join and dedup shape of `paramFacts`;
the two differ only in which fields they surface. Empty without `--params`.

### Datalog

Needs `--params`. Cite every datasheet parameter of each part, as MPN, symbol, document, page, and
section:

```
param.prov(?mpn, ?sym, ?doc, ?page, ?section) => ?mpn, ?sym, ?doc, ?page, ?section
```

Join a value to its citation — the max ceiling and the page it came from, together:

```
param(?mpn, ?sym, ?max), param.prov(?mpn, ?sym, ?doc, ?page, ?section) => ?mpn, ?sym, ?max, ?doc, ?page
```

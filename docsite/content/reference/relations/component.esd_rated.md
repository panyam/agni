---
title: "component.esd_rated"
description: "the part carries a datasheet ESD rating at or above the credit floor (needs --params)"
---

### What it is

`component.esd_rated(ref_des)` yields one row per component whose joined datasheet spec declares
an ESD tolerance at or above the credit floor. It is a datasheet-tier fact: presence of a row
means the part's stub carries a machine-comparable ESD rating high enough to count as protection,
absence means it does not (or that no datasheet is joined). It carries no value, only the
ref-des, because the question it answers is boolean: does this part have a creditable ESD rating.

### For hardware engineers

ESD protection on a connector-facing signal can come from a dedicated clamp or from a part that
is itself rated to survive the strike (a transceiver rated to IEC contact levels). This relation
marks the parts that carry that rating on their datasheet, so a protection review can credit a
signal whose only ESD survivability is the endpoint part's own spec. The floor is deliberately
conservative (2 kV), so a row means a genuinely rated part, not a marginal one.

### For software engineers

This is a boolean predicate over the datasheet join (see ANALOGY.md): resolve the component to
its part-number stub, read the ESD rating rows, and admit the ref-des only if a comparable row
clears the floor. It is keyed by ref-des precisely so a rule joins it against structural
relations (`component-on-net`, `component.class`, `pin.net`) to ask "an ESD-rated part sitting on
this signal". Rows are 1:1 with qualifying components; absence is the normal case, not an error.

### Go projector

`esdRatedFacts` in `check/facts.go` walks `Model.Components()`, looks up each part's spec via
`Model.PartSpec(ref)`, and emits a row when `esdRatingLimits(spec)` returns at least one
qualifying rating. `esdRatingLimits` is the same extractor the `esd-protection` Go rule uses: it
keeps only absolute-max ESD rows that are machine-comparable and at or above `icEsdFloorVolts`
(2 kV). The citation on the row is the datasheet ESD row, the real evidence, not the component's
schematic site.

This is a datasheet-tier relation, so it is silent by construction without seeded parameters:
`PartSpec` is nil for every ref when the model was built without a params set, and the relation
is empty. Run `agni` with `--params <dir>` to seed the datasheet corpus. Empty is skip, never a
false pass: no row means "no evidence of a rated part", not "the part is unprotected".

### Datalog

Every component carrying a creditable datasheet ESD rating:

```
component.esd_rated(?r) => ?r
```

The ESD-rated parts that are also TVS clamps (join the datasheet fact to a class family tag):

```
component.esd_rated(?r), component.class(?r, "tvs") => ?r
```

## component.device_class

### What it is

`component.device_class(ref_des, class)` yields one row per component whose joined datasheet spec
declares a device class, carrying that class as its datasheet states it ("efuse", "ldo",
"supervisor"). It is the authoritative counterpart to `component.class`: `component.class` derives
a family from the ref-des prefix plus description keywords, while `component.device_class` reports
what the part's own datasheet says it is. It is a datasheet-tier fact, so a row means a seeded spec
was joined and named a class; absence means no datasheet is joined or the spec left the field
empty.

### For hardware engineers

Some parts cannot be classified from the schematic alone. A smart high-side switch, an eFuse, and a
load switch all read as a plain IC by ref-des, and their identity lives in a description phrase
("intelligent fuse protection"), not a single token a keyword classifier can key on. The datasheet
states the class unambiguously, so this relation lets a review credit "this MPN is an eFuse" from
the spec rather than a guess. The same value also enriches the part's class set, so a rule asking
`HasClass(ref, "efuse")` answers from the datasheet without any keyword match.

### For software engineers

This is a projection over the datasheet join (see ANALOGY.md): resolve the component to its
part-number stub and read the single `device_class` string the spec declares. It is keyed by
ref-des so a rule joins it against structural relations (`component-on-net`, `pin.net`) to ask "an
eFuse sitting on the input rail". Prefer this relation over `component.class` for a datasheet-class
review item: it gates to not-applicable without a seeded set, where `component.class` (a netlist
relation) would silently pass.

**The class string is NORMALIZED** through `classify.NormalizeDeviceClass` (WS3-044), which folds a
vendor spelling onto the canonical key for every class the WS10-015 vocabulary knows, so `XTAL` and
`Crystal` both project as `crystal`. A class the vocabulary does not recognize passes through
unchanged, **including its case** — so `LDO` stays `LDO` and would not match a query for `ldo`. When
an exact-match query returns nothing, that is the first thing to check.

It previously projected verbatim, which made this relation disagree with the other consumer of the
same field: `check.enrichClassesFromParams` already normalized before merging device_class into a
component's class set, so `component.class` answered on the canonical key while this one answered on
the raw string. Anything matching an exact string across both had to know which it was talking to. A
fully canonical taxonomy is still WS10-004's job; this only removes the disagreement.

### Go projector

`componentDeviceClassFacts` in `check/facts.go` walks `Model.Components()`, looks up each part's
spec via `Model.PartSpec(ref)`, and emits a row when `spec.GetDeviceClass()` is non-empty. The
citation is the spec's source document title (device_class is a `PartSpec`-level field, so there is
no per-parameter provenance to cite). The same datasheet class is merged into the component's
device_classes set at model-build time (`NewModelWithParams`), so `component.class`, `HasClass`,
and this relation all agree.

This is a datasheet-tier relation, so it is silent by construction without seeded parameters:
`PartSpec` is nil for every ref when the model was built without a params set, and the relation is
empty. Run `agni` with `--params <dir>` to seed the datasheet corpus. Empty is skip, never a false
pass: no row means "no datasheet class evidence", not "the part is not an eFuse".

### Datalog

Every component the datasheet classifies as an eFuse:

```
component.device_class(?r, "efuse") => ?r
```

The datasheet-classified eFuses sitting on a rail (join the datasheet class to structure):

```
component.device_class(?r, "efuse"), component-on-net(?r, ?n), rail(?n) => ?r
```

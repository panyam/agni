## component.device_class

### What it is

`component.device_class(ref_des, class)` yields one row per component whose joined datasheet spec
declares a device class, carrying that class as its datasheet states it ("efuse", "ldo",
"supervisor"). It is the authoritative counterpart to `component.class`. `component.class` derives
a family from the ref-des prefix plus description keywords, and `component.device_class` reports
what the part's own datasheet says it is. It is a datasheet-tier fact, so a row means a seeded spec
was joined and named a class; absence means no datasheet is joined or the spec left the field
empty.

### For hardware engineers

Some parts cannot be classified from the schematic alone. A smart high-side switch, an eFuse, and a
load switch all read as a plain IC by ref-des. Their identity is in a description phrase
("intelligent fuse protection"), not in a single token that a keyword classifier can key on. The
datasheet states the class unambiguously, so this relation lets a review credit "this MPN is an
eFuse" from the spec rather than a guess. The same value also enriches the part's class set, so a
rule asking `HasClass(ref, "efuse")` answers from the datasheet without any keyword match.

### For software engineers

This is a projection over the datasheet join (see [the analogy
guide](../../../../docsite/content/reference/analogy.md)). It resolves the component to its
part-number stub and reads the single `device_class` string that the spec declares. It is keyed by
ref-des so a rule joins it against structural relations (`component.net`, `pin.net`) to ask "an
eFuse sitting on the input rail". Prefer this relation over `component.class` for a datasheet-class
review item, because it gates to not-applicable without a seeded set. `component.class` is a
netlist relation, and it would silently pass.

**The class string is NORMALIZED** through `classify.NormalizeDeviceClass` (WS3-044), which folds a
vendor spelling onto the canonical key for every class the WS10-015 vocabulary knows, so `XTAL` and
`Crystal` both project as `crystal`. A class the vocabulary does not recognize passes through
unchanged, **including its case**, so `LDO` projects as `LDO` and would not match a query for
`ldo`. When an exact-match query returns nothing, check the case first.

Normalizing here keeps this relation and `component.class` answering on the same key, since
`classify.StampClassesFromSpecs` normalizes too. A fully canonical taxonomy is still WS10-004's job.

### Go projector

`componentDeviceClassFacts` in `stdlib/relations/facts.go` walks `Model.Components()`, looks up each
part's spec via `Model.PartSpec(ref)`, and emits a row when `spec.GetDeviceClass()` is non-empty.
The citation is the spec's source document title (device_class is a `PartSpec`-level field, so there
is no per-parameter provenance to cite). When the read carries a datasheet corpus, the Loader merges
the same datasheet class into the component's device_classes set. Otherwise it is merged at
model-build time (`check.WithParamProvider`). So `component.class`, `HasClass`, and this relation all
agree.

This is a datasheet-tier relation, so it is silent by construction without seeded parameters. When
the model was built without a params set, `PartSpec` is nil for every ref, and the relation is
empty. Run `agni` with `--params <dir>` to seed the datasheet corpus. Empty is skip, never a false
pass, since no row means "no datasheet class evidence", not "the part is not an eFuse".

### Datalog

Every component the datasheet classifies as an eFuse:

```
component.device_class(?r, "efuse") => ?r
```

The datasheet-classified eFuses sitting on a rail (join the datasheet class to structure):

```
component.device_class(?r, "efuse"), component.net(?r, ?n), net.rail(?n) => ?r
```

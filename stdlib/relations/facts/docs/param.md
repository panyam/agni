## param

### What it is

`param(mpn, symbol, max)` yields one row per parameter of a datasheet spec that joined to a
part in the design, keyed by manufacturer part number (`mpn`) and the parameter's datasheet
symbol (e.g. `VDD`, `VIN`, `RDS(on)`). The third argument is the parameter's numeric maximum, **in
that parameter's SI base unit** (volts, amps, ohms) whatever the vendor printed, so a threshold
written against it means the same thing across vendors. Join `param.unit` for the printed spelling.
Each row also carries the rendered value range and its test conditions, plus a citation back to
the datasheet page and table. One MPN can be shared by several components, so the projector
dedupes by MPN and emits each parameter once. A parameter whose unit has no known scale is omitted
here and appears only in `param.unit` (agni issue 165).

This is the datasheet tier of the query surface. It is EMPTY unless `agni` is run with
`--params <dir>` pointing at a seeded `PartSpec` corpus. That is skip-not-false-pass by
construction: with no datasheet corpus loaded, the relation yields zero rows and every rule that
reads it reports not-applicable rather than a false pass.

### For hardware engineers

A row is one line off a part's datasheet: an absolute-maximum, a recommended-operating limit, a
rated value, under the test conditions the datasheet stated. The `max` value is presented, not
silently compared. A parameter whose conditions survive only as free text (not structured as
`eq`/`min`/`max`) is not machine-comparable, so the tool shows it to you beside its citation
rather than auto-checking a design value against it. You query `param` during a review to see
what the datasheet actually claims for a part before trusting a rule that leans on it, or to spot
which parts have no extracted spec at all (they simply do not appear).

### For software engineers

A `PartSpec` is the `.d.ts` type stub for a closed-source dependency: machine-readable claims
about a part you cannot see inside, each written against one pinned doc revision and linking back
to the prose it came from. `param` projects those claims into query rows. The design-side
identity is `component.mpn(ref_des, mpn)`; `param` is keyed by the same `mpn`, so the two relations
join on it. That join is the whole point of the tier: it is how a rule crosses from "this net in
the design drives R1's VIN" to "R1's MPN has an absolute-maximum VIN of 20 V". Rows are 1:many
with a part (one per parameter). Absent means the datasheet corpus was not loaded, or the part
has no spec in it, never that the part is fine.

An empty join is not a clean result. `component.mpn ⋈ param` is an inner join, so a component
whose part has no seeded row for the symbol is silently dropped, and the query returns zero rows
for the same reason a genuinely-clean design does. Reading that as pass is the SQL bug of treating
a `LEFT JOIN` with a NULL right side as a validated row. A datasheet-backed review item therefore
checks whether the symbol is seeded at all and reports `needs-data` (closer to HTTP 424 Failed
Dependency than 404: the check exists, its input is missing) rather than pass. `needs-data` still
counts as covered, because the mechanism is wired and only the value is absent, which is what lets
an overlay bind a datasheet check before its seed lands and watch it flip to a real verdict once
the value arrives (WS3-097). Whether the corpus holds one seeded part or a thousand is only how
many rows the table has: the relation is the union of every seeded spec, keyed by MPN, so more
seeds mean fewer `needs-data` items, never a different answer.

### Go projector

`paramFacts` in `check/facts.go` iterates `Model.Components()`, reads each component's MPN via
`Model.ComponentMPN` and its spec via `Model.PartSpec`, dedupes by MPN, and emits one row per
`spec.Parameters` entry. `Value` is rendered by `rangeText`, `Conditions` by `conditionsText`,
and `Cite` by `citation` (document title, page, table or figure, extraction method, confidence);
`Num` is set to the parameter's `Value.Max` when present, which is the `max` datalog argument.
One row per parameter of each joined part. Empty without `--params`, because the MPN-to-spec map
is built only when a param set is loaded, so a design with no matching specs yields nothing.

### Datalog

Both queries need `--params`. List every extracted parameter for each part, as MPN and symbol:

```
param(?mpn, ?sym, ?max) => ?mpn
```

Join to the design-side part identity to name the components a datasheet parameter applies to
(the bridge a rule like `supply-exceeds-abs-max` walks to compare a design rail against a
datasheet absolute-maximum):

```
component.mpn(?ref, ?mpn), param(?mpn, ?sym, ?max) => ?ref
```

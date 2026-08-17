---
title: "param.pin_range"
description: "a datasheet limit bound to ONE pin, both bounds in the SI base unit, the per-terminal counterpart to param.range, so a part with several supply pins answers per pin instead of once (needs --params)"
---

### What it is

`param.pin_range(mpn, pin, symbol, kind, min, max)` is `param.range` with a pin column: one row per
(parameter, bound pin) pair, so a limit the datasheet states for a specific terminal is queryable
against that terminal rather than against the part as a whole.

`kind` is the same token `param.range` uses (`absolute_max`, `recommended_operating`,
`characteristic`, `unspecified`), and both bounds are in the parameter's SI base unit whatever the
vendor printed. A bound the datasheet did not state is absent, and `absent(?min)` selects those rows.
Every row carries a citation back to the page and table.

A parameter bound to several pins emits **one row per pin**. That is the point: a part may state a
single output range for a whole port of four terminals, and a query asking about one of them has to
find it without knowing it was written as a group.

**Part-wide rows are deliberately absent.** A parameter with no pin binding is a fact about the die
(a junction-temperature rating, a storage range), and emitting it against every pin would read as
each terminal carrying that limit itself, re-creating in a new place the collapse this relation
exists to undo. Those rows are on `param.range`, which is where a query that wants them should look.
A part with no pin bindings therefore answers nothing here, and a spec seeded before pin binding
answers nothing at all.

### For hardware engineers

`param.range` can tell you a part's supply limits. It cannot tell you *which supply*, and plenty of
parts have more than one at different voltages: a level translator sits between a 1.8 V world and a
5 V world and has a separate supply pin for each, with genuinely different windows. Asked through
`param.range`, that part has one blurred "supply" answer. Asked through `param.pin_range`, it has
two correct ones.

This is what makes "does this connection meet what this pin actually requires" answerable. The limit
you compare a rail against is the limit of the terminal the rail lands on, not an average over the
part.

The usual cautions still apply and are not weakened by the pin column. A row whose conditions survive
only as free text is not machine-comparable, and it is surfaced beside its citation rather than
compared. A row whose unit has no known scale keeps its pin, symbol, kind and citation with both
bounds absent, because an unmeasurable value must not become orderable.

### For software engineers

`param.range` is keyed by `(mpn, symbol)`; this is keyed by `(mpn, pin, symbol)`. It is a
denormalized fact table, not a view to be reconstructed by joining `param.range` to a binding
relation: two parameters can share a symbol and bind to different pins (a part stating one ESD rating
for its A-port terminals and a different one for its B port publishes both under `V(ESD)`), so a join
on symbol alone would cross-product them onto the wrong terminals.

This is the widest relation in the fact base and the reason `FactRow` grew a `Qualifier` slot:
`mpn`, `pin` and `symbol` consume Subject, Object and Value, leaving the limit kind nowhere to go.
The conditions slot was not reused for it, because every param relation carries test conditions there
as unbound metadata and spending it would strip the trust context from exactly the rows most likely
to be compared against.

### Go projector

`paramPinRangeFacts` in `stdlib/relations/facts.go` shares the per-MPN join and dedup of the other
param projectors and emits through `specParamPinRangeRows`, which skips parameters with an empty
`pin_refs` and otherwise emits one row per referenced pin. `Value` is the symbol, `Qualifier` the
kind token, `Min` and `Num` the reduced bounds (each nil when the datasheet omitted that end), and
`Cite` the parameter's datasheet provenance. Empty without `--params`.

### Datalog

All queries need `--params`. Every per-pin recommended-operating window in the corpus:

```
param.pin_range(?mpn, ?pin, ?sym, "recommended_operating", ?min, ?max) => ?mpn, ?pin, ?sym, ?min, ?max
```

Join to `param.pin` so the answer names the terminal the way the datasheet prints it:

```
param.pin_range(?mpn, ?pin, ?sym, "recommended_operating", ?min, ?max),
param.pin(?mpn, ?pin, ?name, ?fn) => ?mpn, ?name, ?fn, ?min, ?max
```

Find the parts this relation exists for: those stating more than one distinct supply window across
their terminals, which `param.range` reports as a single blurred answer.

```
param.pin_range(?mpn, ?a, ?s1, "recommended_operating", ?min1, ?max1),
param.pin_range(?mpn, ?b, ?s2, "recommended_operating", ?min2, ?max2),
?a != ?b, ?max1 != ?max2 => ?mpn, ?a, ?max1, ?b, ?max2
```

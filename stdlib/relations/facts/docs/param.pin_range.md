## param.pin_range

### What it is

`param.pin_range(mpn, pin, symbol, kind, min, max)` is `param.range` with a pin column. It emits one
row per (parameter, bound pin) pair. When a datasheet states a limit for a specific terminal, a query
can then ask about that terminal rather than about the part as a whole.

`kind` is the same token `param.range` uses (`absolute_max`, `recommended_operating`,
`characteristic`, `unspecified`), and both bounds are in the parameter's SI base unit whatever the
vendor printed. A bound the datasheet did not state is absent, and `absent(?min)` selects those rows.
Every row carries a citation back to the page and table.

A parameter bound to several pins emits **one row per pin**. A part may state a
single output range for a whole port of four terminals, and a query asking about one of them has to
find it without knowing it was written as a group.

**Part-wide rows are deliberately absent.** A parameter with no pin binding is a fact about the
die, such as a junction-temperature rating or a storage range. Emitting it against every pin would
read as each terminal carrying that limit itself. That would re-create the collapse this relation
exists to undo, in a new place. Those rows are on `param.range`, and a query that wants them should
look there. A part with no pin bindings therefore answers nothing here, and a spec seeded before pin
binding answers nothing at all.

### For hardware engineers

`param.range` can tell you a part's supply limits. It cannot tell you *which supply*, and plenty of
parts have more than one at different voltages, such as a level translator sitting between a 1.8 V
world and a 5 V world with a separate supply pin for each and genuinely different windows. Asked through
`param.range`, that part has one blurred "supply" answer. Asked through `param.pin_range`, it has
two correct ones.

With the pin column, "does this connection meet what this pin actually requires" has an answer. You
compare a rail against the limit of the terminal it connects to, not an average over the part.

The usual cautions still apply and are not weakened by the pin column. A row whose conditions survive
only as free text is not machine-comparable, and it is surfaced beside its citation rather than
compared. A row whose unit has no known scale keeps its pin, symbol, kind and citation with both
bounds absent, because an unmeasurable value must not become orderable.

### For software engineers

`param.range` is keyed by `(mpn, symbol)`; this is keyed by `(mpn, pin, symbol)`. It is a
denormalized fact table, not a view to be reconstructed by joining `param.range` to a binding
relation. Two parameters can share a symbol and bind to different pins. A part stating one ESD
rating for its A-port terminals and a different one for its B port publishes both under `V(ESD)`.
A join on symbol alone would cross-product them onto the wrong terminals.

This was the first six-column relation in the fact base. `facts.Row` grew a `Qualifier` slot for
it, because `mpn`, `pin` and `symbol` consume Subject, Object and Value and leave the limit kind
nowhere to go. The conditions slot was not reused for the kind. Every param relation carries test
conditions there as unbound metadata. Spending that slot would strip the trust context from exactly
the rows a query is most likely to compare.

### Go projector

`paramPinRangeFacts` in `stdlib/relations/facts.go` shares the per-MPN join and dedup of the other
param projectors and emits through `specParamPinRangeRows`, which skips parameters with an empty
`pin_refs` and otherwise emits one row per referenced pin. `Value` is the symbol, `Qualifier` the
kind token, `Min` and `Num` the reduced bounds (each nil when the datasheet omitted that end), and
`Cites` the parameter's datasheet provenance. Empty without `--params`.

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

Find the parts that state more than one distinct supply window across
their terminals. `param.range` reports each of them as a single blurred answer.

```
param.pin_range(?mpn, ?a, ?s1, "recommended_operating", ?min1, ?max1),
param.pin_range(?mpn, ?b, ?s2, "recommended_operating", ?min2, ?max2),
?a != ?b, ?max1 != ?max2 => ?mpn, ?a, ?max1, ?b, ?max2
```

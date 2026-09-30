## unresolved_symbol

### What it is

`unresolved_symbol(ref_des, symref)` yields one row per PLACEMENT whose symbol the reader could not
open or parse (WS1-052). `ref_des` is the affected component's reference designator; `symref` is the
reference exactly as the source spelled it (an xschem or gEDA `res.sym`, a KiCad `Library:Symbol`
lib_id). One missing file usually produces several rows, one per part drawn with it.

Keyed by `ref_des` rather than by `symref`, because a ref-des is what every other netlist relation
joins on. The interesting question is rarely "which file is missing" but "what did it cost me", and
only a ref-des key can answer that.

An empty result means every symbol resolved. It does not mean the design has no external symbol
references.

### For hardware engineers

A symbol file carries a part's pins. When the reader cannot find it, the part still appears on the
sheet with its designator, but it has no terminals, so it joins no nets. The drawing shows the part
wired up; the netlist behind it does not.

You query this when a check comes back clean and you want to know whether it was clean because the
design is right or because the reader never saw half the connections. A row means some part's
connectivity is missing from the read, so any conclusion about that part is unsupported. The usual
cause is a library that was not on the search path, not a mistake on the schematic.

### For software engineers

A resolution failure the front end recorded instead of swallowing. The parse succeeded; a
transitive dependency of the parse did not, and the result is a smaller graph rather than an error.
That is the dangerous shape: the failure removes edges instead of raising, so every downstream
query returns a confident answer over incomplete input.

It is a projection over `Model.UnresolvedSymbols()`, flattened from one entry per reference to one
row per affected placement so it joins on `ref_des`. Note the asymmetry with the rule of the same
name, which reports one finding per REFERENCE (one cause, one finding); the relation is per
placement (one row per victim) because that is the join granularity.

### Go projector

`unresolvedSymbolFacts` in `stdlib/relations/facts.go` iterates `Model.UnresolvedSymbols()` (the
reader-emitted `ir.UnresolvedSymbol` list off `InputDiagnostics`) and emits one row per entry in
each record's `ref_des` list, with the reference's provenance as the citation. Every symbol-resolving
reader populates the same channel, so the relation is format-neutral.

### Datalog

List every part that lost its pins, and to what:

```
unresolved_symbol(?ref, ?sym) => ?ref, ?sym
```

The blast-radius query, and the reason this relation is keyed by ref-des: did anything that MATTERS
lose its pins?

```
unresolved_symbol(?ref, ?sym), component.class(?ref, "fpga") => ?ref, ?sym
```

Everything affected by one specific missing library:

```
unresolved_symbol(?ref, "Device:R") => ?ref
```

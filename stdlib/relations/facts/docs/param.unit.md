## param.unit

### What it is

`param.unit(mpn, symbol, unit)` yields one row per parameter of a datasheet spec that joined to a
part in the design, carrying the unit the vendor PRINTED that parameter in: `"mV"`, `"mA"`, `"Ohm"`,
`"kV"`. It is the companion to `param` and `param.range`, whose numbers are reduced to SI base units
so that a comparison can trust them. This relation is where the vendor's own spelling survives.

This is the datasheet tier of the query surface. It is EMPTY unless `agni` is run with
`--params <dir>` pointing at a seeded `PartSpec` corpus, so it is skip-not-false-pass by construction.

### For hardware engineers

A controller states its overcurrent threshold in millivolts. A sub-amp regulator states its output
current in milliamps. A modern FET states RDS(on) in milliohms. Those are the ordinary spellings, not
unusual ones, and a seeded spec records them exactly as the sheet prints them so a reviewer can check
a row against the page it came from.

What a QUERY sees is different, and deliberately so. `param` and `param.range` hand you a number in
the base unit: volts, amps, ohms. A threshold written against them is therefore written once, in the
unit the physics is in, and it does not change meaning because one vendor printed millivolts and
another printed volts. When you want to know what was actually printed, join this relation.

### For software engineers

`FactRow` has no unit column, so before agni issue 165 a datalog rule comparing
`param.range(?m,"VDD",_,_,?max), ?max < 5.0` was comparing a bare number with no way to know its
scale, and no gate refused it. A spec seeded 4600 mV compared as 4600 against a 5.0 threshold and read
as comfortably within limits. That is the same wrong-pass agni issue 148 fixed on the Go rule path,
on a surface where there was not even a unit string to gate on.

The fix normalizes the NUMBERS rather than asking a rule to remember a unit column, because an
optional column is advisory and a normalized number is structural (CONSTRAINTS C24). `param.unit`
exists so normalizing loses nothing: the printed spelling is still queryable, it is just no longer
something a numeric comparison can silently get wrong.

**A parameter whose unit the conversion table does not recognize still appears in `param` and
`param.range`, with its NUMBER absent.** It keeps its symbol, kind, conditions and citation, because a
relation that answers "what does this part specify" must not shorten its list silently. Only the
number goes, since an unknown scale means there is no number to publish.

That is safe because ordering was fixed in the same change: `evalCompare` refuses to ORDER an absent
number against a present one, so an unmeasurable value is unorderable by construction. Before that, an
absent numeric bound the variable to the empty string and the comparison fell back to lexicography
(`"" < "5.0"` is true), so a row with no number satisfied every upper-bound guard. The same fix is
what makes `param.range` safe to emit with one bound absent, which it does on any ordinary max-only
datasheet row.

Select them with the `absent` predicate, which asks whether a field carried a value at all:

```
param.unit(?mpn, ?sym, ?unit), param(?mpn, ?sym, ?max), absent(?max) => ?mpn, ?sym, ?unit
```

`not param.range(...)` does NOT find them, and the distinction is the useful one: the row exists, so
negation (which asks whether a ROW exists) sees it. Only its number is missing, and that is what
`absent` asks about.

### Go projector

`paramUnitFacts` in `stdlib/relations/facts.go` iterates `Model.Components()`, reads each component's
MPN via `Model.ComponentMPN` and its spec via `Model.PartSpec`, dedupes by MPN, and emits one row per
`spec.Parameters` entry with `Value` set to the parameter's printed `unit`. It shares the join and
dedup shape of `paramFacts`, and unlike that projector it filters nothing. Empty without `--params`.

### Datalog

Needs `--params`. List every parameter's printed unit:

```
param.unit(?mpn, ?sym, ?unit) => ?mpn, ?sym, ?unit
```

Show a value next to the unit it was printed in, remembering that `?max` is in the BASE unit and
`?unit` is what the vendor wrote (so an 800 mA row reads as `0.8` next to `mA`):

```
param(?mpn, ?sym, ?max), param.unit(?mpn, ?sym, ?unit) => ?mpn, ?sym, ?max, ?unit
```


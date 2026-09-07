## param.typ

### What it is

`param.typ(mpn, symbol, typ)` yields one row per parameter of a datasheet spec that states a
TYPICAL value, carrying that value in the SI base unit. It is the third member of the min/typ/max
triple, and the only one `param.range(mpn, symbol, kind, min, max)` does not carry.

A parameter stating no typ emits no row here, so absence stays absence rather than arriving as a
zero. Like every relation in this tier it is EMPTY unless `agni` runs with `--params <dir>` pointing
at a seeded `PartSpec` corpus.

### For hardware engineers

An electrical characteristics table usually prints three columns. The MIN and MAX are what the vendor
guarantees across the stated conditions, and they are what a limit check compares against. The TYP is
what a part off the middle of the distribution actually does at room temperature, and it is not a
promise about the part on your bench.

That distinction is why this is a separate relation rather than another column. A typical quiescent
current tells you what the board will draw in the usual case, which is exactly what you want when
sizing a battery or estimating thermals. It is the wrong number to design a protection threshold
against, because a part at the edge of the distribution is still in spec and still ruins the
calculation. Reach for `param.range` when you need a guarantee, and for this when you need an
expectation.

### For software engineers

`facts.Row` carries two numeric slots, `Num` and `Min`, and `param.range` spends both on the two
bounds. So a typ had nowhere to land and was dropped, surviving only inside a rendered string where
nothing could bind or compare it. Nothing documented that it was gone (agni issue 545).

The obvious repair is a third numeric slot on the row, and it is the wrong one. The tuple is shared
by 44 graph-tier relations that need nothing of the sort, and the datasheet record has thirteen
fields and four repeated parts, so no arity fixes it. The relational answer to a wide record is a
KEY rather than a wide tuple: several narrow relations sharing `(mpn, symbol)`, joined when a query
wants more than one of them. `param.unit` was already that shape, adopted under CONSTRAINTS C24 for a
safety reason rather than to conserve a slot. See `DECISIONS.md`, "The datasheet tier is normalized
into narrow relations, never flattened into a wider tuple".

Separating it is also the safer shape on its own terms. A typ sitting between `min` and `max` in one
tuple reads as a bound, and a rule comparing a rail against it would look perfectly ordinary while
reporting a confident wrong answer. Its own name puts that choice at the call site.

The number is reduced to its SI base unit by `param.InBaseUnit`, the same conversion `param` and
`param.range` go through, and the row carries `BaseUnit` so an ordering comparison can refuse amps
against volts. A parameter whose printed unit has no known scale yields a row with no number, so its
symbol and citation survive.

### Go projector

`paramTypFacts` in `stdlib/relations/facts.go` goes through `perJoinedSpec`, which iterates
`Model.Components()`, reads each component's MPN via `Model.ComponentMPN` and its spec via
`Model.PartSpec`, and dedupes by MPN. `specParamTypRows` does the per-spec work and is shared with
`SpecLibFacts`, so `agni query --speclib` answers the same relation with no design loaded. One row
per parameter that states a typ, none for a parameter that does not, and empty without `--params`.

### Datalog

Needs `--params`. Every typical value in the joined parts:

```
param.typ(?mpn, ?sym, ?typ) => ?mpn, ?sym, ?typ
```

The typ beside the window it sits inside, which is the join that shows what the relation is for. A
part whose typical value sits near one edge of its own guaranteed range is worth a second look:

```
param.typ(?mpn, ?sym, ?typ), param.range(?mpn, ?sym, "recommended_operating", ?min, ?max)
  => ?mpn, ?sym, ?min, ?typ, ?max
```

Parts stating a typ the conversion table could not scale, so the row exists with no number:

```
param.typ(?mpn, ?sym, ?typ), absent(?typ) => ?mpn, ?sym
```

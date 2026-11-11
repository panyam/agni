## absent

### What it is

`absent(value)` is true when `value` carries no value at all. Absent differs from the empty string
and from zero. A relation binds an absent value when its source did not state a field. `param.range`
does this for a bound the datasheet left out, and `num.unit` does it for a number without a unit.

`absent` is a computed predicate (kind `predicate` in the catalog), not a stored fact. It is a test,
so its argument must be bound before it runs. Calling it with an unbound variable is an evaluation
error. Its negation is as useful as the test itself. `not absent(?min)` reads "this row states a
lower bound".

### For hardware engineers

A datasheet often states only one side of a limit. An absolute maximum table gives a ceiling and no
floor, and a recommended output voltage may give neither. `absent` picks out those rows, so a review
can see which limits a check has nothing to compare against.

### For software engineers

`absent(?x)` is `?x IS NULL` in SQL, or a check for a missing key rather than a zero value in Go. A
field the source left out never binds to `""`, so `absent` is the only way to ask about it. An absent
value cannot take part in an ordering comparison, so `?min < 5` holds for no absent row.

### Go projector

`absent` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The Datalog
engine's standard library registers it, together with `coalesce`, in `Absent` in jaala's
`stdlib/absent.go`. `core/facts/registry.go` installs that library into the vocabulary every query
uses.

### Datalog

The datasheet limits that state no lower bound:

```
param.range(?mpn, ?s, ?k, ?min, ?max), absent(?min) => ?mpn, ?s, ?k, ?max
```

The limits that state both bounds:

```
param.range(?mpn, ?s, ?k, ?min, ?max), not absent(?min), not absent(?max) => ?mpn, ?s, ?min, ?max
```

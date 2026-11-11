## coalesce

### What it is

`coalesce(value, fallback, result)` binds `result` to `value`, or to `fallback` when `value` is
absent.

`coalesce` comes from jaala's standard library. The
[coalesce](https://panyam.github.io/jaala/reference/builtins/#coalesce) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A datasheet range with no stated floor still needs a floor for some checks. A rail input has a
minimum of zero volts in practice, and a check can say so once with `coalesce(?min, 0V, ?lo)`. The
row then compares like every other row, and the stated minimum wins wherever the datasheet gives
one.

### For software engineers

`coalesce` is SQL's `COALESCE(a, b)` with exactly two inputs, written as a relation with its result
as the third argument.

### Datalog

Every datasheet limit with its lower bound, reading zero where the datasheet states none:

```
param.range(?mpn, ?s, ?k, ?min, ?max), coalesce(?min, 0, ?lo) => ?mpn, ?s, ?k, ?lo, ?max
```

The limits that state neither bound. The upper bound falls back to the lower, and the result is
absent only when both are:

```
param.range(?mpn, ?s, ?k, ?min, ?max), coalesce(?max, ?min, ?b), absent(?b) => ?mpn, ?s, ?k
```

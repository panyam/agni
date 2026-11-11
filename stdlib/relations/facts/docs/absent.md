## absent

### What it is

`absent(value)` is true when `value` carries no value at all.

In agni, `param.range` binds an absent value for a bound the datasheet left out.

`absent` comes from jaala's standard library. The
[absent](https://panyam.github.io/jaala/reference/builtins/#absent) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A datasheet often states only one side of a limit. An absolute maximum table gives a ceiling and no
floor, and a recommended output voltage may give neither. `absent` picks out those rows, so a review
can see which limits a check has nothing to compare against.

### For software engineers

`absent(?x)` is `?x IS NULL` in SQL, or a check for a missing key rather than a zero value in Go.

### Datalog

The datasheet limits that state no lower bound:

```
param.range(?mpn, ?s, ?k, ?min, ?max), absent(?min) => ?mpn, ?s, ?k, ?max
```

The limits that state both bounds:

```
param.range(?mpn, ?s, ?k, ?min, ?max), not absent(?min), not absent(?max) => ?mpn, ?s, ?min, ?max
```

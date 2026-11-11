---
title: "str.find"
description: "binds each non-empty match of an unanchored regular expression with its index from 0; given an index, binds that match alone, a negative index counting from the end"
---

### What it is

`str.find(string, regex, index, match)` binds each match of a regular expression in `string`,
numbered from 0. The expression is unanchored, so it matches anywhere in the string. Matches do not
overlap, and an empty match is dropped before the numbering.

With `index` bound it binds that match alone. A negative index counts from the end, so -1 is the
last match. An index past either end gives no row. `string` and `regex` must be bound, and a
malformed expression is an error rather than no match. `str.find` is a computed predicate (kind
`predicate` in the catalog), not a stored fact.

### For hardware engineers

A net name carries numbers in several places: a bus instance, a lane, a bit. `str.find` reads them
one at a time, which `str.split` cannot do when the separators vary. A regex reads every run of
digits, including the 2 inside `I2C_SCL`, so write the expression to match only the part you mean.

### For software engineers

It is Go's `regexp.FindAllString` with the empty matches removed and each match numbered, plus
negative indexing from the end, as in Python.

### Go projector

`str.find` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it in `Strings` in jaala's `stdlib/strings.go`, and
`core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

The first run of digits in each net name:

```
entity(?n, "net"), str.find(?n, "[0-9]+", 0, ?m) => ?n, ?m
```

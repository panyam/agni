---
title: "str.distance"
description: "binds the edit (Levenshtein) distance between two strings, in characters; both strings must be bound"
---

### What it is

`str.distance(string, other, distance)` binds the edit distance between two strings. That is the
fewest single-character insertions, deletions and substitutions that turn one into the other, the
Levenshtein distance. It counts characters rather than bytes, so a non-ASCII letter is one edit.

Both strings must be bound. Listing every string within a distance of another has no finite answer,
so a query that leaves either unbound is an error naming what it needs. With `distance` bound as
well, the predicate tests rather than binds. `str.distance` is a computed predicate (kind
`predicate` in the catalog), not a stored fact.

### For hardware engineers

Two nets whose names differ by one character are often one net misspelled, such as `SPI_MISO` and
`SPI_MIS0`. A schematic tool reads them as two nets and leaves both half-connected. A distance of 1
between two names on one board is worth a look, although `CANH` and `CANL` show that it is often
correct.

### For software engineers

It is the textbook Levenshtein distance over runes, run on one row of the table. Put a bound on it
with a comparison, as `?d <= 1`, the way a fuzzy search does.

### Go projector

`str.distance` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it in `Strings` in jaala's `stdlib/strings.go`, and
`core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

Every pair of nets whose names are one edit apart, each pair once:

```
entity(?a, "net"), entity(?b, "net"), ?a < ?b, str.distance(?a, ?b, ?d), ?d <= 1 => ?a, ?b, ?d
```

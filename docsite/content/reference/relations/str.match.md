---
title: "str.match"
description: "reports whether the string matches an unanchored regular expression"
---

### What it is

`str.match(string, regex)` is true when the regular expression matches anywhere in `string`. The
expression is unanchored, so write `^` and `$` to pin it to the start or end. The syntax is Go's
RE2, which has no backreferences.

Both arguments must be bound. A malformed expression is an error rather than a non-match, so a
typo in a completeness check cannot read as "the data is clean". `str.match` is a computed
predicate (kind `predicate` in the catalog), not a stored fact.

### For hardware engineers

A house naming rule often has more structure than a prefix: a letter block, an instance number, an
underscore. A regular expression states it exactly, and so does the opposite, the names that break
it.

### For software engineers

It is Go's `regexp.MatchString`, compiled once per pattern. Compared with `str.glob` it is
unanchored and far more expressive.

### Go projector

`str.match` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it in `Strings` in jaala's `stdlib/strings.go`, and
`core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

Every net whose name opens with letters, a digit and an underscore:

```
entity(?n, "net"), str.match(?n, "^[A-Z]+[0-9]_") => ?n
```

---
title: "str.suffix"
description: "reports whether a string ends with a suffix"
---

### What it is

`str.suffix(string, suffix)` is true when `string` ends with `suffix`. The first argument is the
text under test, and the second is the ending to look for. The order matters. `str.suffix(?n,
"_CS")` asks whether a net name ends in `_CS`. `str.suffix("_CS", ?n)` asks the reverse.

The match is exact and case-sensitive, so `"_cs"` does not match `FLASH_CS`. A string counts as its
own suffix, and an empty suffix matches every string. `str.suffix` is a computed predicate (kind
`predicate` in the catalog), not a stored fact. It is a filter, so both arguments must be bound
before it runs.

### For hardware engineers

A signal's role often sits at the end of its name. `_CS` marks a chip select, `_N` or `_B` an
active-low signal, and `_P` and `_N` the halves of a differential pair. A suffix test selects every
net in that role, and a negated test finds the ones missing something the role needs.

### For software engineers

`str.suffix` is `strings.HasSuffix` over two bound strings, exposed as a two-argument filter. It is
the `LIKE '%x'` of the query language, without wildcards and without case folding. The profile
matchers in `stdlib/profiles` build their name tests from it and from `str.prefix`.

### Go projector

`str.suffix` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it, together with `str.contains` and `str.prefix`, in
`Strings` in jaala's `stdlib/strings.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses. Calling it with an unbound argument is an evaluation error rather than
an empty answer.

### Datalog

Every chip-select net, with the parts on it:

```
component.net(?r, ?n), str.suffix(?n, "_CS") => ?n, ?r
```

The chip-select nets that are not rails:

```
component.net(?r, ?n), str.suffix(?n, "_CS"), not net.rail(?n) => ?n
```

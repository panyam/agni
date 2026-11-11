---
title: "str.match"
description: "reports whether the string matches an unanchored regular expression"
---

### What it is

`str.match(string, regex)` is true when the regular expression matches anywhere in `string`.

`str.match` comes from jaala's standard library. The [String
tests](https://panyam.github.io/jaala/reference/builtins/#string-tests) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A house naming rule often has more structure than a prefix: a letter block, an instance number, an
underscore. A regular expression states it exactly, and so does the opposite, the names that break
it.

### For software engineers

It is Go's `regexp.MatchString`, compiled once per pattern.

### Datalog

Every net whose name opens with letters, a digit and an underscore:

```
entity(?n, "net"), str.match(?n, "^[A-Z]+[0-9]_") => ?n
```

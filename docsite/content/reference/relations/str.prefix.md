---
title: "str.prefix"
description: "reports whether a string starts with a prefix"
---

### What it is

`str.prefix(string, prefix)` is true when `string` starts with `prefix`.

`str.prefix` comes from jaala's standard library. The [String
tests](https://panyam.github.io/jaala/reference/builtins/#string-tests) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A reference designator starts with its part class by convention, U for an IC, R for a resistor and J
for a connector. A rail often starts with the supply that feeds it, such as `PMIC_` or `VDD_`. A
prefix test selects that group without listing its members.

### For software engineers

`str.prefix` is `strings.HasPrefix` over two bound strings, exposed as a two-argument filter. It is
the `LIKE 'x%'` of the query language, without wildcards and without case folding. A library member
can use it to name a house group once, as `pmic_rail(?n: net) :- net.rail(?n), str.prefix(?n,
"PMIC_");` does in the tutorial project.

### Datalog

Every component whose reference designator starts with U:

```
entity(?name, "component"), str.prefix(?name, "U") => ?name
```

The rails a PMIC feeds, by the house naming convention:

```
net.rail(?n), str.prefix(?n, "PMIC_") => ?n
```

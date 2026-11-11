## str.prefix

### What it is

`str.prefix(string, prefix)` is true when `string` starts with `prefix`. The first argument is the
text under test, and the second is the start to look for. The order matters. `str.prefix(?name,
"U")` asks whether a name starts with U. `str.prefix("U", ?name)` asks whether "U" starts with the
name.

The match is exact and case-sensitive, so `"u"` does not match `U7`. A string counts as its own
prefix, and an empty prefix matches every string. `str.prefix` is a computed predicate (kind
`predicate` in the catalog), not a stored fact. It is a filter, so both arguments must be bound
before it runs.

### For hardware engineers

A reference designator starts with its part class by convention, U for an IC, R for a resistor and
J for a connector. A rail often starts with the supply that feeds it, such as `PMIC_` or `VDD_`. A
prefix test selects that group without listing its members.

### For software engineers

`str.prefix` is `strings.HasPrefix` over two bound strings, exposed as a two-argument filter. It is
the `LIKE 'x%'` of the query language, without wildcards and without case folding. A library
member can use it to name a house group once, as `pmic_rail(?n: net) :- net.rail(?n),
str.prefix(?n, "PMIC_");` does in the tutorial project.

### Go projector

`str.prefix` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it, together with `str.contains` and `str.suffix`, in
`Strings` in jaala's `stdlib/strings.go`. `core/facts/registry.go` installs that library into the
vocabulary every query uses. Calling it with an unbound argument is an evaluation error rather than
an empty answer.

### Datalog

Every component whose reference designator starts with U:

```
entity(?name, "component"), str.prefix(?name, "U") => ?name
```

The rails a PMIC feeds, by the house naming convention:

```
net.rail(?n), str.prefix(?n, "PMIC_") => ?n
```

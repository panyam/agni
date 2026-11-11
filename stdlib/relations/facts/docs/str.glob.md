## str.glob

### What it is

`str.glob(string, pattern)` is true when the whole of `string` matches a glob. `*` matches any run
of characters, `/` included. `?` matches exactly one character. `[a-z]` matches one character in a
class, and `[^a-z]` one outside it. A literal `[` is written `[[]`, so a bus label like `DATA[1:0]`
is matched as `DATA[[]1:0]`.

The match is whole-string and case-sensitive, as SQLite's `GLOB` is. Both arguments must be bound.
A malformed pattern, such as an unclosed `[`, is an error rather than a pattern that matches
nothing. `str.glob` is a computed predicate (kind `predicate` in the catalog), not a stored fact.

### For hardware engineers

A glob states a naming convention the way it is usually written down. `CAN?_*` reads as "CAN, one
character for the instance, an underscore, then anything", which selects every CAN bus's nets
without listing them.

### For software engineers

It is a shell-style glob compiled to an anchored regular expression. Use it for a whole-name shape,
and use `str.match` when the test is about part of a name.

### Go projector

`str.glob` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it in `Strings` in jaala's `stdlib/strings.go`, and
`core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

Every net of every CAN instance:

```
entity(?n, "net"), str.glob(?n, "CAN?_*") => ?n
```

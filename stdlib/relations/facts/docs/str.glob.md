## str.glob

### What it is

`str.glob(string, pattern)` is true when the whole of `string` matches a glob.

`str.glob` comes from jaala's standard library. The [String
tests](https://panyam.github.io/jaala/reference/builtins/#string-tests) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A glob states a naming convention the way it is usually written down. `CAN?_*` reads as "CAN, one
character for the instance, an underscore, then anything", which selects every CAN bus's nets
without listing them.

### For software engineers

It is a shell-style glob. Use it for a whole-name shape, and use `str.match` when the test is about
part of a name.

### Datalog

Every net of every CAN instance:

```
entity(?n, "net"), str.glob(?n, "CAN?_*") => ?n
```

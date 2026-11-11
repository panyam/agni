## str.suffix

### What it is

`str.suffix(string, suffix)` is true when `string` ends with `suffix`.

`str.suffix` comes from jaala's standard library. The [String
tests](https://panyam.github.io/jaala/reference/builtins/#string-tests) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A signal's role often sits at the end of its name. `_CS` marks a chip select, `_N` or `_B` an
active-low signal, and `_P` and `_N` the halves of a differential pair. A suffix test selects every
net in that role, and a negated test finds the ones missing something the role needs.

### For software engineers

`str.suffix` is `strings.HasSuffix` over two bound strings, exposed as a two-argument filter. It is
the `LIKE '%x'` of the query language, without wildcards and without case folding. The profile
matchers in `stdlib/profiles` build their name tests from it and from `str.prefix`.

### Datalog

Every I2C clock net, with the parts on it:

```
component.net(?r, ?n), str.suffix(?n, "_SCL") => ?n, ?r
```

Every net named for a 3.3 V supply, with the parts on it:

```
component.net(?r, ?n), str.suffix(?n, "_3V3") => ?n, ?r
```

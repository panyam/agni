## str.capitalize

### What it is

`str.capitalize(string, capitalized)` binds `capitalized` as `string` with its first character in
upper case and the rest left as written.

`str.capitalize` comes from jaala's standard library. The [Building
text](https://panyam.github.io/jaala/reference/builtins/#building-text) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A query's answer often feeds a report a person reads. A class such as `connector` or a role such as
`rail` reads better there with a leading capital, and the rest of the word stays as written.

### For software engineers

It upper-cases the first rune and copies the rest unchanged. Unlike Python's `str.capitalize` it
does not lower-case the remainder.

### Datalog

A name in upper case, beside the same name capitalized:

```
str.upper("can1_txd", ?u), str.capitalize("can1_txd", ?c) => ?u, ?c
```

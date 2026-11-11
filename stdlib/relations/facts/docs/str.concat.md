## str.concat

### What it is

`str.concat(head, tail, text)` holds when `text` is `head` followed by `tail`. It runs from any two
of its three arguments. With `head` and `tail` bound it builds `text`. With `text` bound and one of
the other two, it cuts that piece off and binds the rest.

Cutting gives no row when `text` does not start with the head or end with the tail, so
`str.concat("CAN1_", ?rest, ?n)` both tests the prefix and binds what follows it. The match is
exact and case-sensitive. `str.concat` is a computed predicate (kind `predicate` in the catalog),
not a stored fact.

### For hardware engineers

A net name usually joins a bus or block prefix to a signal name, as `CAN1_` and `TXD` make
`CAN1_TXD`. Cutting the prefix off reads the signal name alone, so the same role can be compared
across `CAN1_` and `CAN2_` without listing every net.

### For software engineers

Forward, it is string concatenation. Backward, it is Go's `strings.CutPrefix` or
`strings.CutSuffix`, with a failed cut giving no row rather than a flag. It runs in either direction,
as `num.add` does for numbers.

### Go projector

`str.concat` has no `stdlib/relations/facts.go` projector because it is not a stored fact. The
Datalog engine's standard library registers it in `Strings` in jaala's `stdlib/strings.go`, and
`core/facts/registry.go` installs that library into the vocabulary every query uses.

### Datalog

The signal name after each `CAN1_` prefix:

```
entity(?n, "net"), str.concat("CAN1_", ?rest, ?n) => ?n, ?rest
```

The name a prefix and a signal make together:

```
str.concat("CAN1_", "TXD", ?n) => ?n
```

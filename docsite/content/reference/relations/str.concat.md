---
title: "str.concat"
description: "text is head followed by tail, run from any two of the three; cutting a head or tail off holds for none when text lacks it"
---

### What it is

`str.concat(head, tail, text)` holds when `text` is `head` followed by `tail`.

`str.concat` comes from jaala's standard library. The [Building
text](https://panyam.github.io/jaala/reference/builtins/#building-text) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

A net name usually joins a bus or block prefix to a signal name, as `CAN1_` and `TXD` make
`CAN1_TXD`. Cutting the prefix off reads the signal name alone, so the same role can be compared
across `CAN1_` and `CAN2_` without listing every net.

### For software engineers

Forward, it is string concatenation. Backward, it is Go's `strings.CutPrefix` or
`strings.CutSuffix`. It runs in either direction, as `num.add` does for numbers.

### Datalog

The signal name after each `CAN1_` prefix:

```
entity(?n, "net"), str.concat("CAN1_", ?rest, ?n) => ?n, ?rest
```

The name a prefix and a signal make together:

```
str.concat("CAN1_", "TXD", ?n) => ?n
```

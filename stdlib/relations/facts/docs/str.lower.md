## str.lower

### What it is

`str.lower(string, lower)` binds `lower` as `string` in lower case.

`str.lower` comes from jaala's standard library. The [Building
text](https://panyam.github.io/jaala/reference/builtins/#building-text) section of jaala's built-ins
reference says which arguments must be bound and what an absent or malformed input does, and its
examples run in jaala's own tests.

### For hardware engineers

Net names are compared exactly, so `can1_txd` and `CAN1_TXD` are two names. Folding both sides to
one case lets a query compare names from two tools, or two revisions, that disagree only in case.

### For software engineers

It is Go's `strings.ToLower`, which folds by Unicode rules rather than ASCII alone.

### Datalog

Each `CAN1` net beside its name in lower case:

```
entity(?n, "net"), str.prefix(?n, "CAN1"), str.lower(?n, ?l) => ?n, ?l
```

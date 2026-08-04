---
title: "net.external"
description: "the net may extend onto an unread sheet (read-gap marker)"
---

### What it is

`net.external(net)` yields one row per net the read flagged as possibly continuing onto something it
did not cover: a net joined by a name-based mechanism (a global label, a power symbol) whose other
ends may live in files or sheets the read never opened. It is a read-gap marker, not a topology
fact.

It is about read scope, not sheet membership. A net that spans ten sheets of a fully-read design
carries no `net.external` marking, because "which sheets does this net touch" is derivable topology.
`net.external` marks only nets where the read itself may be incomplete.

### For hardware engineers

A row here means: what the tool sees on this net is not proof of what the board has on it. There may
be more pins, more sources, more loads on a sheet the read did not reach. So a rule that would fire
on incomplete connectivity (a decoupling-cap-missing or crystal-load check) suppresses itself on an
external net rather than report a violation it cannot stand behind. During a review you query it to
see where the read is admitting a gap.

### For software engineers

This is the silence-is-not-coverage marker. Absence of a connection on an external net is not
evidence of no connection; the read simply may not have covered the far end, the way a partial index
cannot prove a symbol is unreferenced. Rules use it as a negative guard: fire only where the read is
known-complete. The relation is a projection over `Nets()` filtered on the `external` net attribute
(`netgraph.AttrExternal`), so rows are 1:1 with flagged nets, and an empty result means the read is
complete and the guard is a no-op.

### Go projector

`netExternalFacts` in `check/facts.go` walks `Model.Nets()` and emits a row for each net whose
`Attributes[netgraph.AttrExternal]` is `"true"`. The attribute is stamped by the netgraph solver
(`internal/netgraph/irout.go`) on a by-name net whose other ends may sit in unread files, and cleared
once a complete read resolves it (it becomes `global` instead). One row per external net; empty when
the read is complete.

### Datalog

List every net the read flagged as a possible read gap:

```
net.external(?n) => ?n
```

Use it as a caveat filter: single-connection nets that are genuinely single-pin, excluding the ones
that only look isolated because the read did not cover their far end:

```
net.pin_count(?n, 1), not net.external(?n) => ?n
```

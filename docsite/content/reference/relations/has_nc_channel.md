---
title: "has_nc_channel"
description: "one row when the design can express intentional no-connect"
---

### What it is

`has_nc_channel(present)` yields exactly one row, with the value `true`, when the design's source
format can express an intentional no-connect, and zero rows otherwise. It is a design-capability
flag, not a per-entity relation: there is never more than one row, and its presence or absence is
the whole signal. A KiCad no-connect flag (a pin typed NO_CONNECT, or a net named
`unconnected`/`no_connect`/`nc_`) makes the row appear; a bare netlist that cannot state "this pin
is deliberately open" produces no row.

### For hardware engineers

Some formats let you mark a pin as intentionally left open, so the tool can tell "this pin is
supposed to be unconnected" from "someone forgot to wire it." Others carry no such marker at all.
This flag says which world you are in. It matters because a rule that flags floating or
single-pin conditions would fire on every deliberately-open pin in a format that cannot express
intent, drowning the real problems. Querying it confirms whether the design even carries the
no-connect vocabulary a per-pin absence check depends on.

### For software engineers

This is a capability probe over the whole design, closer to a feature flag than to a row set.
Because a rule reads it as `has_nc_channel(?_)`, an absent row makes the enclosing conjunction
yield nothing, so the guard fails closed: on a format that cannot express intentional no-connect,
the gated rule produces no findings by construction. That is the point. Per-pin absence rules
must not fire where the format cannot distinguish an intentional open from a mistake (the naive
unconnected-pin check fired over 1800 times on a real netlist that had no no-connect channel; the
gate took it to zero). Absent means "the format cannot say," which is treated as "do not fire,"
never as "everything is connected."

### Go projector

`ncChannelFacts` in `check/facts.go` calls `Model.HasNoConnectChannel()`. When it is true the
projector returns a single row (subject `true`); when it is false it returns nil, so the relation
is one row or none, never more. The underlying flag is set during the model build (`check/query.go`)
whenever a pin carries the NO_CONNECT electrical type or a net uses the no-connect naming
vocabulary. Zero rows is the meaningful state, and it is what a gated rule fails closed on.

### Datalog

Probe the flag directly (one row `true`, or empty):

```
has_nc_channel(?present) => ?present
```

Use it as a guard so a per-pin absence check runs only where intentional no-connect is
expressible (unconnected pins reported only when the format can say a pin is deliberately open):

```
pin(?r, ?p), not pin.net(?r, ?p, ?_), has_nc_channel(?present) => ?r, ?p
```

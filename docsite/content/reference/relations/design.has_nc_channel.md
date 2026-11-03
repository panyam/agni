---
title: "design.has_nc_channel"
description: "one row when the design can express intentional no-connect"
---

### What it is

`design.has_nc_channel(present)` yields exactly one row, with the value `true`, when the design's source
format can express an intentional no-connect, and zero rows otherwise. It is a design-capability
flag, not a per-entity relation, so there is never more than one row, and its presence or absence is
the whole signal. A KiCad no-connect flag makes the row appear. The flag is a pin typed
NO_CONNECT, or a net named `unconnected`/`no_connect`/`nc_`. A bare netlist cannot state "this pin
is deliberately open", and it produces no row.

### For hardware engineers

Some formats let you mark a pin as intentionally left open, so the tool can tell "this pin is
supposed to be unconnected" from "someone forgot to wire it." Others carry no such marker at all.
This flag says which world you are in. It matters because of the rules that flag floating
or single-pin conditions. In a format that cannot express intent, such a rule would fire on every
deliberately-open pin and drown the real problems. Querying the flag confirms whether the design
even carries the no-connect vocabulary that a per-pin absence check depends on.

### For software engineers

This is a capability probe over the whole design, closer to a feature flag than to a row set.
A rule reads it as `design.has_nc_channel(?_)`. An absent row makes the enclosing conjunction yield
nothing, so the guard fails closed. On a format that cannot express intentional no-connect, the gated
rule then produces no findings by construction. Per-pin absence rules must not fire where the format
cannot distinguish an intentional open from a mistake. The naive unconnected-pin check fired over
1800 times on a real netlist that had no no-connect channel, and the gate took it to zero. Absent
means "the format cannot say." A rule treats that as "do not fire," never as "everything is
connected."

### Go projector

`ncChannelFacts` in `stdlib/relations/facts.go` calls `Model.HasNoConnectChannel()`. When it is true
the projector returns a single row (subject `true`); when it is false it returns nil, so the
relation is one row or none, never more. The underlying flag is set during the model build
(`core/check/query.go`) whenever a pin carries the NO_CONNECT electrical type or a net uses the
no-connect naming vocabulary. Zero rows is the meaningful state, and a gated rule fails closed on
it.

### Datalog

Probe the flag directly (one row `true`, or empty):

```
design.has_nc_channel(?present) => ?present
```

Use it as a guard so a per-pin absence check runs only where intentional no-connect is
expressible (unconnected pins reported only when the format can say a pin is deliberately open):

```
component.pin(?r, ?p), not pin.net(?r, ?p, ?_), design.has_nc_channel(?present) => ?r, ?p
```

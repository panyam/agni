---
title: "types_power_out"
description: "one row when the source format classifies power-output pins (EDIF/IPC do not, so a driver-absence check is unsound there)"
---

### What it is

`types_power_out(present)` yields exactly one row, with the value `true`, when the design's source
format classifies power-OUTPUT pins, and zero rows otherwise. Like `has_nc_channel`, it is a
design-capability flag, not a per-entity relation: there is never more than one row, and its presence
or absence is the whole signal. A KiCad or gEDA design (whose readers type a regulator output
`POWER_OUT` and a power flag) makes the row appear; an EDIF netlist or an IPC-2581 board (whose grammars
carry no power-output type) produces no row.

### For hardware engineers

Some formats let you mark a pin as a power SOURCE (a regulator's output, a power flag), so the tool can
tell a rail that has a source from one that does not. EDIF and IPC do not — a regulator's output pin
reads as a plain input there, indistinguishable from the loads it feeds. This flag says which world you
are in. It matters because "this power rail has no source" is only a real finding when the format could
have named a source; on EDIF that same net looks sourceless for every switched or derived rail, which
is a false alarm, not a defect.

### For software engineers

This is a capability probe over the whole design, closer to a feature flag than a row set. Because a
rule reads it as `types_power_out(?_)`, an absent row makes the enclosing conjunction yield nothing, so
the guard fails closed: on a format that cannot type power outputs, a driver-absence rule produces no
findings by construction. That is exactly why `power-input-not-driven` gates on it (WS3-072): the
POWER_IN pin stamp makes supply *inputs* visible on EDIF, but the source side stays under-typed, so
inferring "unpowered" from the absence of a typed driver would false-fire on every switched rail.
Absent means "the format cannot say," treated as "do not fire," never as "the rail is unpowered." The
gate reads the same capability through the `design.types_power_out` spec fact; this relation is its
queryable twin, so you can ask "is a driver-absence check even sound on this design" before trusting one.

### Go projector

`typesPowerOutFacts` in `check/facts.go` calls `Model.FormatTypesPowerOut()`. When it is true the
projector returns a single row (subject `true`); when false it returns nil, so the relation is one row
or none, never more. The capability is derived from `SourceFormat` (false for an `edif`/`ipc` prefix),
so it needs no precomputed state. Zero rows is the meaningful state, and it is what a gated rule fails
closed on.

### Datalog

Probe the flag directly (one row `true`, or empty):

```
types_power_out(?present) => ?present
```

Use it as a guard so a driver-absence check runs only where power outputs are typed (a power-input pin
with no source reported only when the format could have named the source):

```
pin.type(?r, ?p, "power_in"), types_power_out(?present) => ?r, ?p
```

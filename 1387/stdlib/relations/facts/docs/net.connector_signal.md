## net.connector_signal

### What it is

`net.connector_signal(net)` yields one row per net that leaves the board through a connector and is a
SIGNAL, not power. Both ESD rules share this scope, and the relation projects it so a
datalog-authored check can select the same nets the Go rules do.

A net qualifies when a connector sits on it and none of the exclusions apply: it is not a rail or
ground (by name or by fact), not marked global or power-driven, not a deliberately unconnected pad,
and no power pin is reachable from it through the series walk.

### For hardware engineers

An ESD review is actually about this set of lines. A signal that leaves the board is a path for a
discharge to travel into whatever drives it, and a person touches the parts on the far end of a
connector.

Each exclusion removes a kind of net, and an ESD verdict would otherwise misjudge it. A power rail
arriving on the same connector is a different review with different rules (input protection: fuses,
reverse blocking, inrush), so it is subtracted here rather than lumped in. A ground pin needs no
clamp. A pad wired to nothing on purpose is not an exposure. Querying this relation on a real board
is a quick way to check that the engine agrees with you about which lines are exposed, before
trusting any ESD verdict built on top of it.

### For software engineers

It is a filtered projection over `Nets()`, 1:1 with in-scope nets. It is empty on a design with no
connectors. That empty answer is genuine rather than permissive, because a board that exposes nothing
has no ESD question to fail.

Every other part of the ESD guard stack could be composed from other relations, and this one could
not. The
protection predicates are reachability questions, and they became plain datalog once `net.reaches` carried
distance (WS3-112):

```
net.reaches(?n, ?rn, ?h), ?h <= 2, component.net(?t, ?rn), component.class(?t, "tvs")
```

The scope could not, because its guards read net ATTRIBUTES (`global`, `power_driven`) and the
no-connect channel. `net.attr` now exposes the attributes (agni 691). Reassembling the scope clause
by clause in datalog would still eventually drop a guard, and a dropped guard here is a false FAIL on
a rail or an unconnected pad rather than a missed defect.

### Go projector

`externalSignalNetFacts` in `stdlib/relations/facts.go` walks `Model.Nets()` and emits a row where
`check.ExternalSignalNet` holds. That function lives in `core/check/guards.go` beside the guards it
calls. `esd-protection` and `esd-clamp-not-tvs` evaluate that SAME predicate, so a datalog check
and the Go rules cannot disagree about scope. One row per in-scope net; zero rows when the
design has no connector-facing signals.

### Datalog

Every net an ESD review is about:

```
net.connector_signal(?n) => ?n
```

The unprotected ones have nothing clamping within two series crossings. The `esd` profile
requirement compiles to the same shape:

```
net.connector_signal(?n),
not_clamped(?n)
=> ?n
```

written out, since `not_clamped` is not built in:

```
clamped(?n) :- net.reaches(?n, ?rn, ?h), ?h <= 2, component.net(?t, ?rn), component.class(?t, "tvs");
exposed(?n) :- net.connector_signal(?n), not clamped(?n);
exposed(?n) => ?n
```

Which parts sit on the exposed lines, for a quick read of what is at risk:

```
net.connector_signal(?n), component.net(?r, ?n) => ?n, ?r
```

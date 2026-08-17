## external_signal_net

### What it is

`external_signal_net(net)` yields one row per net that leaves the board through a connector and is a
SIGNAL, not power. It is the scope both ESD rules share, projected so a datalog-authored check can
select the same nets the Go rules do.

A net qualifies when a connector sits on it and none of the exclusions apply: it is not a rail or
ground (by name or by fact), not marked global or power-driven, not a deliberately unconnected pad,
and no power pin is reachable from it through the series walk.

### For hardware engineers

This is the set of lines an ESD review is actually about. A signal that leaves the board is a path
for a discharge to travel into whatever drives it, and the parts on the far end of a connector are
the ones a person touches.

The exclusions are what keep the question honest. A power rail arriving on the same connector is a
different review with different rules (input protection: fuses, reverse blocking, inrush), so it is
subtracted here rather than lumped in. A ground pin needs no clamp. A pad wired to nothing on purpose
is not an exposure. Querying this relation on a real board is the fastest way to check that the engine
agrees with you about which lines are exposed, before trusting any ESD verdict built on top of it.

### For software engineers

A filtered projection over `Nets()`, 1:1 with in-scope nets. Empty on a design with no connectors,
which is a genuine answer rather than a permissive one: a board that exposes nothing has no ESD
question to fail.

It is the one part of the ESD guard stack that could not be composed from other relations. The
protection predicates are reachability questions, and they became plain datalog once `reaches` carried
distance (WS3-112):

```
reaches(?n, ?rn, ?h), ?h <= 2, component-on-net(?t, ?rn), component.class(?t, "tvs")
```

The scope could not, because its guards read net ATTRIBUTES (`global`, `power_driven`) and the
no-connect channel, none of which have a relation of their own. Reassembling it clause by clause in
datalog would eventually drop one, and a dropped guard here is a false FAIL on a rail or an
unconnected pad rather than a missed defect.

### Go projector

`externalSignalNetFacts` in `stdlib/relations/facts.go` walks `Model.Nets()` and emits a row where
`check.ExternalSignalNet` holds. That function lives in `core/check/guards.go` beside the guards it
calls, and it is the SAME predicate `esd-protection` and `esd-clamp-not-tvs` evaluate, so a datalog
check and the Go rules cannot disagree about scope. One row per in-scope net; zero rows when the
design has no connector-facing signals.

### Datalog

Every net an ESD review is about:

```
external_signal_net(?n) => ?n
```

The unprotected ones, matching the shape the `esd` profile requirement compiles to. Nothing clamping
within two series crossings:

```
external_signal_net(?n),
not_clamped(?n)
=> ?n
```

written out, since `not_clamped` is not built in:

```
clamped(?n) :- reaches(?n, ?rn, ?h), ?h <= 2, component-on-net(?t, ?rn), component.class(?t, "tvs");
exposed(?n) :- external_signal_net(?n), not clamped(?n);
exposed(?n) => ?n
```

Which parts sit on the exposed lines, for a quick read of what is at risk:

```
external_signal_net(?n), component-on-net(?r, ?n) => ?n, ?r
```

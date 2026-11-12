## pin.net

### What it is

`pin.net(ref_des, pin, net)` yields the net that a pin connects to, where `net` is the resolved net
name. A row is emitted only for a connected pin; a pin wired to nothing produces no row. The absence
of a row is the queryable signal for "this pin is unconnected."

### For hardware engineers

This is the connectivity answer at pin granularity, naming the node each pin is tied to after net
solving. If a pin has no row, the read found nothing joined to it. You query it to trace a part's
connections, or, by its absence, to find dangling or unconnected pins. It is keyed by
`(ref_des, pin)`, so it joins a pin to everything the net-tier relations know about the pin's node,
such as its fan-out, whether it is a rail, and whether it is bus-like.

### For software engineers

A net is a **shared channel aliasing fields of many instances** (see
[the analogy guide](../../../../docsite/content/reference/analogy.md)). `pin.net` binds one field to
the channel it aliases, so it is the member-to-net edge. Rows are 1:1 with connected pins. An
unconnected pin has no row at all rather than a row with an empty net. So to find unconnected pins,
a query asks for a `component.pin` with no matching `pin.net` rather than testing a net string for
emptiness. Because a pin belongs to exactly
one net by definition, a well-formed read gives at most one `pin.net` row per pin (two would be the
`reader.pin_net_conflict` integrity break).

### Go projector

`pinFacts` in `stdlib/relations/facts.go` calls `Model.PinNetName(ref, des)` for each pin and emits a
`pin.net(ref_des, pin, net)` row only when the returned name is non-empty. `PinNetName` looks the
pin up in the model's pin-to-net index (built once at load from the solved nets' connections) and
returns `""` for a pin that no net claims. So a connected pin yields exactly one row, an
unconnected pin yields none, and the design's whole set of `pin.net` rows is empty when there is no
part-pin data.

### Datalog

Every connected pin and its net:

```
pin.net(?r, ?p, ?n) => ?n
```

Join to `net.rail` to find the pins that connect directly to a power or ground rail:

```
pin.net(?r, ?p, ?n), net.rail(?n) => ?r
```

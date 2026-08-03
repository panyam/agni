## pin.net

### What it is

`pin.net(ref_des, pin, net)` yields the net a pin lands on, where `net` is the resolved net name.
A row is emitted only for a connected pin; a pin wired to nothing produces no row. The absence of
a row is the queryable signal for "this pin is unconnected."

### For hardware engineers

This is the connectivity answer at pin granularity: which node each pin is tied to after net
solving. A pin with no row is one the read found nothing joined to. You query it to trace a part's
connections, or, by its absence, to find dangling or unconnected pins. Because it is keyed by
`(ref_des, pin)`, it is the join point between a pin and everything the net-tier relations know
about the node it sits on (its fan-out, whether it is a rail, whether it is bus-like).

### For software engineers

A net is a **shared channel aliasing fields of many instances** (ANALOGY.md). `pin.net` is the
binding of one field to the channel it aliases: the member-to-net edge. Rows are 1:1 with
connected pins. The "absent if unconnected" semantics matter: an unconnected pin is not a row with
an empty net, it is no row at all, so a query that wants unconnected pins asks for a `pin` with no
matching `pin.net` rather than testing a net string for emptiness. Because a pin belongs to exactly
one net by definition, a well-formed read gives at most one `pin.net` row per pin (two would be the
`pin_net_conflict` integrity break).

### Go projector

`pinFacts` in `check/facts.go` calls `Model.PinNetName(ref, des)` for each pin and emits a
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

Join to `rail` to find the pins that connect directly to a power or ground rail:

```
pin.net(?r, ?p, ?n), rail(?n) => ?r
```

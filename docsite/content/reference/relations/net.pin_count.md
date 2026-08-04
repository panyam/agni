---
title: "net.pin_count"
description: "the number of connections on a net"
---

### What it is

`net.pin_count(net, count)` yields one row per net, pairing the net name with the number of
connections on it. The count is the size of the net's connection list, so it answers "how many
pins does this net tie together." Every net gets a row (unlike the partial relations, this one is
total over nets).

### For hardware engineers

The connection count tells you a net's electrical role at a glance:

- **A 1-pin net is a stub.** One pin wired to nothing else, which usually means a dangling or
  unfinished connection (unless the pin is an intentional no-connect).
- **A 2-pin net is point-to-point.** A signal running directly between two parts, the normal case
  for a discrete link.
- **A high-count net is distribution.** A rail, a ground, or a clock fanned out to many loads,
  where "what connects to it" is a distribution question rather than a path.

During a review you query it to find stubs to chase down, or to confirm a net you thought was
point-to-point has not quietly grown a large fan-out.

### For software engineers

This is the degree of each net node in the design graph: the number of edges incident on the net.
It is a projection over `Nets()` that reads `len(net.Connections)`, so rows are 1:1 with nets and
the count carries as a number for direct comparison. It is the fan-out primitive the higher-level
relations build on: `net.bus_like` treats a count above 16 as one of the conditions that make a
net a shared-distribution node the reachability walk must stop at, so `net.pin_count` is the
measured input behind that threshold.

### Go projector

`netPinCountFacts` in `check/facts.go` walks `Model.Nets()` and emits one row per net with the
numeric count set to `len(n.Connections)`. One row per net, always. An empty result means the
design has no nets at all (not that counts were unavailable), since every net yields a count.

### Datalog

Every net and its connection count:

```
net.pin_count(?n, ?c) => ?n, ?c
```

Filter to the single-pin stubs (the likely-dangling nets), a threshold query that reads the count
as a number:

```
net.pin_count(?n, ?c), ?c < 2 => ?n
```

### Schematic

![A 1-pin net is a stub, a 2-pin net is point-to-point, a high-count net is distribution]({{.Site.PathPrefix}}/static/images/catalog/relations/net.pin_count.svg)

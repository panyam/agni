---
title: "component.probed_one"
description: "A two-terminal part with a test point on exactly one of its nets, naming the probed net and then the other, which is where a missing test point would go."
---

### What it is

`component.probed_one(part, probed, unprobed)` yields one row per two-terminal part with a test
point on exactly one of its two nets, naming the net that has one and then the net that does not.

### For hardware engineers

These are the cheapest coverage to win back. In-circuit test needs both of a part's nodes, and these
parts already have one, so a single test point on `unprobed` makes the part measurable. When several
parts share the same unprobed net, one test point there recovers all of them, which is the reason to
group by that column.

### For software engineers

It is `component.two_terminal` with exactly one side passing `net.has_test_point`. The relation is
defined by two clauses, one per orientation of the pair, because `two_terminal` orders its nets by
name and the probed net can be either one. Each part answers once, since exactly one of the two
clauses can hold for it.

### Datalog

Which unprobed nets would recover the most parts:

```
component.probed_one(?r, ?p, ?u) => ?u, count(?r), list(?r)
```

The same list with part numbers, as the netlist audit shows it:

```
component.probed_one(?r, ?p, ?u), component.mpn(?r, ?mpn) => ?mpn, ?r, ?p, ?u
```

### How it is defined

`component.probed_one(r: component, probed: net, unprobed: net)`, a derived relation in the `component` module, defined in Datalog in [`stdlib/lib/component.dl`](https://github.com/panyam/agni/blob/main/stdlib/lib/component.dl). In a clause, a bare name is another member of the same module and a dotted name is a full path.

```
probed_one(?r: component, ?probed: net, ?unprobed: net) :- two_terminal(?r, ?probed, ?unprobed), net.has_test_point(?probed), not net.has_test_point(?unprobed);
probed_one(?r, ?probed, ?unprobed) :- two_terminal(?r, ?unprobed, ?probed), net.has_test_point(?probed), not net.has_test_point(?unprobed);
```

`agni query --relations component.probed_one` prints the same definition. [Adding a library member](../../../build/library-member/) explains how the library is built.

---
title: "component.probed_both"
description: "A two-terminal part with a test point on both of its nets, so it can be measured in circuit."
---

### What it is

`component.probed_both(part)` holds for each two-terminal part (`component.two_terminal`) whose two
nets both carry a test point (`net.has_test_point`).

### For hardware engineers

These are the parts in-circuit test can measure. A fixture reaches a part's two nodes through test
points, drives a stimulus across them, and checks the value it reads against the part's nominal, so
a wrong value, a missing part, or a part fitted backwards shows up before the board is powered.
Coverage is usually reported as the share of parts that can be measured, which is this relation
counted against `component.two_terminal`.

A part listed here is reachable rather than guaranteed to measure cleanly. Parts in parallel are
measured together, and a large capacitance on either node can swamp a small one. Those are judged
by the test engineer from the schematic.

### For software engineers

It is `component.two_terminal` filtered by `net.has_test_point` on both of its nets, so the answer
is a set of parts. Its complement within two-terminal parts splits in two: `component.probed_one`
for parts with one probed net, and the parts where neither net is probed, which no member names and
a query finds by negating both.

### Datalog

Measurable parts with their part numbers:

```
component.probed_both(?r), component.mpn(?r, ?mpn) => ?mpn, ?r
```

Two-terminal parts that cannot be measured at all:

```
component.two_terminal(?r, ?a, ?b), not net.has_test_point(?a), not net.has_test_point(?b) => ?r, ?a, ?b
```

### How it is defined

`component.probed_both(r: component)`, a derived relation in the `component` module, defined in Datalog in [`stdlib/lib/component.dl`](https://github.com/panyam/agni/blob/main/stdlib/lib/component.dl). In a clause, a bare name is another member of the same module and a dotted name is a full path.

```
probed_both(?r: component) :- two_terminal(?r, ?a, ?b), net.has_test_point(?a), net.has_test_point(?b);
```

`agni query --relations component.probed_both` prints the same definition. [Adding a library member](../../../build/library-member/) explains how the library is built.

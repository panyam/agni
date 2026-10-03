---
title: "net.test_point_count"
description: "How many test points sit on each net, 0 for a net with none, so every net answers one row."
---

### What it is

`net.test_point_count(net, count)` answers one row for every net in the design, with the number of
test points on it. A net with none answers `0` rather than being left out. A test point is any
component classified `test_point`, the same set `net.has_test_point` reads, so a project that
extends the class in its naming conventions is counted the same way. Each test point counts once per
net however many of its pins land there.

### For hardware engineers

A coverage table lists every net beside how many test points it carries, and the rows that read `0`
matter most, because those are the nets a bed-of-nails fixture or a flying probe cannot reach. A
count of one on a rail or a high-current net can also be thin, since a fixture often wants more than
one contact there. Reading the whole table at once shows both.

### For software engineers

It is a grouped count with the empty groups filled in. A plain `count(distinct ?tp)` grouped by net
gives no row for a net with nothing to count, so the uncovered nets would need a second query and a
merge. This member is two clauses over one private helper: the helper counts the test points on each
covered net, and the second clause gives `0` to every net the helper has no row for. The helper is
separate because a rule that aggregates must be its relation's only rule. Every net answers exactly
once.

### Datalog

Every net and its test-point count, uncovered nets first:

```
net.test_point_count(?n, ?c) => ?n, ?c order by ?c, ?n
```

Nets carrying more than one test point:

```
net.test_point_count(?n, ?c), ?c > 1 => ?n, ?c
```

### How it is defined

`net.test_point_count(n: net, c: number)`, a derived relation in the `net` module, defined in Datalog in [`stdlib/lib/net.dl`](https://github.com/panyam/agni/blob/main/stdlib/lib/net.dl). In a clause, a bare name is another member of the same module and a dotted name is a full path.

```
test_point_count(?n: net, ?c: number) :- _test_points(?n, ?c);
test_point_count(?n: net, 0) :- entity(?n, "net"), not _test_points(?n, _);
```

`agni query --relations net.test_point_count` prints the same definition. [Adding a library member](../../../build/library-member/) explains how the library is built.

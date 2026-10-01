## net.has_test_point

### What it is

`net.has_test_point(net)` holds for each net that at least one test point sits on. A test point
here is any component classified `test_point`, which by default is a part whose reference
designator starts with `TP`, and a project can extend that class in its naming conventions. A net
carrying three test points answers once.

What it does not count matters as much. A connector pin, an exposed pad, or a via someone plans to
probe gives physical access too, and none of those is a `test_point` part, so a net reached only
that way reads as having no test point.

### For hardware engineers

A test point is the place a probe lands. During bring-up it is where a scope or a meter clips on,
and in production it is where a fixture's spring pin (a bed of nails) or a flying probe makes
contact for in-circuit test. A net without one can still be reached at a component pin, but on a
fine-pitch package or under a BGA there may be no pin anyone can touch.

So the question a review asks is usually the negation, which nets have no test point at all, and
then which of those matter: a rail, a reset line, a clock, the two ends of a part that in-circuit
test should be able to measure. `component.probed_both` and `component.probed_one` build that last
question out of this one.

### For software engineers

It is a semi-join. `component.net` pairs parts with the nets they sit on, `component.class`
filters the parts to test points, and the head keeps only the net, so the answer is a set of nets
with no duplicates however many test points share one.

The useful form is usually negated, `not net.has_test_point(?n)`, which needs something else in
the body to range over nets first. `entity(?n, "net")` is the complete list, so it finds nets that
nothing else in the query would reach.

### Datalog

Every net with no test point:

```
entity(?n, "net"), not net.has_test_point(?n) => ?n
```

How many nets are covered, against how many there are:

```
net.has_test_point(?n) => count(?n)
```

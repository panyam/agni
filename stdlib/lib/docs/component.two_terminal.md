## component.two_terminal

### What it is

`component.two_terminal(part, a, b)` yields one row per part whose pins land on exactly two
distinct nets, naming those two nets in name order, so `a` sorts before `b` and each part answers
once.

It is decided by connectivity and not by class. A resistor, a capacitor, an inductor, a ferrite, a
diode, a fuse and a crystal are all two-terminal, and so is a two-pin connector or a three-pin part
with two of its pins tied together. A part with both pins on one net (a zero-ohm link to itself, or
a shorted part) touches one net and is not included. Add a `component.class` atom when you mean one
kind of part.

### For hardware engineers

Two-terminal parts are the ones a tester can measure from outside the part. In-circuit test drives
a known current or voltage between two nodes and reads back a resistance, a capacitance or a diode
drop, so every measurable part is defined by the pair of nets it sits between. Asking about the
pair is how test coverage, series elements in a signal path, and parts bridging a rail to ground
are all found.

One limitation follows from the measurement itself. Two parts in parallel between the same pair of
nets are measured together, and a tester sees their combined value. This relation lists them as
two rows and says nothing about that.

### For software engineers

It joins `component.net_count` (to keep parts on exactly two nets) with `component.net` twice (to
bind both of them), and the `?a < ?b` comparison turns an unordered pair into one ordered row. A
query that needs either orientation can read both, `two_terminal(?r, ?x, ?y)` and
`two_terminal(?r, ?y, ?x)`, as `component.probed_one` does.

### Datalog

Two-terminal parts with a rail on one side:

```
component.two_terminal(?r, ?a, ?b), net.rail(?a) => ?r, ?a, ?b
```

Only the resistors:

```
component.two_terminal(?r, ?a, ?b), component.class(?r, "resistor") => ?r, ?a, ?b
```

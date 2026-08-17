## net.signal_level

### What it is

`net.signal_level(net, volts)` yields the voltage a **non-rail** net's name declares. It is the other
half of `net.nominal_voltage`: the same name-derived number, over the nets that are not rails.

The two are exhaustive and disjoint over the nets whose names parse. A net carrying the rail role
(which includes ground) goes to `net.nominal_voltage`; every other net whose name carries a voltage
token lands here. A name with no parseable token, or with tokens that disagree (`12V_TO_5V`), yields
no row in either relation.

### Why the split exists

`net.nominal_voltage` is documented as a rail's nominal, and its projector used to emit a row for
*any* net whose name parsed. Many teams encode a signalling level into a signal net's name, so those
levels were landing in a relation that means "rail nominal". The number was right and the relation
carrying it was not.

A verified instance: a net named `U3_12_U7_4_3V3` yielded a nominal of 3.3 while classifying as
neither rail nor ground. A query asking for rail voltages got a signal net back, and a rule
quantifying over rails could not state that it meant rails.

### For hardware engineers

A signal net named for its level is telling you what logic family drives it: `..._3V3` is a 3.3 V
signal, `..._1V8` is 1.8 V. That is worth knowing, and it is a different claim from "this rail is
supposed to sit at 3.3 V". A level mismatch across a link is an interfacing question (does the
receiver tolerate the driver's level), while a rail nominal out of range is a supply question.

Treat it as the design's own label, not a measurement. It is exactly as trustworthy as the naming
convention that produced it, so the level and the rail nominal are kept apart rather than
merged into one number a rule cannot interpret.

### For software engineers

The gate is `Model.IsRailNet`, the narrow role question, which reads the stamped `ir.Net.roles` fact
and falls back to the naming lexicon only for a net that skipped the loader. So the split follows the
same role stamp the rest of the engine uses, rather than inventing a second notion of "rail".

This gates the RELATION only. `check.NominalVoltageFromName` is a pure string function with no net to
ask about, so a Go rule that holds a net and wants rails must gate for itself. The pin-tracking rules
are the worked example.

### Go projector

`netSignalLevelFacts` in `stdlib/relations/facts.go` iterates `Model.Nets()`, skips any net carrying
the rail role, and emits one row per remaining net whose name parses, with `Num` set to the voltage
and a citation to the net's IR provenance. Netlist-tier: it needs no `--params`.

### Datalog

Every signal net whose name declares a level:

```
net.signal_level(?net, ?v) => ?net, ?v
```

Levels that disagree across the design, the cheap way to spot a mixed-voltage interface
somebody has to think about:

```
net.signal_level(?a, ?v1), net.signal_level(?b, ?v2), ?v1 != ?v2 => ?a, ?v1, ?b, ?v2
```

A signal whose declared level does not match any rail the design carries, which is usually either a
naming slip or a level nobody generates:

```
net.signal_level(?net, ?v), not net.nominal_voltage(?rail, ?v) => ?net, ?v
```

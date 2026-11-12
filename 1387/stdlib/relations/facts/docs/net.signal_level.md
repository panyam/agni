## net.signal_level

### What it is

`net.signal_level(net, volts)` yields the voltage declared by a **non-rail** net's name. It is the
other half of `net.nominal_voltage`, carrying the same name-derived number over the nets that are
not rails.

The two are disjoint, and since agni 679 they are not exhaustive. A net with the rail role goes to
`net.nominal_voltage`, and the rail role includes ground. Every other net whose name carries a
voltage token goes here. A name with no parseable token, or with tokens that disagree (`12V_TO_5V`),
yields no row in either relation.

**A regulator internal is the third case and yields no row either.** `12V_FB` and `12V_SW` are named
after the rail they serve rather than after what they carry, so the number is a different net's
voltage and is neither a rail nominal nor a signalling level. If only the rail side dropped them,
they would show up here instead, with the same wrong number under a relation that claims less. So
both projectors exclude them.

### Why the split exists

`net.nominal_voltage` is documented as a rail's nominal, and its projector used to emit a row for
*any* net whose name parsed. Many teams encode a signalling level into a signal net's name, so those
levels were showing up in a relation that means "rail nominal". The number was right and the relation
carrying it was not.

In one verified instance, a net named `U3_12_U7_4_3V3` yielded a nominal of 3.3 while classifying as
neither rail nor ground. A query asking for rail voltages got a signal net back, and a rule
quantifying over rails could not state that it meant rails.

### For hardware engineers

A signal net named for its level is telling you what logic family drives it: `..._3V3` is a 3.3 V
signal, `..._1V8` is 1.8 V. That is worth knowing, and it is a different claim from "this rail is
supposed to sit at 3.3 V". A level mismatch across a link is an interfacing question (does the
receiver tolerate the driver's level), while a rail nominal out of range is a supply question.

Treat it as the design's own label, not a measurement. It is exactly as trustworthy as the naming
convention that produced it. So the level and the rail nominal are kept apart, rather than merged
into one number that no rule can interpret.

### For software engineers

The gate is `Model.IsRailNet`, the narrow role question, which reads the stamped `ir.Net.roles` fact
and falls back to the naming lexicon only for a net that skipped the loader. So the split follows the
same role stamp the rest of the engine uses, rather than inventing a second notion of "rail".

This gates the RELATION only. `check.NominalVoltageFromName` is a pure string function with no net to
ask about. So when a Go rule holds a net and wants rails, the rule must gate for itself. The
pin-tracking rules are the worked example.

### Go projector

`netSignalLevelFacts` in `stdlib/relations/facts.go` iterates `Model.Nets()`, skips any net carrying
the rail role or classed as a regulator internal (`Model.IsRegulatorInternalNet`), and emits one row
per remaining net whose name parses, with `Num` set to the voltage and a citation to the net's IR
provenance. The relation is netlist-tier and needs no `--params`.

### Datalog

Every signal net whose name declares a level:

```
net.signal_level(?net, ?v) => ?net, ?v
```

Levels that disagree across the design. This is the cheap way to spot a mixed-voltage interface
that somebody has to think about:

```
net.signal_level(?a, ?v1), net.signal_level(?b, ?v2), ?v1 != ?v2 => ?a, ?v1, ?b, ?v2
```

A signal whose declared level matches no rail in the design. That is usually either a naming slip
or a level that nothing generates:

```
net.signal_level(?net, ?v), not net.nominal_voltage(?rail, ?v) => ?net, ?v
```

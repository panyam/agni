## param.pin

### What it is

`param.pin(mpn, pin, name, function)` yields one row per pin a datasheet declares for a part that
joined to the design, keyed by manufacturer part number (`mpn`) and the pin's **spec-local id**. The
row carries the pin's printed `name` (`VCCA`, `GND`, `NC`) and its `function` token: `power_input`,
`power_output`, `ground`, `input`, `output`, `bidirectional`, `passive`, `no_connect`, or
`unspecified`.

The key is the id and not the name, and that is deliberate. A part routinely prints one name on
several terminals, so a name-keyed relation would merge two pins that may carry different limits,
collapsing exactly what the pin contract exists to keep apart. The name is published as a value, so a query
can still match on it and a finding can print it.

`unspecified` is an ordinary answer here, unlike `unspecified` on `param.range`'s kind. A pin
function table may have no type column at all, and a pin whose name and number are known is still
worth recording.

This is the datasheet tier of the query surface. It is EMPTY unless `agni` runs with `--params <dir>`
pointing at a seeded `PartSpec` corpus, and empty for a spec seeded before pin binding existed, so a
rule reading it reports not-applicable rather than a false pass.

### For hardware engineers

One row is one line of the part's pin function table. `function` is that table's Type column
(I, O, I/O, P, G, NC) as a word, so you can ask questions like "which terminals of this part are
supplies" without knowing the vendor's naming habits.

What this relation does NOT do is tell you which leg of the package a pin is. A pin here is a
terminal of the die, and its number depends on which body the part ships in: the same terminal is
leg 14 in one package and leg 11 in another. Numbering lives in the contract rather than on this
relation, and mapping a schematic's pin onto one of these ids is `param.ResolvePin`'s job, because
that mapping can legitimately REFUSE (an ambiguous name with no package identified, or a name and a
number that disagree) and a datalog join has no way to refuse. See
[pins and packages](../../../../docsite/content/reference/pins-and-packages.md) for the physical
picture.

### For software engineers

This is the dimension table for the pin tier: a stable id per terminal, plus two descriptive
columns. `param.pin_range` is the fact table that references it, and the two join on `(mpn, pin)`.

Think of `mpn` as the type and `pin` as a field on that type. A design may place fifty instances of
the part; nothing here is per-instance, and no reference designator appears in this relation at all.
Getting from an instance to a pin goes through the netlist tier (`component.mpn`, then `pin.net`),
not through this one.

### Go projector

`paramPinFacts` in `stdlib/relations/facts.go` walks `Model.Components()`, resolves each component's
MPN and spec, dedupes by MPN (a `PartSpec` describes a type, so emitting per component would
multiply every fact by its placement count), and emits one row per `spec.Pins` entry via
`specParamPinRows`. `Object` is `Pin.id`, `Value` is `Pin.name`, `Qualifier` is the function token,
and `Cite` is the pin's own datasheet provenance, which `param.Validate` requires. Empty without
`--params`.

### Datalog

Both queries need `--params`. List every supply terminal of every seeded part:

```
param.pin(?mpn, ?pin, ?name, "power_input") => ?mpn, ?pin, ?name
```

Find parts that print one name on more than one terminal, the case that makes a name
unusable as a key:

```
param.pin(?mpn, ?a, ?name, ?f), param.pin(?mpn, ?b, ?name, ?g), ?a != ?b => ?mpn, ?name, ?a, ?b
```

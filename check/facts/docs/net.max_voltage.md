## net.max_voltage

### What it is

`net.max_voltage(net, volts)` yields one row per net that declares a rail voltage, pairing the
net name with a number. It is emitted only where the design states a voltage: an explicit
`max_voltage` attribute on the net wins, and otherwise the value is read from the net's name (a
rail named `+5V`, `3V3`, or `12V0` names its own nominal). A net with neither channel produces no
row, so the relation is a partial map over nets, not a value for every net.

### For hardware engineers

These are the supply rails whose working voltage the netlist actually knows. Most signal nets
carry no voltage claim, so they are simply absent here. During a review you query it to see which
rails the tool can reason about numerically, and to feed a comparison against a part's ratings:
`net.max_voltage` is the design-side number the `supply-exceeds-abs-max` join checks against a
datasheet absolute maximum. A rail you expected to appear but does not has a name the extractor
could not read as a voltage (or two disagreeing tokens like `12V_TO_5V`, which it refuses to
guess between).

### For software engineers

Think of it as a lookup that resolves a net to a declared constant, with a miss when nothing
declares one. It is a projection over `Nets()`: for each net the extractor tries the explicit
attribute first, then the name, and skips the net when both fail. Rows are 1:1 with the nets that
have a declared voltage; an empty result means no net in the design states or names a voltage.
The value is carried as a number, so a query can range or compare on it without re-parsing the
`5V` text form.

### Go projector

`netMaxVoltageFacts` in `check/facts.go` walks `Model.Nets()` and, for each net, calls
`railMaxVoltage(n, n.Name)` (in `check/params.go`). That helper returns the explicit
`max_voltage` attribute when present, else the name-derived nominal (`nominalVoltageFromName`),
and reports `ok=false` when neither yields a number. The projector emits a row only when `ok` is
true, filling both the rendered `5V`-style value and the numeric field. One row per net that
declares a voltage; empty when no net declares or names one.

### Datalog

List every net with a declared voltage:

```
net.max_voltage(?n, ?v) => ?n, ?v
```

Find the rails above 3 V and the parts sitting on them:

```
net.max_voltage(?n, ?v), ?v > 3, component-on-net(?r, ?n) => ?r, ?n, ?v
```

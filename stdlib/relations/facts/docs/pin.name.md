## pin.name

### What it is

`pin.name(ref_des, pin, name)` yields the part type's FUNCTIONAL name for a pin. `pin` is the
package designator, and every other pin relation keys on it. `name` is what the datasheet calls it.
On an MCU that is the difference between `41` and `PTC11`, or between `12` and `SDA`.

A row is emitted only when the part type declares a name, so a format carrying no pin names answers
nothing rather than answering `""`. KiCad spells "this pin has no name" as `~`, which reaches the IR
verbatim and is read as absent here.

### For hardware engineers

Two things name the same pin and neither is redundant. The **designator** says where the pin sits on
the package, and netlists and layout tools speak in designators. The **name** says what the pin is
for, and datasheets, firmware headers and pin-assignment maps speak in names. A review question is
almost always asked in the second vocabulary and the netlist almost always answers in the first.

Until this relation existed only the designator was reachable, so "is the ADC input on the pin the
IO map says" could not be asked at all, even on a design where both spellings sit in the file. agni
issue 517 opens with that gap.

How far apart the two spellings are depends on the format:

| format | `designator` | `name` |
|---|---|---|
| KiCad | pin number | pin name, so the two genuinely differ (`1` against `K`) |
| Telesis | the `$PINS` target | from `PinLabel` |
| EDIF | the port's declared designator, or the port NAME when it declares none | the port name |

**On EDIF the two can be the same string, and a query should not assume otherwise.** When an EDIF
file declares no pin designator the reader falls back to the port name (`readers/edif/reader.go`,
issue 71), because the Model indexes pins by designator while a `portRef` names the port, so without
the fallback nothing resolved. A design read that way answers `pin.name` and `component.pin` with identical
values, which is correct rather than a defect, because the format supplied one spelling and both fields
carry it. Where the file does declare designators, they differ as everywhere else.

### For software engineers

The two are a **field's declared name against its offset** (see
[the analogy guide](../../../../docsite/content/reference/analogy.md)), where `pin` is the position in
the package and `name` is the identifier from the header file. Rows are 1:1 with named pins and
absent for the rest, so `component.pin(?r,?p), not pin.name(?r,?p,?_)` reads as "the read gave this pin no
name". That answer describes the SOURCE rather than the board.

Do not treat `name` as unique within a component. Nothing stops a part type from declaring the same
name on several pins (a device with four `GND` pins is ordinary), so a join on name alone can
multiply rows. Join on `(ref_des, pin)` when you mean one pin.

The name is recorded exactly as the source spells it, with no normalization. `PTE7` and `PTE07` are
the same pin to a person and two different strings here. To compare a name against a name from
another document, you have to canonicalize both sides first. `core/ident`'s `Canonical` is the one
canonical form. Doing that has traps, such as zero padding, invisible format characters and
multi-valued cells, and they are real enough to have produced a whole run of false warnings in a
shipped in-house checker.

### Go projector

`pinFacts` in `stdlib/relations/facts.go` calls `Model.PinName(ref, des)` for each pin and emits a
row only when the result is non-empty and is not `~`. `PinName` reads the part type's pin list,
which ingestion resolves. If a design's read supplied no part-type pin data, it yields no rows at
all. A design of entirely unnamed pins yields the same nothing, and this relation does not tell the
two apart. `component.pin` lists a component's pins, so it says whether the read produced pins in
the first place.

### Datalog

Every pin's functional name:

```
pin.name(?r, ?p, ?n) => ?r, ?p, ?n
```

What a named pin is wired to. The IO-map question is built on this join:

```
pin.name(?r, ?p, ?n), pin.net(?r, ?p, ?net) => ?r, ?n, ?net
```

Every pin the read named but left unconnected:

```
pin.name(?r, ?p, ?n), not pin.net(?r, ?p, ?net) => ?r, ?p, ?n
```

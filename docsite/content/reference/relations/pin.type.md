---
title: "pin.type"
description: "a pin's electrical type (power_in, input, output, ...)"
---

### What it is

`pin.type(ref_des, pin, etype)` yields a pin's electrical type, where `etype` is the direction
string the symbol declares: `input`, `output`, `inout`, `power_in`, `power_out`, `passive`,
`no_connect`, or `unspecified`. Unlike `pin.role`, a row is emitted for every pin; a direction the
tool has no mapping for reads as `unspecified` rather than being dropped.

### For hardware engineers

This is the pin's declared electrical type from the symbol, the same annotation ERC uses to decide
what may connect to what. A regulator's supply pin is `power_out`, an MCU's `power_in`, a
resistor's terminals `passive`. `unspecified` means the author declared nothing, which the
connectivity rules read as "skip, never guess" rather than a fault. You query it to audit
directions on a rail (which parts source it, which sink it) or to find pins the design left
untyped.

### For software engineers

`pin.type` is the **type annotation** on the pin member (ANALOGY.md: pin directions are the type
annotations the connectivity rules dispatch on). Rows are 1:1 with pins, one direction each. An
`unspecified` value is a real annotation, not a missing row: it distinguishes "declared as
untyped" from "no such pin," and rules that key on it treat it as "the author annotated nothing."
An empty result means the design carries no part-pin data.

### Go projector

`pinFacts` in `check/facts.go` calls `Model.PinDir(ref, des)` for each pin, passes it through
`dirString`, and emits a `pin.type(ref_des, pin, etype)` row. `PinDir` returns the direction the
reader recorded for that pin (last section wins for a multi-section part); `dirString` maps the
`ir.PinDirection` enum to its string, defaulting to `unspecified` for any direction it has no
explicit case for. One row per pin; empty when no component declares pins.

### Datalog

Every pin and its declared type:

```
pin.type(?r, ?p, ?t) => ?t
```

Join to `component.class` to find every power-input pin on an IC (a supply pin on a chip):

```
pin.type(?r, ?p, "power_in"), component.class(?r, "ic") => ?r
```

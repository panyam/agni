---
title: "pin.type"
description: "a pin's electrical type (power_in, input, output, ...)"
---

### What it is

`pin.type(ref_des, pin, etype)` yields a pin's electrical type. The symbol declares `etype` as one
of the direction strings `input`, `output`, `inout`, `tri_state`, `open_collector`,
`open_emitter`, `power_in`, `power_out`, `passive`, `no_connect`, or `unspecified`. Three output
kinds cannot drive both ways on their own, a tri-state, an open-collector and an open-emitter pin.
Since agni issue 1082 each keeps its own type, read from KiCad and gEDA. Unlike `pin.role`, a row
is emitted for every pin. If the tool has no mapping for a direction, it reads as `unspecified`
rather than being dropped.

### For hardware engineers

This is the pin's declared electrical type from the symbol. ERC uses the same annotation to decide
what may connect to what. A regulator's supply pin is `power_out`, an MCU's `power_in`, a
resistor's terminals `passive`. `unspecified` means the author declared nothing. The connectivity
rules read that as "skip, never guess" rather than a fault. You query it to audit
directions on a rail (which parts source it, which sink it) or to find pins the design left
untyped.

### For software engineers

`pin.type` is the **type annotation** on the pin member.
[The analogy guide](../../../../docsite/content/reference/analogy.md) calls pin directions type
annotations, and the connectivity rules dispatch on them. Rows are 1:1 with pins, one direction
each. An `unspecified` value is a real annotation rather than a missing row. It distinguishes
"declared as untyped" from "no such pin," and rules that key on it treat it as "the author
annotated nothing."
An empty result means the design carries no part-pin data.

### Go projector

`pinFacts` in `stdlib/relations/facts.go` calls `Model.PinDir(ref, des)` for each pin, passes it
through `check.DirString`, and emits a `pin.type(ref_des, pin, etype)` row. `PinDir` returns the direction the
reader recorded for that pin (last section wins for a multi-section part); `check.DirString` maps the
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

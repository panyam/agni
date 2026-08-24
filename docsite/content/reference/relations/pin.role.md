---
title: "pin.role"
description: "a pin's derived role (power/ground/anode/cathode)"
---

### What it is

`pin.role(ref_des, pin, role)` yields the derived semantic role of a pin, where `role` is one of
`power`, `ground`, `anode`, `cathode`, `gate`, `source`, or `drain`. The role is inferred from the pin's declared name, gated
by the component's device class. A row is emitted only when a role is derived; a pin whose role
cannot be determined produces no row (the role is never guessed).

### For hardware engineers

This is the tool reading a pin's name the way you would: `VCC`/`VDD` on any part is a power pin,
`GND`/`VSS` is ground, and `A`/`K` on a diode-family part are anode and cathode. The polarity
roles are class-gated on purpose. A pin named `A` on an IC is a signal, not an anode, so anode and
cathode are derived only for diodes, LEDs, TVS, and Zeners.

`gate`, `source` and `drain` (WS3-117) are gated the same way and for a sharper version of the same
reason: they are the shortest pin names on a board. A bare `G`, `S` or `D` means something on almost
every part, so they are derived only where the class lexicon reads the component as a **transistor**,
and the patterns are whole-name anchored so `SDA` is not a source and `DIR` is not a drain. A wrong
role is worse than a missing one here, because a topology rule would then walk a path that does not
exist and report on it.

Unlike the polarity tokens, the terminal roles read from the naming lexicon rather than engine
literals, so a house that calls its gate `DRV` declares that under `lexicon.pin.gate` in `--conventions`
instead of patching the engine. Matching is exact-token, not substring:
`CLKA` is not an anode. You query it to find, say, every cathode and check where it lands, or to
confirm the polarity roles resolved on the parts you expect.

### For software engineers

`pin.role` is a **derived tag** over the pin member: not a stored field, but a classification
computed from the pin name plus the owning instance's class. Rows are at most 1:1 with a pin, and
only for pins that classify. An absent row means the role is unknown, the honest default. This
"omit when unknown, never guess" shape is why a query counting roles never sees a phantom
`unknown` value: the projection simply skips it.

### Go projector

`pinFacts` in `check/facts.go` calls `Model.PinRole(ref, des)` for each pin and emits a
`pin.role(ref_des, pin, role)` row only when the result is not `RoleUnknown`. `PinRole` runs
`classifyPinRole`, which upper-cases the pin name, applies the anode/cathode vocabulary when the
component class is in the diode family (`diode`, `led`, `tvs`, `zener`), and otherwise maps
ground-names to `ground` and rail-names to `power`. At most one row per pin; empty when no pin
carries a recognizable role name (or the design has no part-pin data).

### Datalog

Every pin with a derived role:

```
pin.role(?r, ?p, ?role) => ?role
```

Join to `pin.net` to find the net each cathode lands on (the polarity roles gated to the diode
family):

```
pin.role(?r, ?p, "cathode"), pin.net(?r, ?p, ?n) => ?n
```

### Schematic

![A diode's A and K pins resolve to anode and cathode roles; a resistor's passive pins yield no role row]({{.Site.PathPrefix}}/static/images/catalog/relations/pin.role.svg)

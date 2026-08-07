---
title: "intent/property-ac-coupled"
description: "A net the design intent declares AC-coupled is carried by no series capacitor."
---

### What it checks

A net the design intent declares as AC-coupled must actually be carried by a **series capacitor**. If
no series capacitor is on it, the net is DC-connected and the declaration is unmet.

### For hardware engineers

AC coupling puts a capacitor in the signal path so the alternating signal passes while any steady DC
level does not. It is standard on high-speed serial links (PCIe, SGMII, USB SuperSpeed) because the
two ends often sit at different common-mode voltages by design. The transmitter idles at one level,
the receiver expects another, and the capacitor lets each keep its own without fighting.

Leave it out and the two ends are tied together at DC. The receiver's input sits wherever the
transmitter's output holds it rather than at its own bias point, which shows up as a link that either
does not train or trains and then errors under load. It is also a part-count-of-one omission, easy to
lose in a respin.

### How the check tells a coupling cap from a decoupling cap

Both are "a capacitor on the net", and the difference is what the far side connects to.

- **Decoupling**: the far side is ground (or a rail). The cap shunts noise; the signal does not pass
  through it.
- **Coupling**: the far side is another signal net. The signal passes through it.

So the rule looks for a capacitor on the declared net whose *other* net is neither ground nor a power
rail. That is the structural difference between the two uses of the same component.

### What a pass means here

This property is **decidable** from the netlist: a series capacitor is either present or it is not. So
a pass means the declaration is met, not merely "no contradiction found".

That is worth stating because its sibling `property-reset-polarity` is different — read that card
before assuming the two behave alike.

### Declaring it

```yaml
net_properties:
  - {net: PCIE_TX0_P, property: ac-coupled}
```

No `value`; the kind is the whole assertion. A net you do not declare is not checked — the rule
iterates the declaration, never the design, so it has no opinion about nets your intent is silent on.

### Fixing a finding

Either the coupling capacitor is missing from the schematic, or the declaration names the wrong net.
Check which before adding a part: on a differential pair it is easy to declare `_P` and wire `_N`.

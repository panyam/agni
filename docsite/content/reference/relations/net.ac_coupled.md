---
title: "net.ac_coupled"
description: "a SERIES capacitor carries the net (a decoupling cap to ground/rail does not count)"
---

### What it is

`net.ac_coupled(net)` yields one row per net carried by a **series** capacitor. When a net's only
capacitors return to ground or a rail, the net produces no row.

### For hardware engineers

AC coupling puts a capacitor in the signal path so the alternating signal passes while any steady DC
level does not. It is standard on high-speed serial links (PCIe, SGMII, USB SuperSpeed) because the
two ends deliberately sit at different common-mode voltages. The transmitter idles at one level and
the receiver expects another, and the capacitor lets each keep its own.

Leave it out and the two ends are tied together at DC. The receiver sits wherever the transmitter
holds it rather than at its own bias point. That shows up as a link that will not train, or one that
trains and then errors under load.

### Coupling versus decoupling

Both uses of a capacitor are "a capacitor on the net". The difference is the far side.

| | far side | signal passes through it? |
|---|---|---|
| **Decoupling** | ground or a rail | no, it shunts noise |
| **Coupling** | another signal net | yes, it carries the signal |

So this relation looks for a capacitor whose *other* net is neither ground nor a power rail. Without
that test, a predicate would mark nearly every net on the board as AC-coupled, since most carry a
bypass cap.

### For software engineers

It is a filtered projection over `Nets()`, 1:1 with coupled nets. It is empty on a board with no
series caps, which is the common case for a low-speed design. That empty result is a genuine answer
rather than a gap.

### Go projector

`netACCoupledFacts` in `stdlib/relations/facts.go` calls `check.ACCoupled` in
`core/check/guards.go`. The intent rule `intent/property-ac-coupled` reads the same function, so a
declared-versus-actual comparison and an ad-hoc query cannot disagree about what counts as coupling.

### Datalog

Every AC-coupled net:

```
net.ac_coupled(?n) => ?n
```

The parts on them, for a quick read of what a link actually connects:

```
net.ac_coupled(?n), component.net(?r, ?n) => ?n, ?r
```

High-speed pairs that are NOT coupled, the shape a link-integrity review asks about:

```
component.net(?r, ?n), str.suffix(?n, "_TXP"), not net.ac_coupled(?n) => ?n, ?r
```

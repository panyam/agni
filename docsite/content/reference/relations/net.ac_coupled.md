---
title: "net.ac_coupled"
description: "a SERIES capacitor carries the net (a decoupling cap to ground/rail does not count)"
---

### What it is

`net.ac_coupled(net)` yields one row per net carried by a **series** capacitor. A net whose only
capacitors return to ground or a rail produces no row.

### For hardware engineers

AC coupling puts a capacitor in the signal path so the alternating signal passes while any steady DC
level does not. It is standard on high-speed serial links (PCIe, SGMII, USB SuperSpeed) because the
two ends deliberately sit at different common-mode voltages: the transmitter idles at one level, the
receiver expects another, and the capacitor lets each keep its own.

Leave it out and the two ends are tied together at DC. The receiver sits wherever the transmitter
holds it rather than at its own bias point, which shows up as a link that will not train, or trains
and then errors under load.

### The distinction that is the whole predicate

Both uses of a capacitor are "a capacitor on the net". The difference is the far side.

| | far side | signal passes through it? |
|---|---|---|
| **Decoupling** | ground or a rail | no — it shunts noise |
| **Coupling** | another signal net | yes — it carries the signal |

So this relation looks for a capacitor whose *other* net is neither ground nor a power rail. A
predicate that skipped that test would mark nearly every net on the board as AC-coupled, since most
carry a bypass cap.

### For software engineers

A filtered projection over `Nets()`, 1:1 with coupled nets. Empty on a board with no series caps,
which is the common case for a low-speed design and is a genuine answer rather than a gap.

### Go projector

`netACCoupledFacts` in `stdlib/relations/facts.go` calls `check.ACCoupled` in
`core/check/guards.go`. The intent rule `intent/property-ac-coupled` reads the same function, so a
declared-versus-actual comparison and an ad-hoc query cannot disagree about what counts as coupling.

### Datalog

Every AC-coupled net:

```
net.ac_coupled(?n) => ?n
```

The parts on them, which is the quick read of what a link actually connects:

```
net.ac_coupled(?n), component-on-net(?r, ?n) => ?n, ?r
```

High-speed pairs that are NOT coupled, the shape a link-integrity review asks about:

```
component-on-net(?r, ?n), suffix(?n, "_TXP"), not net.ac_coupled(?n) => ?n, ?r
```

---
title: "Series and shunt"
label: "series or shunt"
summary: "The two ways a two-terminal part sits on a net. A series part joins two nets and the signal passes through it. A shunt part hangs off one net to ground or a rail, beside the signal rather than in its path."
level: EE3
---

Picture a capacitor with one end on a data line. Whether it passes the data or kills the noise depends
entirely on where its other end goes.

If the other end goes to another signal net, the part is **in series**. It joins two nets, and
anything travelling from one to the other passes through it. A coupling capacitor, a ferrite bead, a
fuse, a series resistor and a diode in a power path all sit this way.

If the other end goes to ground or a rail, the part is a **shunt**. It hangs off one net, and the
signal goes past it rather than through it. A decoupling capacitor, a TVS diode and a pull-down all
sit this way.

{{ includeFile "figures/series-and-shunt.svg" }}

The part's type doesn't settle the question, so the engine reads the far end. Two places in agni
depend on getting it right.

1. [`property-ac-coupled`](../../rules/intent-property-ac-coupled/) has to tell a coupling cap from a
   decoupling cap. Both are a capacitor on the declared net. The rule looks for one whose other net is
   neither ground nor a rail.
2. [`reverse-blocking-absent`](../../rules/reverse-blocking-absent/) looks for a diode in series
   between a connector and a power input. A freewheel diode with its anode on ground is a shunt
   clamp. The rule once read one as a series diode fitted backwards, and flagged outputs that were
   correctly protected (agni issue 63).

Protection sorts the same way. A fuse and a series diode are in series, and a TVS is a shunt. A TVS
clamps a spike to ground and blocks nothing, so its presence says nothing about whether current can
flow backwards.

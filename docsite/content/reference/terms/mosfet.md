---
title: "MOSFET"
label: "MOSFET"
summary: "A transistor whose gate turns it on while drawing almost no current, and whose drain and source form the switch. It always carries a body diode between drain and source."
level: EE3
---

A load switch turns a rail on and off for one part of a board. Inside it, or beside it, sits a
MOSFET, and every rule about load switches is a statement about one of its three terminals.

{{ includeFile "figures/mosfet.svg" }}

The **gate** is insulated from the rest of the part, so it draws almost no current. Its voltage
relative to the source decides whether the switch conducts. An N-channel part turns on when the gate
sits a few volts above the source. A P-channel part turns on when the gate sits below it.

The **drain** and **source** are the two ends of the switch. With the gate on, current flows
between them through a small resistance. With it off, the part holds off the voltage across them,
up to its rating. A datasheet gives that limit as VDSS, and above it the part breaks down.
[`fet-vdss-below-switched-rail`](../../rules/fet-vdss-below-switched-rail/) checks the switched
rail against it. [`load-switch-trip-above-fet-rating`](../../rules/load-switch-trip-above-fet-rating/)
checks a load switch's trip current against the FET's continuous rating.

The **body diode** comes with the part, whether or not the schematic draws it. It runs from source
to drain in an N-channel part and from drain to source in a P-channel one. It conducts whenever its
anode sits above its cathode, gate on or off. That makes a MOSFET a one-way blocker only when it is
oriented with that diode in mind, and an
[ideal diode](../ideal-diode/) uses exactly that.

A logic input is a MOSFET gate too, so it draws almost nothing, and an undriven one floats rather
than settling.

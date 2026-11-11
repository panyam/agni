---
title: "Ideal diode"
label: "ideal diode"
summary: "A MOSFET and a small controller that together act like a diode with almost no forward drop. The controller turns the transistor on while current flows forward and off when it would flow back."
level: EE4
---

A board takes power from either a wall adapter or a USB port, whichever is present. Each source
feeds the rail through something one-way, so the adapter cannot push current back into the USB host
and the USB host can't push it into the adapter. That arrangement is called ORing.

A diode does the job, and even a Schottky costs about 0.4 V of drop, against about 0.7 V for a
silicon one. At a few amps that is a hot part and a lower rail. An ideal diode does it with a [MOSFET](../mosfet/) and a controller instead.

{{ includeFile "figures/ideal-diode.svg" }}

The controller watches both sides. While the input sits above the load, it turns the transistor on,
and the drop falls to tens of millivolts. When the load rises above the input, it turns the
transistor off. The transistor sits with its body diode pointing from input to load, so the off
state blocks backwards current too. The LTC4412 on the tutorial's power sheet is one such
controller.

A netlist shows none of that intent. It shows a transistor in a power path with a chip beside it,
and the same shape can be a load switch, a level shifter or an ideal diode. So
[`reverse-blocking-absent`](../../rules/reverse-blocking-absent/) counts a series diode as
blocking, flags a path with nothing one-way on it, and reports a path through an unidentified
transistor as inconclusive. It doesn't guess either way. A guess against an ideal diode would fail
every well-designed ORing board, and a guess for one would pass a board with no protection.

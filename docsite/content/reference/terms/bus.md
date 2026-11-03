---
title: "Bus"
label: "bus"
summary: "A set of nets that several parts share, carrying one interface whose requirements come from a standard rather than from anything a netlist can show."
level: EE2
---

A point-to-point link joins two pins. A bus is a set of nets that more than two parts connect to, so
every part on it sees every transition and the parts take turns talking. That sharing is the point of
a bus. Because of it, a standard has a lot to say about a bus. The parts have to agree on levels,
speed, addressing, and who is allowed to drive when.

{{ includeFile "figures/bus.svg" }}

The word does two jobs on this site, and each is checked somewhere different.

**On a schematic it is drawing shorthand.** `DATA[7:0]` is one line standing for eight signals.
Downstream, every member has to come out of the read as its own net. A diff, a
highlight, or any rule over those signals is unreliable until it does.
[`bus-not-modeled`](../../rules/bus-not-modeled/) is that check, and it reports on the reader rather
than on the design.

**As an interface it is a standard.** CAN, I2C and SPI each name their signals and demand things of
them. One wants a [termination](../termination/) resistor at each end of the run. An open-drain bus
wants a [pull-up](../pull-up/) on every line. And a [transceiver](../transceiver/) sits between the
processor's logic pins and whatever voltages the standard actually uses. A two-wire bus is usually a
[differential pair](../differential-pair/), for reasons of noise rather than of protocol.

None of that is visible in a netlist as a requirement. A board with CAN wired wrongly is a
perfectly valid netlist, so an interface profile declares the shape and the engine compiles rules from
the declaration. [`profile-signal-missing`](../../rules/profile-signal-missing/) reports a role that
no net carries, and [`profile-signal-dangling`](../../rules/profile-signal-dangling/) reports one
wired at a single end. [`profile-termination`](../../rules/profile-termination/) checks the resistor
across the pair.
I2C is common enough to be checked without a profile at all, by
[`i2c-pull-up`](../../rules/i2c-pull-up/).

The course teaches it in
[chapter 10](../../../learn/10-interfaces-and-what-they-require/#a-bus-is-a-contract-ee6), which
argues that a bus is a contract, and in [chapter 1](../../../learn/01-what-a-board-is-made-of/),
which reads a terminator off the two nets it bridges.

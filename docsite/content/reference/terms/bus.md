---
title: "Bus"
label: "bus"
summary: "A set of nets that several parts share, carrying one interface whose requirements come from a standard rather than from anything a netlist can show."
level: EE2
---

A point-to-point link joins two pins. A bus is a set of nets that more than two parts connect to, so
every part on it sees every transition and the parts take turns talking. That sharing is the point of
a bus. It is also why a standard has so much to say about one, since the parts have to agree on
levels, speed, addressing, and who is allowed to drive when.

{{ includeFile "figures/bus.svg" }}

The word does two jobs on this site, and each is checked somewhere different.

**On a schematic it is drawing shorthand.** `DATA[7:0]` is one line standing for eight signals, and
what matters downstream is that every member comes out of the read as its own net. A diff, a
highlight, or any rule over those signals is unreliable until it does.
[`bus-not-modeled`](../../rules/bus-not-modeled/) is that check, and it reports on the reader rather
than on the design.

**As an interface it is a standard.** CAN, I2C and SPI each name their signals and demand things of
them: a [termination](../termination/) resistor at each end of the run, a [pull-up](../pull-up/) on
every line an open-drain bus uses, a [transceiver](../transceiver/) between the processor's logic pins
and whatever voltages the standard actually uses. A two-wire bus is usually a
[differential pair](../differential-pair/), for reasons of noise rather than of protocol.

None of that is visible in a netlist as a requirement. A board with CAN wired wrongly is a
perfectly valid netlist, so an interface profile declares the shape and the engine compiles rules from
the declaration: [`profile-signal-missing`](../../rules/profile-signal-missing/) for a role no net
carries, [`profile-signal-dangling`](../../rules/profile-signal-dangling/) for one wired at a single
end, and [`profile-termination`](../../rules/profile-termination/) for the resistor across the pair.
I2C is common enough to be checked without a profile at all, by
[`i2c-pull-up`](../../rules/i2c-pull-up/).

**Where the course teaches it:**
[chapter 10](../../../learn/10-interfaces-and-what-they-require/#a-bus-is-a-contract-ee6) is the
argument that a bus is a contract, and
[chapter 1](../../../learn/01-what-a-board-is-made-of/) reads a terminator off the two nets it
bridges.

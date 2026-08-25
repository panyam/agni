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

<svg viewBox="0 0 680 270" role="img" aria-labelledby="bus-title" style="width:100%;height:auto;font-family:inherit"><title id="bus-title">A two-wire bus running left to right with three nodes tapping the same pair of nets, and a terminating resistor bridging the pair at each end of the run</title><g fill="currentColor" font-size="13" text-anchor="middle" opacity="0.75"><text x="340" y="30">one set of nets, shared by every part on it</text></g><g fill="none" stroke="currentColor" stroke-width="2"><path d="M80 110 H600"/><path d="M80 146 H166 M174 146 H326 M334 146 H486 M494 146 H600"/><path d="M80 110 V146 M600 110 V146"/><path d="M170 110 V186 M190 146 V186 M330 110 V186 M350 146 V186 M490 110 V186 M510 146 V186"/></g><g fill="var(--accent-color)" fill-opacity="0.2" stroke="currentColor" stroke-width="1.6"><rect x="69" y="116" width="22" height="24"/><rect x="589" y="116" width="22" height="24"/></g><g fill="currentColor"><circle cx="170" cy="110" r="3.2"/><circle cx="330" cy="110" r="3.2"/><circle cx="490" cy="110" r="3.2"/><circle cx="190" cy="146" r="3.2"/><circle cx="350" cy="146" r="3.2"/><circle cx="510" cy="146" r="3.2"/></g><g fill="none" stroke="currentColor" stroke-width="1.6" opacity="0.85"><rect x="125" y="186" width="110" height="36"/><rect x="285" y="186" width="110" height="36"/><rect x="445" y="186" width="110" height="36"/></g><g fill="currentColor" font-size="13" text-anchor="middle"><text x="180" y="209">node 1</text><text x="340" y="209">node 2</text><text x="500" y="209">node 3</text></g><g fill="var(--accent-color)" font-size="11.5" font-weight="600" text-anchor="middle"><text x="80" y="176">termination</text><text x="600" y="176">termination</text></g><g fill="currentColor" font-size="11.5" text-anchor="middle" opacity="0.6"><text x="340" y="252">the terminators sit at the two ends of the run, not at every node</text></g></svg>

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

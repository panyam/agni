---
title: "Ground"
label: "ground"
summary: "The net every other voltage on a board is measured against, and the path the current takes back to whatever supplied it."
level: EE2
---

Two things at once, and both are load-bearing. Ground is the reference: saying a rail is at 3.3 V is
shorthand for saying it is 3.3 V above ground, and there is no other sense in which a single point has
a voltage. Ground is also the return: current that leaves a regulator through a rail has to get back,
and ground is how.

<svg viewBox="0 0 660 254" role="img" aria-labelledby="ground-title" style="width:100%;height:auto;font-family:inherit"><title id="ground-title">A regulator feeding three parts along a 3V3 rail, with every part also returning to a wide ground bar below, and the 3.3 V measured as the difference between the two</title><rect x="56" y="190" width="560" height="16" fill="currentColor" opacity="0.18"/><line x1="96" y1="64" x2="616" y2="64" stroke="var(--accent-color)" stroke-width="2"/><g fill="none" stroke="currentColor" stroke-width="1.4" opacity="0.8"><rect x="56" y="100" width="88" height="52"/><rect x="224" y="100" width="64" height="52"/><rect x="364" y="100" width="64" height="52"/><rect x="504" y="100" width="64" height="52"/><path d="M100 100 V64"/><path d="M100 152 V190"/><path d="M256 100 V64"/><path d="M256 152 V190"/><path d="M396 100 V64"/><path d="M396 152 V190"/><path d="M536 100 V64"/><path d="M536 152 V190"/></g><g fill="currentColor" font-size="12" text-anchor="middle"><text x="100" y="130">regulator</text><text x="256" y="130">U3</text><text x="396" y="130">U4</text><text x="536" y="130">C1</text></g><text x="96" y="52" font-size="12" font-weight="600" fill="var(--accent-color)">3V3 rail</text><text x="560" y="52" text-anchor="end" font-size="11" fill="currentColor" opacity="0.6">current out</text><text x="68" y="202" font-size="12" font-weight="600" fill="currentColor">GND</text><text x="560" y="202" text-anchor="end" font-size="11" fill="currentColor" opacity="0.75">current back</text><line x1="590" y1="64" x2="590" y2="190" stroke="var(--accent-color)" stroke-width="1.4"/><polygon points="590,64 586,72 594,72" fill="var(--accent-color)"/><polygon points="590,190 586,182 594,182" fill="var(--accent-color)"/><text x="596" y="131" font-size="11.5" font-weight="600" fill="var(--accent-color)">3.3 V</text><text x="330" y="232" text-anchor="middle" font-size="11.5" fill="currentColor" opacity="0.6">every voltage on the board is a difference measured against this net</text></svg>

The consequence for reading a schematic is that ground touches nearly everything. A rail feeds the
parts that want its voltage; ground is on almost every part on the board, which is why it is drawn as
a scatter of little triangles rather than as a wire. On a real board it is not a wire at all but a
copper plane, and the drawing's twenty separate symbols are one net.

The engine treats it as its own thing rather than as a rail at zero volts, in two places worth
knowing. Ground names come from a vocabulary of their own, separate from the rail vocabulary, so a
project that spells it `AGND` or `0V` declares that alongside its rail patterns. And ground stops the
series-reach walk. A walk that traced connectivity through passives would arrive at ground on its
first hop and from there reach the entire design, which is true and useless, so ground counts as
shared distribution rather than as a path.

The rules that read it are mostly about whether it is claimed correctly.
[`dl-power-pin-mistyped`](../../rules/dl-power-pin-mistyped/) catches a pin named `GND` whose symbol
never typed it as a supply, sitting alone on a stub because nothing merged it into the plane.
[`power-tap-conflict`](../../rules/power-tap-conflict/) fires when one net carries two design-wide
names, which for ground usually means two spellings that a human reads as obviously the same and a
tool must not. [`unconnected-pin`](../../rules/unconnected-pin/) and
[`single-pin-net`](../../rules/single-pin-net/) pick up the part that never made it there at all.

**Where the course teaches it:**
[chapter 1](../../../learn/01-what-a-board-is-made-of/) uses it as one end of the two-terminal
decision procedure, and
[chapter 5](../../../learn/05-who-drives-this-net/#one-net-one-decider-ee3) explains why an output pin
deciding a net's voltage means connecting it either to a [rail](../rail/) or to ground.

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

{{ includeFile "figures/ground.svg" }}

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

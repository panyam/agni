---
title: "Via"
label: "via"
summary: "A plated hole through the board that joins copper on one layer to copper on another, so a net can change layers."
level: EE7
---

A board carries copper on several layers, and a track lives on exactly one of them. A via is the hole
that lets a net cross between them. It is drilled through the board, plated on the inside with copper,
and landed on a pad on each layer it joins.

{{ includeFile "figures/via.svg" }}

Three numbers describe one, and each is a separate promise. The **drill** is the hole the fabricator
makes. The **pad** is the copper ring, and the drill goes inside it. The **annular ring** is the
copper left of that pad once the drill is subtracted, half the pad diameter minus half the drill. A
fourth fact is its **span**, which says whether it runs the whole thickness of the board or only
between two of the inner layers.

A via is more likely than any other feature to be built differently from how it was drawn, so those
numbers get checked. Drills wander inside their tolerance, and a ring too thin to absorb that wander
lets the barrel break out of the edge of its pad. The board still passes visual inspection and the
connection still works on the bench, and then it goes intermittent in the field. A drill smaller
than the fab's smallest bit fails more quietly. The order is either rejected outright or the hole is
silently upsized, and every clearance around it moves with it.

Three rules read a via, and they answer to two different authorities.
[`hole-size`](../../rules/hole-size/) and [`annular-width`](../../rules/annular-width/) compare the
drill and the ring against the loosest mainstream fabrication floor, so a finding means the board
may not be manufacturable at all. [`netclass-via-drill`](../../rules/netclass-via-drill/) compares
the same drill against the size declared by the net's own class, so a finding there means the board
is buildable and is not what the design asked for. The same distinction runs through the geometric
rules generally. [`track-width`](../../rules/track-width/) and
[`copper-clearance`](../../rules/copper-clearance/) ask what a process can hold, while
[`netclass-track-width`](../../rules/netclass-track-width/) asks what the project said it wanted.

A component's through-hole is a related but separate feature. A resistor's leg sits in a hole
drilled and plated the same way. The difference is that a via carries no part and exists only to
change layers. `hole-size` covers vias alone today, because checking a component pad's drill needs
facts that the geometry sidecar does not carry yet.

None of this is visible in a netlist. Two revisions can agree on every connection and disagree on every
via, because the graph does not know the copper has a position or a width.

The course teaches it in [chapter 12](../../../learn/12-when-the-copper-matters/), and the whole
chapter is about it.
[What geometry can answer](../../../learn/12-when-the-copper-matters/#what-geometry-can-answer-ee7)
runs the three geometric rules over a board.
[A fourth kind of authority](../../../learn/12-when-the-copper-matters/#a-fourth-kind-of-authority-ee7)
covers the fab-versus-net-class distinction on its own.

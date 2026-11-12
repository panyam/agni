---
title: "Netlist"
label: "netlist"
summary: "The list of nets a CAD tool exports, each naming the pins that share it. The board gets built from it, and it carries none of the drawing."
level: EE2
---

A list of nets. Each net has a name and the set of pins that sit on it, and that is very nearly the
whole data structure. Everything a board house needs in order to build the thing is in there, and
everything about how the schematic looked is not.

{{ includeFile "figures/netlist.svg" }}

A tool reading the netlist is worth having because of that gap. The picture on screen is one
rendering of this list, drawn by a human for other humans, and the two can disagree. Imagine a wire
that taps another with no junction dot. That leaves two separate nets, and every reviewer read them
as one. By the time the design is a netlist the dot has done its work and vanished, so the netlist
cannot tell you it was absent. Two separate nets look exactly like two separate nets.

So a handful of checks run inside the reader rather than over the netlist.
[`wire-no-junction`](../../rules/wire-no-junction/) and
[`dangling-endpoint`](../../rules/dangling-endpoint/) name a coordinate on a sheet. What they found
does not survive into the list of nets, so it has nothing else to be called.
[`duplicate-net-name`](../../rules/duplicate-net-name/) and
[`single-pin-net`](../../rules/single-pin-net/) work the other way round, reading the netlist for the
shapes a drawing error leaves behind in it.

Inside the engine, a netlist is also the unit that decides where a rule belongs. If a rule can be
computed from the final netlist IR alone, it is an analysis check and runs the same way on every input
format. If it needs something the reader normalized away, it is an input diagnostic and has to be
caught while the file is being parsed. The
[rules and checks page](../../../architecture/rules-and-checks/) works through that split.

The course teaches it in
[chapter 2](../../../learn/02-the-drawing-is-not-the-circuit/#the-drawing-is-a-rendering-ee2), which
opens on the distinction and spends the rest of the chapter on the ways a drawing and its netlist disagree.

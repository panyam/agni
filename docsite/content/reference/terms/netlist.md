---
title: "Netlist"
label: "netlist"
summary: "The list of nets a CAD tool exports, each naming the pins that share it. It is what the board gets built from, and it carries none of the drawing."
level: EE2
---

A list of nets. Each net has a name and the set of pins that sit on it, and that is very nearly the
whole data structure. Everything a board house needs in order to build the thing is in there, and
everything about how the schematic looked is not.

<svg viewBox="0 0 660 236" role="img" aria-labelledby="netlist-title" style="width:100%;height:auto;font-family:inherit"><title id="netlist-title">A schematic fragment with three parts joined at a junction dot, and beside it the same connection written as a netlist entry naming the three pins</title><g fill="currentColor" font-size="13" font-weight="600"><text x="10" y="20">The drawing</text><text x="360" y="20">The netlist</text></g><g fill="none" stroke="currentColor" stroke-width="1" opacity="0.3"><rect x="10" y="30" width="290" height="190"/><rect x="360" y="30" width="290" height="190"/></g><g fill="none" stroke="currentColor" stroke-width="1.4" opacity="0.85"><rect x="36" y="70" width="54" height="26"/><rect x="36" y="170" width="54" height="26"/><rect x="216" y="120" width="54" height="26"/><path d="M90 83 H152 V133"/><path d="M90 183 H152 V133"/><path d="M216 133 H152"/></g><circle cx="152" cy="133" r="4" fill="var(--accent-color)"/><g fill="currentColor" font-size="12.5" text-anchor="middle"><text x="63" y="88">R1</text><text x="63" y="188">U1</text><text x="243" y="138">C1</text></g><g fill="currentColor" font-size="11" opacity="0.7"><text x="96" y="78">R1.2</text><text x="96" y="178">U1.5</text><text x="212" y="127" text-anchor="end">C1.1</text></g><text x="152" y="112" text-anchor="middle" font-size="11.5" font-weight="600" fill="var(--accent-color)">VDD</text><g stroke="currentColor" stroke-width="1.4" opacity="0.7"><line x1="308" y1="125" x2="345" y2="125"/></g><polygon points="352,125 344,121 344,129" fill="currentColor" opacity="0.7"/><text x="330" y="112" text-anchor="middle" font-size="10.5" fill="currentColor" opacity="0.6">export</text><text x="376" y="72" font-size="12" fill="currentColor" opacity="0.7">what the CAD tool writes out</text><g fill="currentColor" font-size="12.5" font-family="ui-monospace, monospace"><text x="376" y="104">VDD:  R1.2  U1.5  C1.1</text><text x="376" y="128">GND:  U1.8  C1.2</text><text x="376" y="152" opacity="0.5">...</text></g><g fill="currentColor" font-size="11.5" text-anchor="middle" opacity="0.6"><text x="155" y="210">wires, dots and positions</text><text x="505" y="210">no wires, no dots, no coordinates</text></g></svg>

That gap is the whole reason a tool reading the netlist is worth having. The picture on screen is one
rendering of this list, drawn by a human for other humans, and the two can disagree. A junction dot
missing where a wire taps another leaves two separate nets that every reviewer read as one, and by the
time the design is a netlist the dot has done its work and vanished, so the netlist cannot tell you it
was absent. Two separate nets look exactly like two separate nets.

That is why a handful of checks run inside the reader rather than over the netlist.
[`wire-no-junction`](../../rules/wire-no-junction/) and
[`dangling-endpoint`](../../rules/dangling-endpoint/) name a coordinate on a sheet, because the thing
they found does not survive into the list of nets and has nothing else to be called.
[`duplicate-net-name`](../../rules/duplicate-net-name/) and
[`single-pin-net`](../../rules/single-pin-net/) work the other way round, reading the netlist for the
shapes a drawing error leaves behind in it.

Inside the engine, a netlist is also the unit that decides where a rule belongs. If a rule can be
computed from the final netlist IR alone, it is an analysis check and runs the same way on every input
format. If it needs something the reader normalized away, it is an input diagnostic and has to be
caught while the file is being parsed. The
[rules and checks page](../../../architecture/rules-and-checks/) works through that split.

**Where the course teaches it:**
[chapter 2](../../../learn/02-the-drawing-is-not-the-circuit/#the-drawing-is-a-rendering-ee2) opens on
the distinction and spends the rest of the chapter on the ways a drawing and its netlist disagree.

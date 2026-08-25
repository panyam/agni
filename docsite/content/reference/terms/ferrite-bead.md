---
title: "Ferrite bead"
label: "ferrite bead"
summary: "A two-terminal part that passes DC almost freely and becomes a lossy resistance at high frequency, so noise travelling along a rail or a signal is turned into heat rather than reflected."
level: EE3
---

A bead of ferrite material with a conductor running through it. On a schematic it looks like an
inductor, and at low frequency it behaves like one. At DC it is a few tens of milliohms, near enough
to a piece of wire that the rail through it does not care. Somewhere in the tens of megahertz it
becomes a resistance of tens or hundreds of ohms, and that resistance is lossy on purpose. Energy
arriving there is dissipated as heat instead of being reflected back down the line, which is what a
plain inductor would do with it.

<svg viewBox="0 0 680 290" role="img" aria-labelledby="ferrite-title" style="width:100%;height:auto;font-family:inherit"><title id="ferrite-title">Impedance of a ferrite bead against frequency on a logarithmic axis: near zero ohms at 1 MHz, rising through about 100 ohms at 100 MHz, broadly flat to 1 GHz</title><g fill="currentColor" font-size="11" opacity="0.7"><text x="90" y="26">impedance</text><text x="616" y="272" text-anchor="end">frequency</text></g><g stroke="currentColor" stroke-width="1" opacity="0.15" stroke-dasharray="3 4"><path d="M90 40 H610 M90 103 H610 M90 167 H610"/></g><g stroke="currentColor" stroke-width="1.4" fill="none" opacity="0.7"><path d="M90 34 V230 M90 230 H616"/></g><g fill="currentColor" font-size="11.5" text-anchor="end" opacity="0.75"><text x="82" y="234">0 &#937;</text><text x="82" y="171">50 &#937;</text><text x="82" y="107">100 &#937;</text><text x="82" y="44">150 &#937;</text></g><g fill="currentColor" font-size="11.5" text-anchor="middle" opacity="0.75"><text x="90" y="250">1 MHz</text><text x="263" y="250">10 MHz</text><text x="437" y="250">100 MHz</text><text x="610" y="250">1 GHz</text></g><path d="M90 227 C150 222 200 205 263 190 C320 170 380 132 437 112 C468 101 492 95 520 95 C558 95 586 104 610 116" fill="none" stroke="var(--accent-color)" stroke-width="2.5"/><g stroke="currentColor" stroke-width="1" opacity="0.4"><path d="M150 158 V214 M406 72 V116"/></g><g stroke="var(--accent-color)" stroke-width="1.2" opacity="0.5" stroke-dasharray="3 3"><path d="M437 230 V112"/></g><circle cx="437" cy="112" r="3.5" fill="var(--accent-color)"/><g fill="currentColor" font-size="11.5" opacity="0.75"><text x="96" y="150">a near short at DC</text><text x="330" y="64">the noise turns into heat</text></g><text x="449" y="146" font-size="11.5" font-weight="600" fill="var(--accent-color)">about 100 &#937; at 100 MHz</text></svg>

The usual job is to stop one part of a board polluting another. A switching regulator makes a rail
carrying tens of millivolts of ripple at its switching frequency and every harmonic above it, which is
fine for logic and not fine for an ADC reference. A bead in series between the two, with decoupling on
each side of it, gives that noise somewhere to go while the quiet side keeps the DC it needs.

Two things about a bead catch people out.

**It is not protection.** A bead passes DC, and it passes a fault current just as happily, so one
sitting between a connector and a regulator input protects nothing. That case is what
[`input-protection`](../../rules/input-protection/) exists to flag. The rule walks out from each
connector net through series pass elements looking for a fuse or a clamp, and a path whose only
inline part is a bead leaves the finding standing.

**It splits one logical rail into two nets.** The noisy side and the quiet side have different names
after the bead, so a rule reasoning about one net at a time cannot see across it. The engine answers
that with a reachability walk that crosses two-terminal series parts (resistor, inductor, ferrite,
fuse) rather than stopping at each one. That walk is how
[`profile-termination`](../../rules/profile-termination/) still finds a split terminator that no
single net touches directly. The same boundary is why
[`floating-input`](../../rules/floating-input/) goes quiet on any net carrying a bead. Claiming that
an input floats when a passive sits beside it is guesswork rather than a finding.

**Where the course teaches it:**
[chapter 1](../../../learn/01-what-a-board-is-made-of/#the-decision-procedure-ee3) puts a ferrite in
the table of what a two-terminal part is doing, and
[the recurring jobs](../../../learn/01-what-a-board-is-made-of/#the-recurring-jobs-ee3) files it
under taming a fast edge.

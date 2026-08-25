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

{{ includeFile "figures/ferrite-bead.svg" }}

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

---
title: "The seven levels"
description: "What each level of hardware knowledge means, how to tell you have it, and which sections of the course operate there."
---

The [course](../) is organised by topic, because a topic is what a chapter can be about. A chapter takes one subject up the ladder, which is how [chapter 3](../03-why-every-chip-needs-capacitors/) can show the same capacitor as a role, a failure mode, a number and a layout problem in one sitting.

This page is the other axis. Each level is defined here, with a test for whether you have it and a map of every section in the course that operates there. Read the course by topic, or read one level across the whole course.

**The levels are a difficulty scale, not a syllabus.** Nobody climbs them uniformly. A working engineer is at EE7 on the two subsystems they own and EE3 on the rest of the board, which is the ordinary shape of the job rather than a gap in it. Their purpose is to calibrate an explanation: "explain `output-output-conflict` at EE4" asks for the bench symptom rather than the definition, in the same way ELI15 asks for a register.

## Parts (EE1)

**The question you can answer:** what is that component?

Ohm's law, and what a resistor, capacitor, inductor, diode and transistor each do on their own. This is the floor the course assumes rather than teaches, and most people arriving from software already have it.

- [A board is a few kinds of thing](../01-what-a-board-is-made-of/#a-board-is-a-few-kinds-of-thing-ee1), the whole vocabulary, and how short it is

## Nets (EE2)

**The question you can answer:** are these two pins connected?

The drawing is not the circuit. A schematic is a rendering of a netlist, several things that look like connections are not, and several connections are invisible. Checked by the `integrity` category and the structural half of `connectivity`.

- [The same part, different jobs](../01-what-a-board-is-made-of/#the-same-part-different-jobs-ee2), reading a part from what it connects to
- [The drawing is a rendering](../02-the-drawing-is-not-the-circuit/#the-drawing-is-a-rendering-ee2), what actually gets built
- [One dot changes the circuit](../02-the-drawing-is-not-the-circuit/#one-dot-changes-the-circuit-ee2), a junction dot, and the netlist without it
- [Names are wires too](../02-the-drawing-is-not-the-circuit/#names-are-wires-too-ee2), why renaming a net rewires a board
- [Wires that reach nothing](../02-the-drawing-is-not-the-circuit/#wires-that-reach-nothing-ee2)
- [What the reader can and cannot see](../02-the-drawing-is-not-the-circuit/#what-the-reader-can-and-cannot-see-ee2), why these checks live in the reader

## Roles (EE3)

**The question you can answer:** why is *that* resistor there?

Every part has a job, and there are about twenty recurring ones. This is the unlock: most of `connectivity` and `power` stops looking like arbitrary rules once you can name what a part is for.

- [The decision procedure](../01-what-a-board-is-made-of/#the-decision-procedure-ee3), a lookup table for any two-terminal part
- [The recurring jobs](../01-what-a-board-is-made-of/#the-recurring-jobs-ee3), the twenty, with the rules attached to each
- [The role](../03-why-every-chip-needs-capacitors/#the-role-ee3), why a chip needs charge locally
- [A crystal is not an oscillator](../11-crystals-and-oscillators/#a-crystal-is-not-an-oscillator-ee3), where the oscillator actually is
- [What the capacitors decide](../11-crystals-and-oscillators/#what-the-capacitors-decide-ee3), load capacitance and frequency
- [Presence is checkable, value is not](../11-crystals-and-oscillators/#presence-is-checkable-value-is-not-ee3)
- [Nothing is a voltage nothing has](../04-pull-ups-and-undefined-states/#nothing-is-a-voltage-nothing-has-ee3), why a floating input is not zero
- [Some pins can only pull one way](../04-pull-ups-and-undefined-states/#some-pins-can-only-pull-one-way-ee3), open-drain
- [One net, one decider](../05-who-drives-this-net/#one-net-one-decider-ee3), and what two deciders costs
- [Two nets, opposite problems](../05-who-drives-this-net/#two-nets-opposite-problems-ee3)
- [Pins the designer said to leave alone](../05-who-drives-this-net/#pins-the-designer-said-to-leave-alone-ee3)
- [Most parts do not care](../06-parts-that-care-which-way-round/#most-parts-do-not-care-ee3), which parts have an orientation at all
- [Here is one, wired backwards](../06-parts-that-care-which-way-round/#here-is-one-wired-backwards-ee3)
- [What the tool has to know](../06-parts-that-care-which-way-round/#what-the-tool-has-to-know-ee3), the two facts a polarity check needs

## Failure modes (EE4)

**The question you can answer:** how does this break, and what does the bench see?

A defect has a symptom, not just a state. Some faults never appear at power-on, which is what makes them expensive. This is the level at which severity, the `Impact` field and triage become readable.

- [The failure mode](../03-why-every-chip-needs-capacitors/#the-failure-mode-ee4), why missing decoupling is a warning
- [Why this one is an error](../04-pull-ups-and-undefined-states/#why-this-one-is-an-error-ee4), and why the other is not
- [The same fault on a rail](../05-who-drives-this-net/#the-same-fault-on-a-rail-ee4), plus why a passing rule proves nothing
- [The same wiring, on a different part, is correct](../06-parts-that-care-which-way-round/#the-same-wiring-on-a-different-part-is-correct-ee4), why a topology is not a verdict

## Numbers (EE5)

**The question you can answer:** is this within spec, and which datasheet row says so?

Absolute maximum against recommended operating, derating, tolerance, worst case. A claim you cannot trace to a row in a document is an opinion. Checked by the `datasheet` category over the parameter layer.

- [The numbers](../03-why-every-chip-needs-capacitors/#the-numbers-ee5), voltage rating, derating, and DC bias
- [Choosing the resistor](../04-pull-ups-and-undefined-states/#choosing-the-resistor-ee5), which way a pull-up can be wrong, in both directions
- [More is worse](../11-crystals-and-oscillators/#more-is-worse-ee5), why adding load capacitors can be the defect
- [A part is a contract](../07-reading-a-datasheet/#a-part-is-a-contract-ee5), the type-signature model
- [Two numbers that look alike](../07-reading-a-datasheet/#two-numbers-that-look-alike-ee5), absolute maximum against recommended operating
- [The comparison](../07-reading-a-datasheet/#the-comparison-ee5)
- [Where did the number come from?](../07-reading-a-datasheet/#where-did-the-number-come-from-ee5), provenance and the trust floor
- [What this layer does not cover](../07-reading-a-datasheet/#what-this-layer-does-not-cover-ee5), why coverage is bounded by the corpus

## Systems (EE6)

**The question you can answer:** what has to come up first, and what feeds what?

A board is a power tree with a sequence, a current budget, and interfaces carrying requirements. This is where the design-intent layer and the `profile/` family live.

- [A board is fed by a cascade](../08-the-power-tree/#a-board-is-fed-by-a-cascade-ee6), reading a power tree off its regulators
- [Nothing here is a fact about the world](../08-the-power-tree/#nothing-here-is-a-fact-about-the-world-ee6), why this level needs a declaration
- [Enough current to go round](../08-the-power-tree/#enough-current-to-go-round-ee6), capacity against margin

- [Order matters, and the datasheet says so](../09-sequencing-and-straps/#order-matters-and-the-datasheet-says-so-ee6), why a chip cares which rail arrives first
- [Declared, then checked](../09-sequencing-and-straps/#declared-then-checked-ee6), the enable chain in copper
- [The same board, declared differently](../09-sequencing-and-straps/#the-same-board-declared-differently-ee6)
- [What a part reads on the way up](../09-sequencing-and-straps/#what-a-part-reads-on-the-way-up-ee6), straps and address collisions

- [A bus is a contract](../10-interfaces-and-what-they-require/#a-bus-is-a-contract-ee6), what a standard requires beyond two wires
- [Declared once, checked everywhere](../10-interfaces-and-what-they-require/#declared-once-checked-everywhere-ee6), profiles and interface hosts
- [The trap in that file](../10-interfaces-and-what-they-require/#the-trap-in-that-file-ee6), supersession switching checks off silently
- [The silence at the end](../10-interfaces-and-what-they-require/#the-silence-at-the-end-ee6), why a satisfied requirement says nothing

## Layout (EE7)

**The question you can answer:** why must that capacitor be *at* the pin?

Where a part physically sits changes what it does electrically. The `board` category and the geometry tier.

- [The copper](../03-why-every-chip-needs-capacitors/#the-copper-ee7), why a netlist check cannot finish the job
- [The promises this chapter is here to keep](../12-when-the-copper-matters/#the-promises-this-chapter-is-here-to-keep-ee7)
- [What geometry can answer](../12-when-the-copper-matters/#what-geometry-can-answer-ee7), clearance, track width, annular ring
- [A fourth kind of authority](../12-when-the-copper-matters/#a-fourth-kind-of-authority-ee7), where a fabrication limit comes from
- [The netlist did not change](../12-when-the-copper-matters/#the-netlist-did-not-change-ee7)

[Chapter 12](../12-when-the-copper-matters/) is the one properly about this level.

---
title: "Learn the domain"
description: "Seven levels of hardware engineering knowledge, and a course that climbs them using the rule catalog."
---

The [tutorials](../tutorials/) teach the tool. These pages teach the domain the tool checks.

They exist because there is a gap between the two. Read [`decoupling-present`](../reference/rules/decoupling-present/) and it says "the cap is the local charge reservoir that serves the chip's switching transients; the regulator is electrically far away." That is a correct and useful sentence which lands only if you already know why a few inches counts as far. The rule pages explain the rule and assume the instinct. These pages build the instinct.

They are organized around the observation that **on a real board, most of the passive components are doing one of about twenty jobs.** Learn the twenty jobs and the catalog stops being seventy-odd separate rules and becomes a short list of engineering instincts, each with a few checks attached to it.

## The seven levels

Each links to [its full definition](levels/), which also maps every section of the course that operates at that level.

Each level is defined by a question you can answer, not by a topic you have read about. The test is the definition, so a level you cannot demonstrate is a level you have not got.

| | Level | The question you can answer | What is new at this level | Where the tool checks it |
|---|---|---|---|---|
| **[EE1](levels/#parts-ee1)** | Parts | "What is that component?" | Ohm's law. What a resistor, capacitor, inductor, diode and transistor each do on their own. | nothing yet; this is the floor |
| **[EE2](levels/#nets-ee2)** | Nets | "Are these two pins connected?" | The drawing is not the circuit. A schematic is a graph, and several things that look like connections are not. | the structural half of `connectivity`, and reader checks such as `symbol-unresolved` |
| **[EE3](levels/#roles-ee3)** | Roles | "Why is *that* resistor there?" | Every part has a job. There are about twenty recurring jobs and they cover most of a board. | most of `connectivity` and `power` |
| **[EE4](levels/#failure-modes-ee4)** | Failure modes | "How does this break, and what does the bench see?" | A defect has a symptom, not just a state. Some faults never appear at power-on. | severity, the `Impact` field, triage and review |
| **[EE5](levels/#numbers-ee5)** | Numbers | "Is this within spec, and which datasheet row says so?" | Absolute maximum versus recommended operating. Derating, tolerance, worst case. | the `datasheet` category and the parameter layer |
| **[EE6](levels/#systems-ee6)** | Systems | "What has to come up first, and what feeds what?" | A board is a power tree with a sequence, a current budget, and interfaces that carry requirements. | the design-intent layer and the `profile/` family |
| **[EE7](levels/#layout-ee7)** | Layout | "Why must that capacitor be *at* the pin?" | Where a part physically sits changes what it does electrically. | the `board` category and the geometry tier |

The levels are cumulative and the numbering is not a ranking of people. A working engineer operates at EE7 on the two subsystems they own and at EE3 on the rest of the board, which is the normal shape of the job rather than a gap.

**EE3 is the level that matters most.** EE1 and EE2 are prerequisites most people arriving from software already have, and everything above EE3 is a refinement of "why is this part here" rather than a new kind of question.

## Asking for a level

Every page here marks the level of each section in its heading, so you can read one page at several depths and stop where it stops being useful. A chapter's headings run `The role (EE3)`, `The failure mode (EE4)`, `The numbers (EE5)`, and so on.

When you want an explanation pitched at a level, name it. "Explain `output-output-conflict` at EE4" asks for the failure mode and the bench symptom rather than the definition, and it is the same request shape as ELI15 calibrated to this domain instead of to an age.

## The course

Each page teaches a model, names the rules that encode it, and then has you run those rules and read what they say. The order climbs the levels, so reading straight through works, and each page states its own prerequisites so jumping in works too.

- **[1. What a board is made of, and why](01-what-a-board-is-made-of/)** (EE1 → EE3) shows that a board is a few kinds of part doing about twenty jobs, and that you can usually tell which job from what a part connects to. Start here.
- **[2. The drawing is not the circuit](02-the-drawing-is-not-the-circuit/)** (EE2) explains that a schematic is a rendering of a netlist and that the two can disagree. One junction dot changes the circuit.
- **[3. Why every chip needs capacitors](03-why-every-chip-needs-capacitors/)** (EE3 → EE7) covers decoupling and bulk capacitors. It is the best single example of the whole ladder, because the same capacitor is a different question at four levels.
- **[4. Pull-ups and undefined states](04-pull-ups-and-undefined-states/)** (EE3 → EE5) starts from a wire nothing drives, which has no voltage, and covers why that needs a resistor, why an open-drain bus fails completely without one, and what severity actually encodes.
- **[5. Who drives this net?](05-who-drives-this-net/)** (EE3 → EE4) explains why two outputs on one net is a short circuit. It is also the chapter where a rule passes a net that is broken, and is right to.
- **[6. Parts that care which way round](06-parts-that-care-which-way-round/)** (EE3 → EE4) starts from the fact that most components have no orientation, and shows the same wiring as a defect on one part and correct usage on another.
- **[7. Reading a datasheet like a type signature](07-reading-a-datasheet/)** (EE5) separates two maximum voltages that look alike and license completely different things, and explains why the same defect reads as an error to one command and provisional to another.
- **[8. The power tree](08-the-power-tree/)** (EE6) reads a board as fed by a cascade, at the first level where correctness comes from a declaration rather than from physics.
- **[9. Sequencing and straps](09-sequencing-and-straps/)** (EE6) covers the order rails arrive in and the configuration parts read at the instant they do, and has the same board pass one declaration and fail another.
- **[10. Interfaces and what they require](10-interfaces-and-what-they-require/)** (EE6) treats a bus as a contract rather than two wires, and closes on a satisfied requirement that produces no output at all.
- **[11. Crystals and oscillators](11-crystals-and-oscillators/)** (EE3 → EE5) shows that a crystal produces nothing on its own, that two capacitors decide what frequency it runs at, and that adding more of them is a defect.
- **[12. When the copper matters](12-when-the-copper-matters/)** (EE7) treats the board as a physical object, on which two nets can pass every connectivity rule in the catalog while the board is defective.

All twelve chapters are written. Read them in order the first time, and after that they stand alone.

## The other direction

If you are coming from software and want the structural map rather than the engineering one, [the software analogy](../reference/analogy/) is the companion to this section. It maps a `PartType` to a class and a `Net` to a shared channel, which gets you to EE2 quickly and deliberately says nothing about EE3 and up. That section answers "what is this thing, in terms I know", and this one answers "why did an engineer put it there".

---
title: "13. Design for test"
description: "A board that passes every check still has to be measured once it is built. What a probe can reach, what an in-circuit tester can measure, and what a coverage number leaves out."
---

Every chapter so far has asked whether the design is right. This one asks a different question, which is whether a board built from a correct design can be shown to match it.

Assembly makes mistakes the schematic never had. A reel of 10k resistors gets loaded where the 4k7 should be, a part is left off, a diode goes on backwards, solder bridges two pins. None of that is visible in the netlist, because the netlist is still right. Finding it means touching the built board, and design for test is the work of deciding, while the board is still a drawing, where those touches can land.

Read [chapter 1](../01-what-a-board-is-made-of/) first for two-terminal parts and rails, and [chapter 3](../03-why-every-chip-needs-capacitors/) for why every rail has capacitors to ground.

This page teaches at [EE3](../levels/#roles-ee3) and [EE4](../levels/#failure-modes-ee4), and the [levels page](../levels/) says what each level means.

## A pad whose only job is to be touched (EE3)

A **test point** is a small pad on one net, connected to nothing else, placed so that a probe can make contact with that net. It does nothing electrically. It shows up in the netlist as a one-pin part, usually with a reference designator starting `TP`, and its only job is to give a probe somewhere to land.

It is not the only way to reach a net. A probe can land on a component's pin, as long as the pin is exposed and large enough to hold a probe tip. A connector pin works too, and so does a via or a bare pad, as long as solder mask does not cover it. What a test point adds is a contact someone chose deliberately, sized for a probe and kept clear of other parts. A pin on a fine-pitch package is a poor substitute, and a ball under a BGA is no substitute at all, because nothing can touch it once the part is soldered.

The tool sees one of those routes. `net.has_test_point` holds for a net that carries a part classified `test_point`, which by default means a `TP` designator, and `net.test_point_count` counts them. The fixture for this chapter is a small regulator circuit built so that each coverage case occurs once:

{{ agniRun "content/learn/runs/dft-reach.yaml" }}

GND carries no test point. The second query shows what is on GND anyway, and one of those parts is `J1`, a two-pin connector whose second pin sits on ground. On a real board a probe could land on that pin. So a `0` here means the net has no dedicated pad, which is a weaker claim than "a probe cannot reach it". A connector pin, an exposed via and a component lead all give access the relation never counts, and the people who review a board for test know that and read the zero as a question.

## Bring-up and production probe differently (EE4)

There are two quite different occasions for touching a board, and they want different things from it.

**Bring-up** is the first few boards on an engineer's bench. The tools are a multimeter and an oscilloscope probe with a ground clip, and the questions are whether each rail came up, whether the clock is running, and whether reset was released. A component lead will do if the engineer can reach it. A test point turns a careful job into a quick one, and it removes a real hazard, since a probe tip that slips off a fine-pitch pin can bridge two neighbours, and on a powered board that can damage the part being measured. (Some test points are a small wire loop rather than a pad, so a scope's hook clip has something to hang on while both hands are busy.)

**Production test** is every board, with no engineer watching. Two kinds of machine touch the nets directly.

- A **bed-of-nails** fixture is built for one board design. It holds a spring-loaded pin for every net it tests, and the board is pressed down onto all of them at once. Building the fixture is expensive, and after that a board takes seconds. It can only touch the places a pin was designed in, which means pads of a minimum size and spacing, usually all on one side of the board, and a change to the board can mean a change to the fixture.
- A **flying probe** tester has no fixture. A few probes on moving heads visit points one at a time, so it is slower per board and better suited to prototypes and small runs. Because it is positioned by software, it can often land on a component pad or a via where a bed of nails would need a test point.

The difference matters for what a design has to provide. Bring-up can get by with component pins and patience. A bed-of-nails fixture reaches exactly the nets that have a pad for it, so on a board meant for one, every net the factory wants to test needs a test point, including nets an engineer would happily have probed at a pin.

## A part is measured between its two nodes (EE4)

The test both machines usually run is **in-circuit test** (ICT), and it runs with the board unpowered. The tester drives a small known current or voltage between two nets and reads what comes back, which is a resistance, a capacitance, or a diode's forward drop. Against the part's expected value, that catches a missing part, a wrong value and a diode fitted backwards. The same machine checks for shorts between nets, so a solder bridge from a rail to ground shows up as a low resistance before anyone applies power. Found then, it costs a rework. Found at power-on, it can cost the board.

The measurement has one requirement that shapes everything else in this chapter. **A two-terminal part is measured between its two nets, so the tester needs to reach both.** A resistor with a test point on one end and nothing on the other cannot be measured at all. Probing one end proves only that the net exists.

`component.two_terminal` names the parts that question applies to. It is decided by connectivity rather than by class, so it lists every part whose pins land on exactly two nets: resistors and capacitors, and also the connector `J1`, because two pins on two nets is what the measurement cares about. `component.probed_both` keeps the parts with a test point on both nets, and `component.probed_one` lists the parts with one, naming the net that is missing:

{{ agniRun "content/learn/runs/dft-measurable.yaml" }}

Only `R1` can be measured. The second table is the more useful one, because it groups the half-probed parts by the net that would rescue them. One test point on GND makes `C1`, `C2` and `J1` all reachable, so the cheapest coverage on this board is a single pad on ground.

## What the measurement cannot see (EE4)

"Reachable" is a statement about the nets, and the tester measures the nets rather than the part. Whatever else sits between the same two nets is in the measurement too.

**Two parts directly across the same pair of nets are measured as one.** Resistors in parallel read as their combined resistance, and capacitors in parallel read as the sum of their values. A rail with a 10 µF bulk capacitor beside a 100 nF decoupling capacitor is the common case, and the arithmetic is short:

{{ includeFile "figures/parallel-capacitors.svg" }}

The 100 nF part is one percent of the reading, and the 10 µF part's own tolerance is ten times that. The tester cannot tell whether the small capacitor is fitted. The same arithmetic hides a large resistor in parallel with a small one, since the small one sets the reading.

Testers have a partial answer to this, called guarding. When the stray path runs through a third net, the tester can hold that net at a potential that stops current flowing through the stray path, but only if the third net is reachable too. Guarding does nothing for two parts directly across the same pair, because there is no third net to hold. An unpowered chip on the same nets adds its own paths, mostly through its protection diodes, which the test engineer allows for when setting limits.

This is why the library's own page for `component.probed_both` calls a listed part reachable rather than guaranteed to measure cleanly. The relation answers the access question exactly. Whether each reachable part produces a reading that can fail is a judgement made from the schematic, usually by the test engineer, and nothing in a netlist query makes it for them.

## Ground and the rails first (EE4)

If a board gets test points on only a few nets, those nets should be ground and the supply rails, and the reasons pretty much all point the same way.

Ground is the reference for every measurement anyone takes, at the bench or on a fixture. Every decoupling and bulk capacitor sits between a rail and ground, as chapter 3 showed, so a pad on ground and a pad on each rail makes every one of those capacitors reachable at once. And the most damaging assembly defect, a short from a rail to ground, is a measurement between exactly those two nets. The first question at bring-up is whether each rail came up, which needs the same pads.

The catalog has a rule for this:

{{ agniRun "content/learn/runs/dft-ground-first.yaml" }}

`test-point-coverage` fails GND and passes VIN and VOUT, naming the test point each one carries. Its severity is `info`, because how much test access a board needs is a policy for each project rather than a defect in the circuit. It only fires on a board that places test points somewhere, since a design with none has chosen a different approach rather than forgotten one. And it leaves out nets named like rails that are not rails, such as a regulator's feedback node or switch node, because a probe there disturbs the circuit it is measuring.

The parts with neither net probed are the last bucket, and grouping them by part number shows something a list of designators does not:

{{ agniRun "content/learn/runs/dft-same-mpn.yaml" }}

`R3` and `R4` share a part number. One uncovered part is usually an oversight. Several of the same part number, all uncovered, more often point to one circuit block that was copied several times with no test point in it, such as the same pull-down repeated on every channel. That is a placement habit, and the fix belongs in the block, once, rather than on each copy.

## Coverage is a number, and a number leaves things out (EE4)

Test coverage is usually reported as the share of parts the tester can measure. Here it is one count over another:

{{ agniRun "content/learn/runs/dft-coverage-number.yaml" }}

One part in seven. The arithmetic is exact, and what it does not say is worth keeping in mind whenever a coverage figure appears in a review.

1. **It does not say whether a reachable part is measured well.** A 100 nF capacitor beside a 10 µF one would add one to the count and nothing to what the test can catch.
2. **The denominator is a choice.** Seven includes `J1`, a connector, and the two pins of an unmated connector have no value between them to measure. Leave connectors out and the figure becomes one in six. A coverage figure without its denominator stated cannot be compared with another one.
3. **It undercounts access.** A net reached through a connector pin or an exposed via counts as unprobed, so a flying probe may reach more than the figure says.
4. **It overcounts access.** A test point under a tall part, on the wrong side for the fixture, or too close to its neighbour for a pin, counts as probed. Whether a pad is usable is a question about the copper, which a netlist does not hold.

None of this makes the number useless. It is a quick way to find the gaps, and the `probed_one` grouping turns it into a short list of pads to add. It is a measure of access, and treating it as a measure of how well the board is tested is the mistake.

## What you can now answer

- What a test point is, and the other ways a probe reaches a net that a test-point count does not see. *(EE3)*
- Why bring-up can manage with component pins and a bed-of-nails fixture cannot. *(EE4)*
- Why in-circuit test needs both of a part's nets, and why one probed end measures nothing. *(EE4)*
- Why a missing 100 nF capacitor beside a 10 µF one passes in-circuit test. *(EE4)*
- Why ground and the rails get test points first, and what several unprobed parts of one part number usually mean. *(EE4)*
- What a coverage figure leaves out, in both directions. *(EE4)*

## The rules this page explains

| Rule | Severity | What it catches |
|---|---|---|
| [`test-point-coverage`](../../reference/rules/test-point-coverage/) | info | a rail or ground net with no test point, on a board that uses them |

The rest of this chapter's questions are asked with the shipped library rather than with rules, because which parts need test access is a project's decision and a query lets each project ask its own version.

| Library member | What it answers |
|---|---|
| [`net.has_test_point`](../../reference/relations/net.has_test_point/) | a net at least one test point sits on |
| [`net.test_point_count`](../../reference/relations/net.test_point_count/) | every net, with how many test points it carries, `0` included |
| [`component.two_terminal`](../../reference/relations/component.two_terminal/) | a part on exactly two nets, decided by connectivity rather than class |
| [`component.probed_both`](../../reference/relations/component.probed_both/) | a two-terminal part with a test point on both nets, which in-circuit test can reach |
| [`component.probed_one`](../../reference/relations/component.probed_one/) | a two-terminal part with a test point on one net, naming the net that would make it measurable |

The [`dft-coverage` example](https://github.com/panyam/agni/tree/main/examples/dft-coverage) walks the same fixture step by step, prints the `agni` command behind each step, and runs over your own board when you point it at one. It ends on the verdicts `test-point-coverage` gives, which say what the rule examined as well as what it found.

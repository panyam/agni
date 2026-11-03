---
title: "5. Who drives this net?"
description: "Every net needs exactly one thing deciding its voltage. What happens when two do, when none does, and why a pass on one rule proves nothing about the others."
---

[Chapter 4](../04-pull-ups-and-undefined-states/) was about a net with nothing driving it. This one is about the other end of the same question, a net with two things driving it. It ends on what a passing rule does and does not tell you.

Read [chapter 4](../04-pull-ups-and-undefined-states/) first, particularly the open-drain section.

This page teaches at [EE3](../levels/#roles-ee3) and [EE4](../levels/#failure-modes-ee4), and the [levels page](../levels/) says what each level means.

## One net, one decider (EE3)

A net's voltage has to be decided by something. An output pin decides it through a transistor, by connecting the wire either to the supply rail or to ground. The rest of the net follows whichever one it picks.

That works cleanly while exactly one pin is doing the deciding. Put two outputs on one net and let them disagree, and you have a problem with a mechanism rather than a vague badness. One output has connected the net to the rail. The other has connected the same net to ground. The only thing between the rail and ground is now two transistors in series, and a transistor that is fully on is a small resistance rather than a wire, typically tens of ohms.

{{ includeFile "figures/output-contention.svg" }}

So the current is the rail voltage divided by that resistance, which comes to tens to hundreds of milliamps. Output pins are usually rated for a few milliamps. (If you have ever touched a chip on a prototype and found it uncomfortably hot with nothing obviously wrong, this is one of the two or three things worth suspecting.) Both parts heat, both are operating far outside their ratings, and if the condition persists one of them eventually stops working.

[Chapter 4's](../04-pull-ups-and-undefined-states/) open-drain outputs solve this. An open-drain pin can only ever pull the net toward ground, so two of them can share a wire and never fight: both pulling low at once is simply low. Buses that let several devices talk are built on that idea, or on the alternative, an output that can be switched into a third state where it drives nothing at all and lets someone else decide.

## Two nets, opposite problems (EE3)

Here is a fixture with one of each, run through three rules at once:

{{ agniRun "content/learn/runs/drivers.yaml" }}

`FIGHT` has two drivers and fails contention, exactly the case described above.

`FLOAT` has none. Look at what the rules say about it.

**`output-output-conflict` passes `FLOAT`, and that pass is correct.** The rule asks whether more than one thing is driving, and nothing is driving, so there is no contention.

**`FLOAT` is still broken.** `floating-input` fails it, because every pin on it is an input and chapter 4 explained what that costs.

**`single-pin-net` passes both**, because both nets reach two pins and neither is a stub.

So one net collects a pass, a pass, and a fail from three rules, and every one of those correctly answers the question that rule asks. A pass means "this specific thing is fine here". It does not mean the net is fine, and no rule in the catalog claims otherwise.

It is worth internalising early. The whole tool behaves this way, and knowing it separates reading a report from trusting one. A green run tells you the questions that were asked came back clean. The [considered set](../../tutorials/09-read-the-verdicts/) says which questions those were, and usually more rules had nothing to say about your board than had something to say.

## The same fault on a rail (EE4)

Signals are not the only place this happens. Two supplies feeding one rail is the same defect with more energy behind it:

{{ agniRun "content/learn/runs/power-contention.yaml" }}

Note that it is an `error` here for the same reason [chapter 4's](../04-pull-ups-and-undefined-states/) I2C rule was. Two regulators with slightly different output voltages on one net do not average out. The higher one supplies the rail and pushes current backwards into the lower one. Regulators are not built to accept that.

This arrangement is sometimes deliberate, and the deliberate version combines the supplies through diodes or a dedicated OR-ing controller, so the higher one wins and nothing flows backwards. A design doing that on purpose has the parts to show for it. A design that simply wired two outputs together does not, and this rule tells them apart.

## Pins the designer said to leave alone (EE3)

There is one more way a net can be wrong. Unlike the others in this chapter, the tool could not possibly work this one out for itself.

A designer marks a pin **no-connect** to annotate it as intentionally unused. An unconnected pin is normally worth flagging and a no-connect one is not, so the annotation changes what other checks mean. The annotation exists to remove that ambiguity.

Wiring something to a pin marked no-connect is therefore an error, and a slightly unusual one, because the design is contradicting itself. Somebody wrote down that the pin should be left alone and somebody wired it up. Whichever is right, both statements are in the file and they disagree.

The tool can only see this because a human annotated it. That follows [chapter 1's](../01-what-a-board-is-made-of/) pattern of declaring what a netlist cannot infer. The same goes for pin direction itself. The symbol says whether a pin is an input, an output or a supply. If a symbol never says, none of this chapter's questions can be answered for that pin, and `unspecified-pin-with-driver` reports that rather than guessing. `dl/power-pin-mistyped` catches a related slip. A pin is named like a supply, the symbol never typed it as one, and it sits on a net of its own.

## What you can now answer

- Why two outputs on one net is a short circuit rather than a disagreement, and roughly how much current flows. *(EE3)*
- Why open-drain and tri-state exist, and what problem each solves. *(EE3)*
- Why a rule passing a net says nothing about whether the net is correct. *(EE4)*
- Why two regulators on a rail is a defect, and what the deliberate version looks like instead. *(EE4)*
- Why a no-connect annotation is information rather than decoration. *(EE3)*

## The rules this page explains

| Rule | Severity | What it catches |
|---|---|---|
| [`output-output-conflict`](../../reference/rules/output-output-conflict/) | error | two or more pins driving one net |
| [`nc-pin-connected`](../../reference/rules/nc-pin-connected/) | error | a pin marked no-connect wired into a net |
| [`unspecified-pin-with-driver`](../../reference/rules/unspecified-pin-with-driver/) | warning | a pin whose type the symbol never stated, on a driven net |
| [`unconnected-pin`](../../reference/rules/unconnected-pin/) | warning | a pin on no net, without the no-connect annotation that would explain it |
| [`dl/power-pin-mistyped`](../../reference/rules/dl-power-pin-mistyped/) | warning | a pin named like a supply but not typed as one, sitting alone |

The next chapter, [parts that care which way round](../06-parts-that-care-which-way-round/), is about parts with an orientation.

---
title: "4. Pull-ups and undefined states"
description: "A wire nothing drives has no voltage, which is stranger than it sounds. Why that needs a resistor, and why one bus fails completely without it."
---

Ask a software engineer what an unconnected input reads and you usually get "zero". It is a reasonable guess, and it is wrong in a way that turns out to matter.

A wire nothing drives does not sit at zero volts. It does not sit anywhere. This chapter is about why, and about the large family of resistors that exist because of it and otherwise look like they are doing nothing.

**Prerequisites:** [Chapter 1](../01-what-a-board-is-made-of/) for the idea that a part's job is readable from what it connects to.

**Levels on this page:** [EE3](../levels/#roles-ee3), [EE4](../levels/#failure-modes-ee4), [EE5](../levels/#numbers-ee5). Each links to [what that level means](../levels/).

## Nothing is a voltage nothing has (EE3)

A logic input on a modern chip is the gate of a MOSFET, and a gate is one plate of a very small capacitor with an insulating layer underneath. Almost no current flows into it. The input leakage on a typical part is measured in nanoamps, which means the input presents an impedance of hundreds of megohms or more.

That is the surprising part, and everything follows from it. **A node with enormous impedance has its voltage set by whatever tiny currents happen to reach it.** Capacitive coupling from a trace running alongside. Leakage across the board surface. Charge left over from the last time something did drive it. None of those are big enough to matter anywhere else on the board, and on a floating input they are the only thing there is.

So the line drifts. It can sit anywhere between ground and the rail, including the region in the middle that is neither a valid low nor a valid high.

That middle region is worse than it sounds, and this is the part people miss. A CMOS input stage is a pair of transistors, one pulling toward the rail and one toward ground. At a valid high, one is on and the other is off. At a valid low, the reverse. **Held in the middle, both are partly on**, which opens a direct path from the rail to ground through the input stage. The part draws current it was never meant to draw, warms up, and the input can oscillate as it drifts back and forth across the threshold.

Hence the rule every datasheet states and nobody explains: do not leave a CMOS input floating. A resistor to a rail (**pull-up**) or to ground (a {{ explainable "pull-down" }}) gives the node somewhere to sit. It is weak enough that anything genuinely driving the line overrides it easily, and strong enough that stray coupling cannot.

The tool has a rule for this, and its verdicts are more interesting than its findings:

{{ agniRun "content/learn/runs/floating-input.yaml" }}

Read the four `not-considered` rows closely, because **you already know how to answer them and the tool does not.**

`MCU_NRST` and `PMIC_EN` are the two nets [chapter 1](../01-what-a-board-is-made-of/) used to teach the decision procedure. `PMIC_EN` carries R2, a 10k to the 12V rail, which chapter 1 read as a pull-up. `MCU_NRST` carries R3, a 10k to power-good, which chapter 1 read as a series resistor. The rule sees only "there is a passive part on this net" and stops, because from the netlist alone a resistor might be the pull that fixes the float or a series element with the real driver on the far side of it.

That is a rule declining to guess rather than a rule failing. You can look at the net names and resolve it; the tool cannot, and reports which of the two situations it is in rather than picking one. Chapter 9 of the tutorials is entirely about reading that distinction.

## Some pins can only pull one way (EE3)

There is a second reason a pull-up exists, and it is not about floating at all.

Some outputs are built to drive **only low**. The pin can connect the line to ground or it can let go, and letting go is all it does at the top. These are called open-drain, and the point of them is that many devices can share one wire without ever fighting. If two pull low at once, nothing is damaged and the line is simply low. Compare that with two ordinary outputs on one net, one driving high and one driving low, which is a short across the supply through two transistors. (That is `output-output-conflict`, and it is chapter 5.)

The catch is that if nothing drives high, nothing brings the line back up. An open-drain bus needs an external resistor to a rail, and it does not work at all without one.

I2C is the standard example. Two lines, both open-drain, both needing a pull-up:

{{ agniRun "content/learn/runs/i2c-pullup.yaml" }}

The witness is worth a second look. It says "within 3 hops" rather than "no resistor to a rail", because the rule walks outward through series elements rather than demanding a resistor wired straight to the rail. A pull-up sitting on the far side of a series element or a filter is still a pull-up, and a check that insisted on a direct connection would report a defect on a board that is fine.

## Why this one is an error (EE4)

Here is the same board with three rules at once, and the interesting thing is the severity column:

{{ agniRun "content/learn/runs/pullup-severity.yaml" }}

`decoupling-present` is a **warning**. `i2c-pull-up` is an **error**. Both are missing passives on a board that will be built either way, and [chapter 3](../03-why-every-chip-needs-capacitors/) argued at some length that missing decoupling is genuinely serious. So why the difference?

Because severity in this catalog answers **"does it work at all"**, not "how sure are we" and not "how much do we care".

A board with no decoupling on a rail powers up and runs. It fails later, intermittently, under load or heat, on some units. An I2C bus with no pull-up does not work once. Every device on it can pull the line down and nothing can bring it back up, so after the first low the bus stays low forever. Nothing communicates, at any temperature, on every board built. The rule's own `Impact` calls it a total-function failure.

Keep that pair in mind, because it defines the whole scale. `error` means the thing cannot work. `warning` means it usually indicates a defect and the board still runs. Neither says anything about how confident the check is, which the verdicts carry on a separate axis.

## Choosing the resistor (EE5)

At EE3 the answer was "a pull-up". At EE5 you have to pick a value, and it is a trade-off with a limit at each end.

**Too weak** (too large a resistance) and the line rises too slowly. Everything on the bus has capacitance, and the pull-up has to charge all of it through itself, so the rise is an RC curve. I2C caps total bus capacitance at 400pF and sets a maximum rise time per speed mode, and a large pull-up on a heavily loaded bus misses it.

**Too strong** (too small a resistance) and the device pulling the line down has to sink all the current the pull-up passes. Every open-drain output has a maximum sink current in its datasheet, and going under it is how you exceed the low-level output voltage the receivers need.

The usual landing zone is a few kilohms, with weaker values on short lightly loaded buses and stronger ones as the bus grows. (If you ever meet a board where I2C works at room temperature and stops when it warms up, a marginal pull-up against a slow rise time is one of the first things worth measuring.) The catalog checks presence rather than value, because value needs the bus capacitance and the sink ratings, which is a datasheet-tier question and the subject of chapter 7.

## What you can now answer

- Why an unconnected input does not read zero, and what it does instead. *(EE3)*
- Why a floating CMOS input costs supply current rather than just reading unpredictably. *(EE3)*
- Why an open-drain bus needs a resistor to work at all, and why several devices can share one line safely. *(EE3)*
- Why one missing passive is an error and another is a warning, on the same board. *(EE4)*
- Which way a pull-up value can be wrong, in both directions. *(EE5)*

## The rules this page explains

| Rule | Severity | What it catches |
|---|---|---|
| [`i2c-pull-up`](../../reference/rules/i2c-pull-up/) | error | an I2C line reaching no rail through a resistor |
| [`floating-input`](../../reference/rules/floating-input/) | warning | a net whose pins are all inputs, with nothing driving or pulling it |
| [`profile/missing-pullup`](../../reference/rules/profile-missing-pullup/) | warning | the same question for any bus a declared interface says needs one |
| [`unspecified-pin-with-driver`](../../reference/rules/unspecified-pin-with-driver/) | warning | a pin whose electrical type the design never stated, sitting on a driven net |

Next: [who drives this net](../05-who-drives-this-net/), the other half of the question this chapter started, and the case where two parts both answer.

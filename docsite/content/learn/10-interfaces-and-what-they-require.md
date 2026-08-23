---
title: "10. Interfaces and what they require"
description: "A bus is a contract, not two wires. Declaring what it requires once, and a silence at the end that carries the whole course."
---

The last of the three system chapters. [Chapter 8](../08-the-power-tree/) was how a board is fed, [chapter 9](../09-sequencing-and-straps/) was when and in what configuration, and this one is how it talks to anything else.

**Prerequisites:** [Chapter 8](../08-the-power-tree/) for the idea of a declaration.

**Levels on this page:** [EE6](../levels/#systems-ee6). It links to [what that level means](../levels/).

## A bus is a contract (EE6)

"This board has CAN on it" sounds like a statement about two wires. It is closer to a statement about a specification, and the specification requires things.

CAN needs a **differential pair**, two wires carrying the same signal in opposite senses, because a receiver looking at the difference between them rejects any interference that hit both equally. It needs **termination**, a resistor bridging the pair at each end of the bus, for the transmission-line reason [chapter 1](../01-what-a-board-is-made-of/) gave. It needs a **transceiver**, since a processor's logic pins cannot drive the differential voltages the bus uses. And anything leaving the board through a connector needs **protection**, because a cable is an antenna and a person touching it is a static discharge.

None of that is visible in a netlist as a requirement. A board with CAN wired wrongly is a perfectly valid netlist. (This is the chapter where the tool stops being able to work anything out for itself, and it has been heading that way since chapter 8.) So, exactly as in the last two chapters, somebody has to declare what the interface is and what it demands.

## Declared once, checked everywhere (EE6)

That declaration is a **profile**, and the useful property is that it is written once and applies to every design carrying the interface:

```yaml
name: CAN
host: {attr: interface, value: CAN}
signals:
  - {name: CANH, suffix: _CANH, anchor: true}
  - {name: CANL, suffix: _CANL}
  - {name: TXD,  suffix: _TXD}
  - {name: RXD,  suffix: _RXD}
  - {name: STB,  suffix: _STB}
requirements:
  - {type: signal-missing}
  - {type: host-incomplete}
  - {type: termination, params: {high: _CANH, low: _CANL}}
  - {type: signal-dangling}
  - {type: esd}
```

`host` is how a part gets recognised as a CAN device. [Chapter 1](../01-what-a-board-is-made-of/) noted that the tool could not tell what kind of chip `U4` was, and this is the answer: the design carries an `interface: CAN` attribute on it, and the profile says that attribute is what makes something a host.

Run it against the board:

{{ agniRun "content/learn/runs/interface-profile.yaml" }}

`U4` declares itself a CAN host and has no `_STB` net, so `host-incomplete` fires. Note that STB is a **house** requirement rather than a CAN one: standby is how this organisation puts a bus to sleep, and the standard has no opinion about it. That is the point of a profile living in your project rather than in the tool.

The ESD finding is the other kind, a genuine requirement of anything with a connector on it. Both CAN nets reach the outside world through `J1` with no clamp in reach.

## The trap in that file (EE6)

Worth pausing on, because it is the kind of thing that costs a real board.

This project's profile is named `CAN`, and so is the built-in one. Sharing the name is what makes the project's version **supersede** the built-in rather than run alongside it. That is deliberate and it is what lets a house add its STB requirement.

It also means that **anything the built-in checked and this file omits stops being checked, silently**. The file's own comment says so, which is why its `signals` and `requirements` lists repeat the built-in set rather than listing only the delta. A profile that listed just the STB addition would have quietly switched off termination, ESD and everything else.

That shape recurs anywhere configuration replaces rather than merges, and the failure is always the same: the run gets quieter and nothing says why.

## The silence at the end (EE6)

Now the thing this chapter is really for.

Look at the profile's `termination` requirement, then look at what the board has. [Chapter 1](../01-what-a-board-is-made-of/) opened the course by reading three resistors off a query, and the first of them was `R1`, a 120 Ω part bridging `CAN1_CANH` and `CAN1_CANL`. That is what the requirement asks for.

{{ agniRun "content/learn/runs/termination-silent.yaml" }}

R1 is there. The requirement is satisfied. **The rule says nothing whatsoever.**

That is not a bug and it is not an oversight. Profile rules are compiled from datalog queries, and a datalog goal yields the rows that *satisfy* it. `unterminated(?h)` produces unterminated buses; there is no natural complement that produces "the buses that were fine". So these rules report violations and claim nothing about anything else, which the run summary says out loud in its last line. Whether they could state one is agni issue 424.

Sit with what that means, because one sentence carries the whole course. **A clean result from those rules and a board with no CAN on it produce identical output.** If you had deleted R1, the termination rule would have fired. If you had deleted the entire CAN bus, it would also have said nothing, exactly as it does now. The silence cannot distinguish them.

Everything in this tool that looked fussy earlier is downstream of that observation. [Chapter 5's](../05-who-drives-this-net/#two-nets-opposite-problems-ee3) insistence that a pass is a pass about one question, the `not-considered` verdicts in [chapters 4](../04-pull-ups-and-undefined-states/) and [8](../08-the-power-tree/), and the summary line counting how many rules declined to say what they examined, are all the same idea: a report is only worth something if you can tell what it looked at.

## What you can now answer

- Why "this board has CAN" is a claim about a specification rather than about two wires. *(EE6)*
- How a part gets recognised as an interface host when a netlist cannot say what a chip is. *(EE6)*
- Why a profile that lists only what it adds silently switches off everything it omits. *(EE6)*
- Why a satisfied requirement produces no output here, and why that makes a clean run and an absent bus indistinguishable. *(EE6)*

## The rules this page explains

| Rule | Severity | What it catches |
|---|---|---|
| [`profile/signal-missing`](../../reference/rules/profile-signal-missing/) | error | a signal the interface declares, absent from the design |
| [`profile/termination`](../../reference/rules/profile-termination/) | warning | a bus needing termination with none across its pair |
| [`profile/esd`](../../reference/rules/profile-esd/) | warning | an interface signal leaving the board with no clamp |
| [`profile/signal-dangling`](../../reference/rules/profile-signal-dangling/) | warning | an interface net reaching fewer than two connections |
| [`profile/missing-pullup`](../../reference/rules/profile-missing-pullup/) | warning | an interface signal needing a pull-up and reaching no rail |

A project's own profile compiles these under its own name, which is why the board above reports `gateway-profiles/can-esd-missing` rather than `profile/esd`.

Next: [crystals and oscillators](../11-crystals-and-oscillators/), a short chapter on the part whose value matters as much as its presence.

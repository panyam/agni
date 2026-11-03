---
title: "10. Interfaces and what they require"
description: "A bus is a contract, not two wires. Declaring what it requires once, and what a satisfied requirement should say."
---

The last of the three system chapters. [Chapter 8](../08-the-power-tree/) was how a board is fed, [chapter 9](../09-sequencing-and-straps/) was when and in what configuration, and this one is how it talks to anything else.

Read [chapter 8](../08-the-power-tree/) first, for the idea of a declaration.

This page teaches at [EE6](../levels/#systems-ee6), and the [levels page](../levels/) says what each level means.

## A bus is a contract (EE6)

"This board has CAN on it" sounds like a statement about two wires. It is closer to a statement about a specification, and the specification requires things.

CAN needs a {{ explainable "differential-pair" }}, {{ explainable "termination" }} at each end of the bus, a {{ explainable "transceiver" }} between the bus and the processor, and {{ explainable "port-protection" }} on anything leaving the board through a connector. [Chapter 1](../01-what-a-board-is-made-of/) taught all four as jobs a part does. This chapter adds that CAN *requires* them, and requires them together.

{{ includeFile "figures/can-interface-contract.svg" }}

None of that is visible in a netlist as a requirement. A board with CAN wired wrongly is a perfectly valid netlist. (In this chapter the tool stops being able to work anything out for itself, and it has been heading that way since chapter 8.) So somebody has to declare what the interface is and what it demands, exactly as in the last two chapters.

## Declared once, checked everywhere (EE6)

That declaration is a **profile**. It is written once and applies to every design carrying the interface:

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

`host` is how a part gets recognised as a CAN device. [Chapter 1](../01-what-a-board-is-made-of/) noted that the tool could not tell what kind of chip `U4` was. The design answers that by carrying an `interface: CAN` attribute on `U4`, and the profile says any part carrying that attribute is a host.

Run it against the board:

{{ agniRun "content/learn/runs/interface-profile.yaml" }}

`U4` declares itself a CAN host and has no `_STB` net, so `host-incomplete` fires. STB is a **house** requirement rather than a CAN one, because standby is how this organisation puts a bus to sleep and the standard has no opinion about it. A profile lives in your project rather than in the tool so that a house can add a requirement like this one.

The ESD finding is the other kind, a genuine requirement of anything with a connector on it. Both CAN nets reach the outside world through `J1` with no clamp in reach.

That assumes `J1` faces the outside world, and the tool cannot know. If this board plugged into a carrier through `J1`, the CAN pins would meet the carrier's copper inside the enclosure, and a cable would touch the carrier's own connector instead. The same connector part can be either, so the design says which in its intent with `components: {J1: {exposure: internal}}`. The exposure rules then leave alone any net that reaches only `J1` (see [Design intent](../../guide/design-intent/)).

## The trap in that file (EE6)

This one costs real boards.

This project's profile is named `CAN`, and so is the built-in one. Because the names match, the project's version **supersedes** the built-in rather than running alongside it. That is deliberate, and it lets a house add its STB requirement.

It also means that **anything the built-in checked and this file omits stops being checked, silently**. The file's own comment says so. For that reason its `signals` and `requirements` lists repeat the built-in set rather than listing only the delta. If a profile listed just the STB addition, it would have quietly switched off termination, ESD and everything else.

That shape recurs anywhere configuration replaces rather than merges, and in every case the run gets quieter and nothing says why.

## What a satisfied requirement says (EE6)

Look at the profile's `termination` requirement, then look at what the board has. [Chapter 1](../01-what-a-board-is-made-of/) opened the course by reading three resistors off a query, and the first of them was `R1`, a 120 Ω part bridging `CAN1_CANH` and `CAN1_CANL`. The requirement asks for exactly that part.

{{ agniRun "content/learn/runs/termination-witnessed.yaml" }}

R1 is there, the requirement is satisfied, and the rule **says so, naming the net and why**.

For most of this tool's life that last part did not happen. Profile rules are compiled from datalog queries, and a datalog goal yields the rows that *satisfy* it. `unterminated(?h)` produces unterminated buses; the same query has no complement that produces "the buses that were fine". So the rule reported the violations and claimed nothing about anything else.

**Such a rule produces identical output for a correctly terminated bus and for a board with no CAN on it.** Delete R1 and the termination rule fires. Delete the entire CAN bus and it says nothing. It says the same nothing about a correctly terminated bus. Silence cannot tell those apart, so a reader cannot either.

The fix leaves the query alone. The rule now declares the set it examined, separately from the answer it reached, and the tool reports the difference. Nothing infers the scope from the goal, because for these rules the goal does not hold enough to recover it. `signal-missing` carries two negated conditions and only one of them is the test, and `signal-dangling` ends in a comparison with no negation at all. An author knows which half is which. A derivation would guess, and guessing wrong reports the failures as the coverage.

Several things in this tool looked fussy earlier, and they all come from the same idea. [Chapter 5](../05-who-drives-this-net/#two-nets-opposite-problems-ee3) insisted that a pass is a pass about one question. [Chapters 4](../04-pull-ups-and-undefined-states/) and [8](../08-the-power-tree/) showed `not-considered` verdicts. The summary line counts what a run considered. Each follows from one rule, that a report is only worth something if you can tell what it looked at.

Try it on the interface as a whole, and note that a house requirement and a protocol one are answered in the same voice:

```
agni check --verdicts --rule gateway-profiles/can-host-incomplete designs/gateway/gateway.edn
```

`U4` gets one verdict per required signal rather than one verdict as a part, because a host wired to four of its five lines is four right answers and one wrong one. Each is addressable on its own, and `gateway-profiles/can-host-incomplete:(component:U4,signal:STB)` is a question you can type before running anything. [Chapter 9](../09-sequencing-and-straps/) used the same pair-shaped subject for strap collisions, for the same reason. The answer belongs to a relation rather than to either thing in it.

## What you can now answer

- Why "this board has CAN" is a claim about a specification rather than about two wires. *(EE6)*
- How a part gets recognised as an interface host when a netlist cannot say what a chip is. *(EE6)*
- Why a profile that lists only what it adds silently switches off everything it omits. *(EE6)*
- Why a satisfied requirement used to produce no output, and why that made a clean run and an absent bus indistinguishable. *(EE6)*
- Why the set a rule examined has to be declared by its author rather than inferred from the check it runs. *(EE6)*

## The rules this page explains

| Rule | Severity | What it catches |
|---|---|---|
| [`profile/signal-missing`](../../reference/rules/profile-signal-missing/) | error | a signal the interface declares, absent from the design |
| `profile/host-incomplete` | error | a declared interface host missing one of the signals the profile requires |
| [`profile/termination`](../../reference/rules/profile-termination/) | warning | a bus needing termination with none across its pair |
| [`profile/esd`](../../reference/rules/profile-esd/) | warning | an interface signal leaving the board with no clamp |
| [`profile/signal-dangling`](../../reference/rules/profile-signal-dangling/) | warning | an interface net reaching fewer than two connections |
| [`profile/missing-pullup`](../../reference/rules/profile-missing-pullup/) | warning | an interface signal needing a pull-up and reaching no rail |

A project's own profile compiles these under its own name. So the board above reports `gateway-profiles/can-esd-missing` rather than `profile/esd`.

The next chapter, [crystals and oscillators](../11-crystals-and-oscillators/), is a short one on the part whose value matters as much as its presence.

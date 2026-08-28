---
title: "5. Your interfaces"
description: "Declare a bus once, check every board against it, and understand what superseding a built-in profile costs."
---

A bus like CAN is the same set of requirements on every board that has one. An interface profile
declares that set once: which signals make up the bus, how they are named here, and what has to be
true of them. Then any board can be checked against it.

The engine ships profiles for CAN, LIN, eMMC, PCIe, SGMII, SPI-NOR, and A2B. This rung is about
adding your own, and about the two things that surprise people when they do.

## What the built-in profile already finds

The tutorial board has CAN, and the shipped profile has something to say about it. This run moves the
project's own `profiles/` aside first, so what you see is the built-in tier on its own:

{{ agniRun "content/tutorials/runs/05-check-profiles-off.yaml" }}

The shipped CAN profile contributes four rules here. The bus is complete and terminated, so three of
them pass and say nothing in this view. The pair reaches a connector with no protection on it, so
`profile/can-esd-missing` fires twice. Rung 9 shows you those passes.

## Absence is not a pass

Before adding anything, the more important half. The board has no LIN at all. A checklist item
asking "is the LIN interface complete" must not come back green just because a check for a bus that
is not there found nothing to complain about.

The profile tier gates on presence. An interface whose anchor signal is nowhere on the board is
reported as unevaluated rather than passed. You will see this directly in rung 9, where the LIN item
reads `not-automated` with the reason attached, and the difference between that and `pass` is the
whole point of the review layer.

## Adding your own

`profiles/can.yaml` in the tutorial project declares this team's CAN:

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

`suffix` is how a net is recognized as playing that role, so `CAN1_CANH` is this bus's CANH.
`anchor: true` marks the signal whose presence means the bus exists at all. `host` says which
component declares it.

The difference from the built-in is one line: a `STB` signal. This team always routes the
{{ explainable "transceiver" }}'s standby pin back to the MCU so firmware can put the bus to sleep. The built-in profile
has no opinion about that, because it is a house practice rather than a CAN requirement.

There is no flag. The project names its own `profiles/` directory, so agni composes it for every
design under `designs/`, the same way it composes `conventions.yaml` and `params/`. Putting the file
in place is the whole step:

{{ agniRun "content/tutorials/runs/05-check-profiles.yaml" }}

The board does not route STB, so the new requirement has a real finding.

## Supersession, and what it costs

Compare the rule names against the run at the top of this page. `profile/can-esd-missing` has become
`gateway-profiles/can-esd-missing`, and no `profile/can-*` rule appears in the second run at all.

Your profile has the same name as a built-in one, so it **replaces** it. It does not run alongside
it. Every rule the built-in CAN profile contributed is gone, and only what your file declares is
checked.

Supersession is per interface, not per run. The shipped LIN profile is untouched by any of this,
because this project declared no LIN.

This is the right default. Two profiles both claiming to be CAN, both firing on the same nets, would
double-report everything and there would be no way to say your naming differs from the built-in
assumption.

But it means an omission is silent. If `profiles/can.yaml` had listed only the `STB` signal and the
`host-incomplete` requirement, then {{ explainable "termination" }}, dangling signals, and ESD would simply stop being
checked on every board this project reviews, and nothing would say so. The run would look healthier,
because there would be fewer findings.

So the file above repeats the built-in's signals and requirements rather than listing only the
delta. That repetition states the whole declaration rather than repeating itself, because a whole declaration
is what supersession replaces.

`agni check --verdicts` is your check on this. When you add a profile, read which rules ran and
confirm your file covers each one the built-in contributed, or that you meant to drop it.

## What the rung bought you

Compare the last line of the two runs on this page:

```
without profiles/   27 finding(s)   257 subject(s) considered by 36 rule(s)
with profiles/      28 finding(s)   258 subject(s) considered by 36 rule(s)
```

One more finding and one more subject, and both of them are the `STB` line you added. That is the
size of the change you made, which is what a tier ought to move.

Supersession shows up here as the rule count holding at 36. Four built-in CAN rules left and four of
yours arrived, so the same number of rules has an opinion about this board. They are now your
rules.

The second number is the one worth watching as you add tiers. Findings tell you what is wrong today;
the considered count tells you how much of the board anything is looking at, which is what you are
actually buying when you declare an interface.

Ask a requirement what it concluded and it will tell you, whether or not it found anything:

```
agni check --verdicts --rule gateway-profiles/can-host-incomplete designs/gateway/gateway.edn
```

`U4` comes back as one answer per required signal rather than one answer as a part: `STB` fails and
the other four pass, each with the evidence behind it. A satisfied requirement that printed nothing
would be indistinguishable from a requirement that never ran, which is the distinction
[rung 9](../09-read-the-verdicts/) is built on.

## Next

[Part limits](../06-part-limits/), where checks start comparing your design against what a part's
datasheet actually allows.

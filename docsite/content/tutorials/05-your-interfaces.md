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

The tutorial board has CAN, and the shipped profile has something to say about it:

```
agni check designs/gateway/gateway.edn
```

```
  profile/can-esd-missing 2
  [warning] profile/can-esd-missing: CAN1_CANH (CAN signal net CAN1_CANH is exposed on a connector with no ESD protection in reach)
  [warning] profile/can-esd-missing: CAN1_CANL (CAN signal net CAN1_CANL is exposed on a connector with no ESD protection in reach)
9 finding(s) total
```

The bus is complete and terminated, so those checks pass silently. The pair reaches a connector with
no protection on it, so that one fires.

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
transceiver's standby pin back to the MCU so firmware can put the bus to sleep. The built-in profile
has no opinion about that, because it is a house practice rather than a CAN requirement.

```
agni check designs/gateway/gateway.edn --profile-path profiles
```

```
note: profile-overlay supersedes 5 rule(s): profile/can-signal-missing, profile/can-host-incomplete, profile/can-termination-missing, profile/can-signal-dangling, profile/can-esd-missing
  profile-overlay/can-esd-missing 2
  profile-overlay/can-host-incomplete 1
  [error] profile-overlay/can-host-incomplete: U4 (CAN host U4 declares the interface but is missing required signal STB)
10 finding(s) total
```

The board does not route STB, so the new requirement has a real finding.

## Supersession, and what it costs

Read that first line again:

```
note: profile-overlay supersedes 5 rule(s): profile/can-signal-missing, profile/can-host-incomplete, profile/can-termination-missing, profile/can-signal-dangling, profile/can-esd-missing
```

Your profile has the same name as a built-in one, so it **replaces** it. It does not run alongside
it. Every rule the built-in CAN profile contributed is gone, and only what your file declares is
checked.

This is the right default. Two profiles both claiming to be CAN, both firing on the same nets, would
double-report everything and there would be no way to say your naming differs from the built-in
assumption.

But it means an omission is silent. If `profiles/can.yaml` had listed only the `STB` signal and the
`host-incomplete` requirement, then termination, dangling signals, and ESD would simply stop being
checked on every board this project reviews, and nothing would say so. The run would look healthier,
because there would be fewer findings.

So the file above repeats the built-in's signals and requirements rather than listing only the
delta. That is not redundancy. It is the whole declaration, because a whole declaration is what
supersession replaces.

The printed note is your check on this. When you add a profile, read the list of superseded rules
and confirm your file covers each of them or that you meant to drop it.

## Next

[Part limits](../06-part-limits/), where checks start comparing your design against what a part's
datasheet actually allows.

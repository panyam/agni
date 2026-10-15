---
title: "5. Your interfaces"
description: "Declare a bus once, check every board against it, and understand what superseding a built-in profile costs."
---

A bus like PCIe is the same set of requirements on every board that has one. An interface profile
declares that set once: which signals make up the bus, how they are named here, and what has to be
true of them. Then any board can be checked against it.

The engine ships profiles for CAN, LIN, eMMC, PCIe, SGMII, SPI-NOR, MDIO and A2B. This rung is about
adding your own, and about the two things that surprise people when they do.

## What the built-in profiles already find

This run moves the project's own `profiles/` aside first, so what you see is the built-in tier on its
own. The first number counts the profile rules with any verdict at all:

{{ agniRun "content/tutorials/runs/05-check-profiles-off.yaml" }}

None. The board has four PCIe links, to the two M.2 sockets and the FMC+ header, and the shipped PCIe
profile says nothing about any of them. It recognizes a PCIe signal by a suffix such as `_PETP` or
`_REFCLKN`, and this board draws each link as a KiCad bus whose members are named `PCIe_{C1x1}.TX0+`
and `PCIe_{C5x4}.RX3-`. Nothing on the board looks like PCIe to it, so it reports nothing, and a
findings list cannot tell you that apart from four healthy links.

## Absence is not a pass

That silence is the more important half, before adding anything. The profile tier gates on presence:
a profile is in use only when its anchor signal, the one whose presence means the bus exists at all,
is on the board. An interface whose anchor is nowhere is reported as unevaluated rather than passed,
and the review layer in rung 9 keeps that apart from `pass`.

So the question a silent profile raises is never "is the bus fine". It is "did the profile recognize
the bus", and on this board the answer is no.

## Adding your own

`profiles/pcie.yaml` in the project declares this team's PCIe, in this board's naming:

```yaml
name: PCIe
signals:
  - {name: TXP,    regex: '^PCIe_\{C[0-9]x[0-9]\}\.TX[0-9]\+$', anchor: true}
  - {name: TXN,    regex: '^PCIe_\{C[0-9]x[0-9]\}\.TX[0-9]-$'}
  - {name: RXP,    regex: '^PCIe_\{C[0-9]x[0-9]\}\.RX[0-9]\+$'}
  - {name: RXN,    regex: '^PCIe_\{C[0-9]x[0-9]\}\.RX[0-9]-$'}
  - {name: CLKP,   regex: '^PCIe_\{C[0-9]x[0-9]\}\.CLK\+$'}
  - {name: CLKN,   regex: '^PCIe_\{C[0-9]x[0-9]\}\.CLK-$'}
  - {name: RST,    regex: '^PCIe_\{C[0-9]x[0-9]\}\.~\{RST\}$'}
  - {name: CLKREQ, regex: '^PCIe_\{C[0-9]x[0-9]\}\.~\{CLKREQ\}$'}
requirements:
  - {type: signal-missing}
  - {type: signal-dangling}
  - {type: esd}
```

A signal says how a net is recognized as playing that role. A `suffix` is the readable default, and a
`regex` is the escape hatch for naming a suffix cannot express, such as a bus member whose role sits
after a dot. `anchor: true` marks the signal whose presence means the bus exists.

There is no flag. The project names its own `profiles/` directory, so agni composes it for every
design under `designs/`, the same way it composes `params/` and the `conventions:` section of
`project.yaml`. Putting the file in place is the whole step:

{{ agniRun "content/tutorials/runs/05-check-profiles.yaml" }}

The findings did not move, and the considered count did. The four links were complete all along, and
now something has looked.

Ask a requirement what it concluded and it tells you, whether or not it found anything:

{{ agniRun "content/tutorials/runs/05-verdicts-pcie.yaml" }}

Each anchor, one per lane, comes back with one answer per required signal, and the list stops at
twenty passes per rule and says how many it folded away. Read what a pass claims, though. Presence
is matched across the board, so `PCIe_{C1x1}.TX0+ + CLKN` says a PCIe clock exists, not that this
link's clock does. A profile describes one bus, and four links named alike read as one
bus four lanes wide.

`esd` says nothing at all, and that is correct. Every PCIe line here runs from the module socket
J1, through its coupling capacitors, to the M.2 and FMC+ connectors, and the project's `design.yaml`
declares all of those internal, so no PCIe net leaves the product. [Rung 7](../07-your-architecture/) is about that declaration.

## Supersession, and what it costs

The note in that run says `jetson-profiles` supersedes four rules, `profile/pcie-signal-missing`,
`profile/pcie-host-incomplete`, `profile/pcie-signal-dangling` and `profile/pcie-esd-missing`.

Your profile has the same name as a built-in one, so it **replaces** it rather than running
alongside it. Every rule the built-in PCIe profile contributed is gone, and only what your file
declares is checked. Supersession is per interface, not per run, so the shipped CAN and eMMC
profiles are untouched.

Two profiles both claiming to be PCIe, both firing on the same nets, would double-report everything,
and there would be no way to say your naming differs from the built-in assumption.

But it means an omission is silent. Your file lists three requirements, and the built-in declared
four. `host-incomplete` asks whether a component DECLARED as a PCIe host carries every signal, and this
board declares no host, so the file leaves it out on purpose. Had it left out `esd` by accident, ESD
on PCIe would simply stop being checked on every board this project reviews, and nothing would say so.
The run would look healthier, because there would be fewer findings.

So write the file as the whole declaration rather than the delta, and read the supersession note
whenever you add one. It names every rule your file replaced.

## What the rung bought you

Compare the last lines of the two runs on this page. The findings are the same and the considered
count rose by about a hundred subjects, with two more rules having an opinion about this board. Those
are the PCIe signals, which before this file nothing looked at.

The second number is the one worth watching as you add tiers. Findings tell you what is wrong today,
and the considered count tells you how much of the board anything is looking at, which is what
declaring an interface buys you.

## Next

[Part limits](../06-part-limits/), where checks start comparing your design against what a part's
datasheet actually allows.

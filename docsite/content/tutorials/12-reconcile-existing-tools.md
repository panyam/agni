---
title: "12. Reconcile with the tools you already run"
description: "Bring your existing DRC or ERC report into the same model, and see where the two tools agree, differ, and cannot see each other's work."
---

You already run a design-rule check. Your CAD tool ships one, you have tuned it, and it gates your
release. The honest question about anything new is not "is it good" but "what does it add to what I
already have, and where do the two disagree".

Answering that by reading two reports side by side does not scale and nobody does it twice. So bring
the other tool's report into the same model and ask directly.

## Import the other tool's report

Run your existing check and keep the JSON:

```
kicad-cli pcb drc --format json -o kicad-drc.json designs/gateway/gateway.kicad_pcb
```

Then import it:

```
agni import-results kicad-drc.json --design designs/gateway/gateway.kicad_pcb -o kicad.results.json
```

```
kicad-cli pcb drc 10.0.4 — 309 finding(s) from designs/gateway/gateway.kicad_pcb
attached to an entity: 219 of 309
    90 not attached — board outline geometry, which belongs to no component or net
         e.g. Rectangle on Edge.Cuts

This document carries no coverage axis: a vendor report lists violations and says
nothing about what it did not check, so its silence must not be read as a pass.
```

Three things in that summary are worth reading slowly.

**219 of 309 attached.** A vendor report names things in prose, so importing it means resolving
"Track [PMIC_MAIN_12V0] on F.Cu" to the net your design calls `PMIC_MAIN_12V0`. Most resolve.

**The 90 that did not are named, with a reason.** They are board-outline geometry, which genuinely
belongs to no component or net. Nothing was dropped quietly, which matters because a silently
discarded finding is worse than one that was never imported.

**The document carries no coverage axis.** This is the same idea as [rung
9](../09-read-the-verdicts/), applied to somebody else's tool. A vendor report is a flat list of
violations. It does not say which checks ran, so it cannot distinguish "checked and fine" from
"never checked". The import records that limitation rather than papering over it, and every reader
of the document inherits the caveat.

## Compare the two runs

Now produce your own run over the same file and compare them:

```
agni check designs/gateway/gateway.kicad_pcb --results-out agni.results.json
agni results agni.results.json --compare kicad.results.json
```

```
comparing:
  ours:   agni (devel) — 20 finding(s) over designs/gateway/gateway.kicad_pcb
  theirs: kicad-cli pcb drc 10.0.4 — 309 finding(s) over designs/gateway/gateway.kicad_pcb   [no coverage axis: its silence is not a pass]

entities flagged:
  both         5
  ours only    6
  theirs only  20

ours only:
  net CAN1_RXD
  net CAN1_TXD
  net I2C_SCL
  net I2C_SDA
  net MCU_NRST
  net PMIC_EN
```

## Reading the split

The instinct is to compare 20 against 309 and conclude something about which tool is better. That
reading is wrong, and the three-way split is there to stop you making it.

**Theirs only, 20 components.** Physical manufacturability: edge clearance, silkscreen over pads,
footprint library mismatches. Agni has no opinion about most of that and should not pretend to.
Your existing DRC is not being replaced.

**Ours only, 6 nets.** Look at what they are. `I2C_SCL` and `I2C_SDA` are missing {{ explainable "pull-up" "pull-ups" }}.
`CAN1_TXD` and `CAN1_RXD` are the {{ explainable "transceiver" }}'s logic side. `PMIC_EN` and `MCU_NRST` are control
signals. Every one is a statement about what the circuit *means*, and a board DRC structurally
cannot reach any of them. It is checking copper against fabrication limits. It has no model in which
"this bus needs a pull-up" is expressible.

**Both, 5 entities.** The overlap, where the two tools genuinely agree, including the sub-floor track
on `CAN1_CANH` that both flag by their own route.

That is the useful answer to "what does this add". Not a bigger number. A different axis.

## About this board's copper

Full disclosure, because it affects the numbers above. The tutorial board's `.kicad_pcb` is
generated from the netlist rather than laid out by a person, so its geometry is crude and DRC has a
great deal to say about it. On your own board, laid out properly, the "theirs only" column will be
far shorter.

That does not change the shape of the result. The columns stay complementary, because the two tools
are answering different questions, and no amount of careful layout gives a copper checker access to
the fact that an I2C bus has no pull-up.

## Using it as a gate

`agni import-results` writes an ordinary check-result document, so everything from [rung
11](../11-archive-and-gate/) applies: archive it, re-render it later, diff it against next month's
run. A vendor report that was a terminal scroll becomes an artifact with the same shape as your own.

The comparison is the more useful gate. A finding that appears in "theirs only" and stays there for
months is a check you are not doing, and a finding that moves from "both" into "theirs only" means
one of your two tools stopped seeing something it used to.

## Next

[Drive it in the browser](../13-drive-it-in-the-browser/), the last rung.

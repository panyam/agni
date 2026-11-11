---
title: "Glossary"
description: "The terms a page can use without re-teaching. Each one hovers as a popover wherever it appears in the docsite."
---

The rest of the docsite can USE every term here rather than re-explain it. A page writes
`{{ "{{" }} explainable "termination" {{ "}}" }}` and the reader gets a hoverable link carrying the
whole definition, so the sentence can be short and nobody has to remember what they read four pages
ago.

A `learn/` chapter still teaches a term in full the first time it introduces it. Every later mention,
anywhere on the site, is a tag. Within a single page, tag the FIRST mention only and leave the rest as
plain text, since a page of dotted underlines helps nobody.

For authors, [the docsite README](https://github.com/panyam/agni/blob/main/docsite/README.md) has the
mechanics, and `terms_test.go` fails the build on a tag naming a term that does not exist, a term
missing from this index, or a term nothing references.

[The course](../../../learn/levels/) defines the levels below. Each term sits at the level whose
question it helps answer.

## Parts (EE1)

- [Reference designator](./reference-designator/), the `R5` or `U1` label, and the name every tool
  uses for that component
- [Footprint](./footprint/), the copper pads that one part solders onto, as against the designator
  that names it

## Nets (EE2)

- [Netlist](./netlist/), the list of nets that make up a board, carrying none of the drawing
- [Rail](./rail/), a net whose job is to carry a supply voltage rather than information
- [Ground](./ground/), the net every other voltage is measured against, and the return path
- [Bus](./bus/), nets that several parts share, with requirements set by a standard
- [Differential pair](./differential-pair/), two wires carrying one signal in opposite senses

## Roles (EE3)

- [Pull-up](./pull-up/), the resistor that holds a signal high when nothing is driving it
- [Pull-down](./pull-down/), the same with the other default
- [Series and shunt](./series-and-shunt/), whether a part sits in a signal's path or beside it
- [Voltage divider](./voltage-divider/), two resistors setting a net to a fraction of a rail
- [MOSFET](./mosfet/), the gate, drain, source and body diode every load-switch rule talks about
- [Decoupling capacitor](./decoupling-capacitor/), the local charge store at a chip's supply pin
- [Ferrite bead](./ferrite-bead/), a near short at DC that turns high-frequency noise into heat
- [Termination](./termination/), the resistor that stops a signal reflecting off the end of a bus
- [Test point](./test-point/), a pad that exists so a probe can reach a net
- [Transceiver](./transceiver/), the translator between logic pins and a bus's own voltages

## Failure modes (EE4)

- [Port protection](./port-protection/), what absorbs a static discharge arriving on a connector
- [Ideal diode](./ideal-diode/), a MOSFET and a controller that block reverse current with almost
  no drop

## Numbers (EE5)

- [Absolute maximum rating](./absolute-maximum-rating/), the damage threshold, as against the
  recommended operating condition, which is often confused with it
- [Derating](./derating/), running below a limit on purpose, so tolerance and temperature still
  leave margin

## Systems (EE6)

- [Power tree](./power-tree/), the cascade of regulators from input voltage to every rail
- [Strap](./strap/), a pin sampled once at reset that configures a part for good

## Layout (EE7)

- [Via](./via/), the plated hole that lets a net change layers
- [Fiducial](./fiducial/), the copper dot an assembly machine's camera finds to line the board up

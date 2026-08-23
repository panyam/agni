---
title: "Glossary"
description: "The terms a page can use without re-teaching. Each one hovers as a popover wherever it appears in the docsite."
---

Every entry here is a term the rest of the docsite can USE rather than re-explain. A page writes
`{{ "{{" }} explainable "termination" {{ "}}" }}` and the reader gets a hoverable link carrying the
whole definition, so the sentence stays short and nobody has to remember what they read four pages
ago.

A `learn/` chapter still teaches a term in full the first time it introduces it. Every later mention,
anywhere on the site, is a tag. Within a single page, tag the FIRST mention only and leave the rest as
plain text, since a page of dotted underlines helps nobody.

Authors: [the docsite README](https://github.com/panyam/agni/blob/main/docsite/README.md) has the
mechanics, and `terms_test.go` fails the build on a tag naming a term that does not exist, a term
missing from this index, or a term nothing references.

## Nets (EE2)

- [Differential pair](./differential-pair/), two wires carrying one signal in opposite senses

## Roles (EE3)

- [Termination](./termination/), the resistor that stops a signal reflecting off the end of a bus
- [Transceiver](./transceiver/), the translator between logic pins and a bus's own voltages

## Failure modes (EE4)

- [Port protection](./port-protection/), what absorbs a static discharge arriving on a connector

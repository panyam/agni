---
title: "Design for test: what can a probe actually reach"
description: "Which nets carry a test point, and which parts a tester can measure on an assembled board. Driven by the fact relations rather than the rule catalog."
actors:
  - id: you
    name: You
  - id: agni
    name: Agni
---

## What this shows

A board can be electrically correct and still impossible to debug. A test point is a bare pad whose
only job is to expose a net so something can touch it: a scope probe at bring-up, or a bed-of-nails
in the factory. This walkthrough asks what that layer covers, using queries over the design's fact
relations rather than the rule catalog.

The last step goes past what a coverage report can ask.

## Pick a design {#pick}

> The bundled fixture is `../common/designs/probe-coverage.edn`, a small board built so each coverage
> case occurs once. Point this at any design you can read. Setting `AGNI_EXAMPLE_DESIGN` changes the
> default, so a walkthrough can be driven over a board this repo does not carry without that path
> being typed or committed.

## What is on the board {#inventory}

> `component.class` derives a family from the ref-des prefix and description keywords, so test points
> separate from passives without anything being annotated.

## Which nets a probe can reach {#probed}

> One join answers it: a test point, and the net it sits on. An unprobed rail is the one that costs
> you at bring-up, because the alternative is holding a probe against a component lead.

## Which parts a tester can measure {#passives}

> Measuring a two-terminal part needs BOTH ends reachable, so one end covered measures nothing. Three
> buckets, from one pair of derived relations: both ends, one end, neither. The third needs negation
> through a unary helper, because a negated atom with an unbound variable is unsafe and returns
> nothing rather than an error.

## The parts nobody can measure, by part number {#by-mpn}

> One uncovered part is an oversight. Several of one part number is a placement habit, and that is the
> difference between a list to work through and a thing to fix once.

## What a coverage report cannot say {#beyond}

> The same GND gap is a rule, and the rule says more than the query did. A findings list names what is
> wrong. A verdict names what was ASKED: which subjects passed and why, and which the rule could not
> decide at all. "No findings" and "nobody looked" print identically in a spreadsheet.


## One gap worth knowing before you trust the counts

A thermistor is a two-terminal passive and carries no device class today, so `component.class`
yields nothing for one and it drops out of the passive buckets above. On a board with thermistors
the counts this walkthrough prints are low by that many parts. Tracked as agni issue 627.

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
only job is to expose a net so something can touch it, such as a scope probe at bring-up or a bed-of-nails
in the factory. This walkthrough asks what that layer covers, using queries over the design's fact
relations rather than the rule catalog.

The last step goes past what a coverage report can ask.

## Pick a design {#pick}

> The bundled fixture is `../common/designs/probe-coverage.edn`, a small board built so each coverage
> case occurs once. Point this at any design you can read. Setting `AGNI_EXAMPLE_DESIGN` changes the
> default. A walkthrough can then run over a board this repo does not carry, and nobody types or
> commits that board's path.

## What is on the board {#inventory}

> `component.class` derives a family from the ref-des prefix and description keywords, so test points
> separate from passives without anything being annotated.

## Which nets a probe can reach {#probed}

> `net.has_test_point` answers it. It is a derived relation from the shipped library, and it pairs
> each test point with the net it sits on. Negated over every net, it lists the nets a probe cannot
> reach. An unprobed rail costs you the most at bring-up, because you have to hold a probe against a
> component lead instead.

## Which parts a tester can measure {#passives}

> Measuring a two-terminal part needs BOTH ends reachable, so one end covered measures nothing. The
> library has the first two buckets as members, `component.probed_both` and `component.probed_one`,
> and the walk only adds what it means by a passive. The third bucket negates `net.has_test_point` on
> both nets, and `component.two_terminal` binds those nets first. So neither negated atom has an
> unbound variable. An unbound variable there would be unsafe, and the query would return nothing
> rather than an error. `agni query --relations
> component.probed_one` prints any of them with its definition.

## How one row holds {#why}

> A bucket names the parts and not the reason. `--witnesses` asks the engine to keep how each row was
> derived. The tree shows the walk's own `passive` rule, then the library member
> `component.probed_one` with each of its body facts filled in, then the `not net.has_test_point`
> that held, shown as an absence. Each fact from the design cites its source, so the tree ends in the
> netlist. In the viewer, open a row of the query panel to see the same tree. Each net and part in it
> links to the drawing.

## The parts nobody can measure, by part number {#by-mpn}

> One uncovered part is an oversight. Several of one part number is a placement habit. You work
> through a list of oversights one at a time, and you fix a habit once.

## What a coverage report cannot say {#beyond}

> The same GND gap is a rule, and the rule says more than the query did. A findings list names what is
> wrong. A verdict names what was ASKED, including which subjects passed and why, and which the rule could
> not decide at all. "No findings" and "nobody looked" print identically in a spreadsheet.

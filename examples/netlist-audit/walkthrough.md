---
title: "A netlist audit as one query set"
description: "The tables a review workbook holds, written as named queries in one file and answered over one read of the design."
actors:
  - id: you
    name: You
  - id: agni
    name: Agni
---

## What this shows

A netlist review usually ends in a workbook rather than a single answer: every part and its nets,
every part number, test points per net, which two-terminal parts a tester can measure. Each of those is a
query. Asked one at a time, each one reads the whole design again, and the questions live in
whatever script happens to call them.

A query set keeps them in one file, `audit.yaml`. The relations the tables share, such as
`net.has_test_point` and `component.probed_both`, come from the shipped library, so the file holds
only the questions. `agni query --set` answers all of them from a single read of the design.

## Pick a design {#pick}

> The bundled fixture is `../common/designs/netlist-audit.tel`, a small synthetic board where every
> table has something in it: a test point on ground, two-terminal parts probed on both nets, and some probed
> on only one. Setting `AGNI_EXAMPLE_DESIGN` points this at a board this repo cannot carry.

## The set {#the-set}

> The set has no preamble. What the tables share is in the shipped library: `net.has_test_point`,
> `component.two_terminal` (a part on exactly two nets), and `component.probed_both` and
> `component.probed_one` built from those two. `agni query --relations component.probed_one` prints
> any of them with its definition. Each query then reads as the question it asks.

## One read, every table {#answer}

> The design is read and projected once, and each query runs against the same indexes. A query that
> cannot be answered is reported under its name and the others still answer, so one typo does not
> cost the whole audit.

## As a document or a workbook {#workbook}

> `--format markdown` and `--format html` write the whole set as one document, one section per query
> with the question above its answer. `--format json` is the wire message, which the Python client
> reads to write an `.xlsx` with one sheet per query.

## What is not in the set

A set holds TABLES. A question with a pass or fail answer, such as "is every I2C line pulled up to a
rail", is a check, which `agni check --rule i2c-pull-up` answers with verdicts that also say which
lines passed and why. A house threshold, such as one ground test point per thirty nets, is two
counts from the set and one division by whoever owns the threshold, because the query language has no arithmetic
yet (panyam/jaala#5).

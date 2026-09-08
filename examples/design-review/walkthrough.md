---
title: "Design review: which of your checks can this answer"
description: "A team's schematic-review checklist, run against a board. Every item resolves to pass, fail, not-applicable, needs a declaration, or nothing covers it yet, and the last one is the number that matters."
actors:
  - id: you
    name: You
  - id: agni
    name: Agni
---

## What this shows

A review checklist is a list of questions someone decided to ask before a board is signed off. Most
tooling answers some of them and says nothing about the rest, so a clean report and an unasked
question look the same.

This walks a checklist and reports every item's outcome, including the ones nothing can answer.

## Pick a design and a checklist {#pick}

> The bundled checklist is `checklist.yaml` beside this file, and the bundled design is the I2C
> sensor at `../common/designs/i2c-sensor/i2c-sensor.edn`. Point either at your own with `AGNI_EXAMPLE_DESIGN` and
> `AGNI_EXAMPLE_REVIEW`, or by typing a path.

## How much of the list can be answered {#coverage}

> Covered counts the items a mechanism exists for. Answered counts the ones this run actually
> decided, which is the stricter number: an item whose rule exists but whose inputs are missing is
> covered and unanswered. Neither is a pass rate.

## What a failing item rests on {#drill}

> An item names a rule, and the rule states what it examined. A finding says what is wrong; a verdict
> says what was asked, which is the half a checklist needs to claim coverage of a question.

## An item nobody wrote a rule for {#house}

> A checklist item can carry its own question instead of naming a shipped rule. The house rule below
> is a query, so a team's own convention enters the review without anyone writing Go.

## The items nothing answers {#gap}

> This is the step the walkthrough exists for. Three items are not answered, for three different
> reasons, and only one of them is a gap in the tool. Deleting the unbound item would raise the
> coverage number and lower its truthfulness.

---
title: See how a design's names are read
description: List the naming vocabulary a design is read with and where each pattern came from, explain one net's roles, and see what a different convention would change
actors:
  - id: You
    label: You
  - id: Agni
    label: Agni engine
---

## A name is read before any rule runs

Most rules never look at a net's name. They ask whether a net is a rail, a ground, an I2C line, and the answer was settled once, when the design was read, by matching each name against a vocabulary of patterns. So a rail spelled in a way the vocabulary does not know is not a rail to any rule, and the rules that depend on rails go quiet or go wrong about it without saying why.

The vocabulary is built-in patterns with at most one convention laid over them. The convention can come from the server's `--conventions`, the design's project, the design's own `design.yaml`, or the request, and the nearest one wins.

## Pick a design {#design}

> A path to a design folder or file, relative to this folder. The default is ../common/designs/lexicon-demo, a sensor that runs from its own regulator, whose output is named SENSOR_3V3. Its design.yaml carries a convention adding one rail pattern.

## The vocabulary, and where each pattern came from {#lexicon}

> GetLexicon lists every pattern, exclusion, ref-des prefix and part-number family the read used. Each one is either built in or from the convention, and a built-in pattern a convention switched off with `replace: true` stays in the list, marked, so you can see it went. Here the design's convention adds one rail pattern to the built-in ones.

```mermaid
You ->> Agni: GetLexicon(snapshot)
Agni -->> You: entries, each with its source
```

## Why a net has its roles {#net}

> ExplainNet gives the roles the read stamped on one net, each with its reason, and every pattern the name matches, with where that pattern came from. SENSOR_3V3 is a rail because of the design's pattern and no other.

```inputs
- name: net
  prompt: Net name
  type: string
  default: SENSOR_3V3
```

## What taking the convention away would change {#try}

> CompareLexicons reads the design again with a candidate convention in place of the one it was read with, runs the checks under both, and lists the nets whose roles change and the findings that appear or disappear. The candidate here states no lexicon, so the names are read with the built-ins alone. SENSOR_3V3 stops being a rail, and the I2C pull-ups that go to it no longer count, so both bus lines fail.

```mermaid
You ->> Agni: CompareLexicons(snapshot, candidate)
Agni -->> You: role_changes, finding_changes
```

## Where to go next

`agni lexicon <design>` asks the same three questions from the command line, with `--net` and `--try <conventions.yaml>`, and `--format yaml` writes the effective vocabulary as a conventions file you can edit and carry in a project.

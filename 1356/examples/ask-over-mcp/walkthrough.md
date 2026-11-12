---
title: Ask a design questions the way an LLM client does
description: Start agni's MCP server over a folder and make the calls a model makes, reading back what it reads
actors:
  - id: You
    label: You (the model)
  - id: Agni
    label: agni mcp
---

## The model asks, agni answers

An LLM connected to agni never states a fact about the board on its own. It writes a question, agni answers it from the design, and the answer comes back as rows, each with the file it was read from. This walkthrough makes the same calls a model would, in the order one usually does: find the design, look up a relation, ask, then follow a signal and compare two revisions.

## Start the server {#connect}

> A path to the folder to serve. The default is ../tutorial-project, the project the docsite's tutorial runs on, and the walkthrough asks about its designs/gateway design. Give a file instead of a folder, and agni serves the file's own folder and asks the questions of that file. This starts agni mcp as a subprocess (AGNI_BIN, then this repo's bin/agni, then the agni on PATH) and lists the tools it offers.

```mermaid
You ->> Agni: initialize, tools/list
Agni -->> You: query, schema, look, values, explain, check, trace, diff, ...
```

## Find the design {#designs}

> designs lists every design the server answers about, with its declared revisions. Each design appears under the name that the other tools take. A model that doesn't know a design's name starts here.

## Look before asking {#look}

> look describes one relation: its signature, its documentation and, for a derived one, the rules that define it. net.has_test_point is from the shipped library, so the answer shows the Datalog behind it.

## Ask {#query}

> The rails with no test point. The answer opens with the design and the question, then the rows, each with the file it was read from, then how many rows there are and why this run has no viewer link (agni mcp links only when --server names a running agni serve).

```mermaid
You ->> Agni: query(design, program)
Agni -->> You: question, rows with provenance, link or why not
```

## Follow a signal {#trace}

> The tutorial gateway's input connector pin J1.1 to the regulator's pin U1.1, through whatever series parts sit between them, with the probe points on each net.

## Compare two revisions {#diff}

> Rev B added pull-ups on the I2C bus, so the diff names the parts it added and the nets whose connections changed.

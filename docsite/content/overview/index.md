---
title: "What is Agni"
description: "The engine, its two personas, and how the documentation is organized."
---

Agni is an EDA tooling engine. It reads electronic designs (schematics and boards) from several
industry formats into one neutral intermediate representation, and then runs analysis over that
one representation: rule checks, semantic diff between revisions, datalog queries, and faithful
rendering.

The central idea is **many producers, one contract, many consumers**. Each format reader
(EDIF, KiCad, IPC-2581, and others) targets the same neutral IR. Everything downstream (checks,
diff, query, render) is written once against that IR and works for every format. The same shape
repeats for datasheets: many extractors, one parameter contract, one set of datasheet-aware
checks.

## Two personas

Agni is open core. The engine is open source under Apache-2.0; work that is private to a company
lives in a separate overlay that depends on the engine.

- **You use Agni.** You bring a design and run checks, diffs, and queries. You do not need to read
  any Go. Start with [Use it](../guide/).
- **You build on Agni.** You add a format reader, author a check rule, or write a private overlay
  with your own readers and house rules. Start with [Build on it](../build/).

## How these docs are organized

- **[Use it](../guide/)** &mdash; task guides for running the tool.
- **[Build on it](../build/)** &mdash; how to extend the engine.
- **[Understand it](../architecture/)** &mdash; how the internals fit together, by subsystem.
- **[Design decisions](../decisions/)** &mdash; the rationale behind the larger choices.
- **[Reference](../reference/)** &mdash; the software-to-hardware analogy and format primers.

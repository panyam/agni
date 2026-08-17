---
title: "What is Agni"
description: "Where Agni came from, who it is for, and how these docs are organized."
---

Agni began as a way to learn hardware design by building tooling for it. I am a software
engineer, and reading a domain's files and reasoning about them in code is how I tend to learn
a domain. The idea was to read real schematics and board layouts, turn them into a
representation a program can work with, and then see what ordinary software techniques like an
intermediate representation, static checks, queries, and structured diffs can say about a
hardware design.

So Agni is closer to a playground than a finished product. It is useful for looking at a design,
running some analysis over it, and understanding what is there. It is not trying to replace a
professional EDA suite, and it does not claim to catch everything a real design review would.
The interesting part is the process of learning the domain by making software analysis work on
real files, and adding more analysis as the understanding grows.

The whole thing is built for a software engineer's eyes. Hardware has its own vocabulary, and
most of it maps onto something a programmer already knows. A netlist is a graph. A bill of
materials is a lockfile. A datasheet is a type definition with runtime limits. These mappings
carry real weight here. The docs explain hardware through them, and they are how someone from
software can pick up a real design and reason about it. The [software analogy](../reference/analogy/)
collects them in one place, and the [concepts](../guide/concepts/) page in the user guide is
the read-first version.

## What it does today

- reads EDIF, KiCad, and IPC-2581 into one neutral representation,
- runs a catalog of electrical and integrity checks, each finding cited to its evidence,
- compares two revisions as a structured diff,
- answers datalog queries about nets, parts, copper, and datasheet limits,
- renders schematics and boards in the browser.

## Two ways in

- You want to look at or check a design. Start with [Use it](../guide/). No Go required.
- You want to add analysis, a new format, or a rule. Start with [Build on it](../build/).

## How these docs are organized

- **[Use it](../guide/)** covers running the tool on a design.
- **[Build on it](../build/)** covers adding readers, rules, or an overlay.
- **[Understand it](../architecture/)** covers how the internals fit together, by subsystem.
- **[Reference](../reference/)** holds the software-to-hardware analogy and the format primers.

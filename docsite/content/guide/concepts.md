---
title: "Concepts, if you come from hardware"
description: "The handful of software ideas Agni introduces, each mapped back to something on your bench."
---

You already know what a resistor, a net, and a design rule are. This page is not about
those. It is about the handful of **software ideas the tool introduces**, each mapped
back to something on your bench so the words stop being jargon.

Every entry has the same three parts:

- **What the tool calls it**: the word you will see in the docs and the UI.
- **What it's like for you**: the hardware or lab intuition.
- **Why it matters**: the practical consequence when you run the tool.

Agni is a software engineer's attempt to learn hardware design by building tooling for it,
so it leans on ordinary software ideas throughout. For the reverse map, circuit ideas
explained to a software engineer, see [the software analogy](../../reference/analogy/).

## IR (the internal representation)

**What the tool calls it:** the IR, or "the design model."

**What it's like for you:** one normalized {{ explainable "netlist" }}-plus-BOM that every
EDA export collapses into. KiCad, Allegro, OrCAD, and an EDIF dump all land in the same
shape, the way a bench multimeter reads volts the same whether the board came from any of
them.

**Why it matters:** a rule you write, or a report you read, behaves identically no matter
which tool exported the design. You learn the tool once, not once per CAD package. When a
finding looks wrong, the question is "did the IR capture my design faithfully," and the
tool tells you which parts of the source it read and which it dropped.

## Tiers (what the tool actually has to work with)

**What the tool calls it:** the Model and its tiers, a netlist tier, a board tier, a
parameters tier.

**What it's like for you:** each tier is a *fact source you either connected or didn't*.
No `.kicad_pcb` loaded means there is no copper to run clearance against, the same way a
DNP {{ explainable "footprint" }} contributes no connection to the circuit. The tier is not
broken, it is simply empty.

**Why it matters:** **silence is not a pass.** A quiet board tier means "I had no layout
to check," not "your layout is clean." The report names which tiers were live, so you can
tell a real all-clear from a check that never ran. Feed it more inputs (a board file, a
datasheet set) and more tiers light up.

## Checks and rules

**What the tool calls it:** the check catalog. Each entry is a rule.

**What it's like for you:** ERC and DRC, except the catalog is open. The built-in rules
cover the general electrical cases (decoupling present, supply exceeds a part's abs-max,
copper clearance, {{ explainable "port-protection" "ESD protection" }} on exposed nets). You can add your own house rules on
top without touching the engine.

**Why it matters:** the rules your team argues about in review ("every regulator gets a
bulk cap," "USB nets get ESD") stop being tribal knowledge and become something every
export is checked against automatically.

## Findings, severity, and provenance

**What the tool calls it:** a finding, with a severity and its provenance.

**What it's like for you:** a DRC violation with a callout, but every finding also carries
a *paper trail*. Provenance is the finding pointing back at exactly what it saw: which
net, which component pin, and for a datasheet check, which page and table of the vendor
PDF it read the limit from.

**Why it matters:** you never have to take a finding on faith. "Supply exceeds abs-max"
cites your +24V rail *and* "SNOS412Q page 4, Absolute Maximum Ratings," so you can open
the datasheet to that page and confirm it yourself. Severity tells you what blocks a build
versus what is a note.

## Datasheets as checkable data

**What the tool calls it:** a parameter set (`PartSpec` values). A datasheet becomes a
`doc-IR`.

**What it's like for you:** the {{ explainable "absolute-maximum-rating" "abs-max" }} and
operating limits you would otherwise read off a PDF by eye, turned into numbers the tool
can compare against your design. Think of it as transcribing the "Absolute Maximum Ratings"
table once, with the conditions attached, so it can be checked a thousand times.

**Why it matters:** the tool can catch "you are driving 24V into a part rated for 20V"
before the board is built, and show you the exact row it read. A limit that has only a
text condition ("at 25°C ambient") is flagged for a human rather than auto-compared, so
the tool never pretends to a certainty it does not have.

## Diff (comparing two revisions)

**What the tool calls it:** `agni diff`.

**What it's like for you:** a redline between two schematic or board revisions. Component
added, removed, value changed. Net created, deleted, renamed. It understands that a net
kept its connections under a new name and calls that a rename, not a delete-plus-add.

**Why it matters:** review "what actually changed between rev A and rev B" as a precise
list instead of eyeballing two prints side by side.

## Naming conventions as patterns

**What the tool calls it:** a conventions file (allow / exempt regex sets).

**What it's like for you:** your net and {{ explainable "reference-designator" "ref-des" }}
house style, written down once as patterns instead of a wiki page nobody reads. "Power nets
look like `+3V3`," "diff pairs end in `_P`/`_N`," "no ref des reused."

**Why it matters:** the style is enforced on every export automatically, and tool-generated
stub names (the `N$…` autonames CAD tools invent) are exempt by default, so you are only
flagged on names a human chose.

## Where to go next

- Run these ideas for real: [Getting started](../getting-started/).
- The deeper design rationale behind each concept lives in the developer docs. You do not
  need them to use the tool, but they are there when you want the why.

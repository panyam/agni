# User guide

This guide is for people who **use** the tool: hardware engineers who run reports,
load datasheets, compare revisions, and encode a house style. You do not need to read
any Go to get value here.

If you want to **extend** the engine (add a format reader, a check rule, a render
backend), that is the developer track. It lives in the numbered docs (`docs/13` to `docs/24`)
today and will get its own front door at `docs/devguide/` later.

## Start here

- **[Concepts, if you come from hardware](concepts.md)**: the one page worth reading
  first. The circuit parts are not the mystery; the software ideas the tool wraps around
  them are (an "IR", "tiers", "provenance"). This maps each of those back to something you
  already know. Every other page assumes these terms.
- **[Getting started](getting-started.md)**: install, run your first check on a bundled
  sample, read the report.

## Tasks

- **[Checks and reports](checks-and-reports.md)**: run the rule catalog, read findings,
  severity, and where each finding came from.
- **[Querying your design](querying.md)**: `agni query` — search your design as
  data (nets, parts, datasheets, board copper) with ad-hoc questions, each answer cited.
- **[Datasheets](datasheets.md)**: load a parameter set and let checks compare your
  design against a part's real limits.
- **[Comparing revisions](comparing-revisions.md)**: `agni diff` as a schematic redline.
- **[Naming conventions](naming-conventions.md)**: encode your net and ref-des house
  style so every export is checked against it.

## Reference

- **[CLI reference](cli-reference.md)**: the command surface.

---

A companion page, [`docs/ANALOGY.md`](../ANALOGY.md), runs the *opposite* direction: it
explains circuit concepts to software engineers. If you ever pair with the tool's
developers, that is the shared vocabulary you both point at.

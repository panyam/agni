---
title: "Tutorials"
description: "One board, carried from first read to a house checklist gating CI."
---

The [guide](../guide/) documents each feature on its own, which is what you want when you know the
name of the thing you need. These pages are the other shape: one board, carried from first read all
the way to a house checklist running in CI, adding one capability at a time.

Work through them in order the first time. After that they stand alone.

## The board

Every rung runs against `examples/tutorial-project` in the engine repo. It is a synthetic automotive
gateway ECU plus the project files a team wraps around one. Every part, MPN, and datasheet value in
it is invented, so you can copy the whole folder and change it freely.

```
git clone https://github.com/panyam/agni
cd agni/examples/tutorial-project
make review
```

The folder is checked in complete, with every file present. Each rung below tells you which file it
is about and passes only the flags earned so far, so you can start at any rung and it will run. If
you would rather build it up yourself, delete `conventions.yaml`, `profiles/`, `params/`, and
`designs/gateway/intent.yaml` and add them back as you go.

The board is deliberately imperfect. Each flaw is a real defect a reviewer would flag, and each one
exists so some part of the tool has something true to report.

## The rungs

**Evaluate.** Does it read my board, and what does it say?

1. [Read a design](01-read-a-design/): confirm the tool read your board the way you expect, before
   trusting anything downstream.
2. [Run the catalog](02-run-the-catalog/): the built-in rules, how to read a finding, and how to
   fail a build on one.
3. [See it](03-see-it/): draw the board, and get a picture of a netlist that has no drawing.

**Teach it your house.** Four independent tiers, one per rung. Stop after any of them and the ones
you added still work.

4. [Your names](04-your-names/): which nets are rails, and what a legal name looks like here.
5. [Your interfaces](05-your-interfaces/): declare a bus once, check every board against it.
6. [Part limits](06-part-limits/): compare the design against what the datasheet actually allows.
7. [Your architecture](07-your-architecture/): declare what the board is supposed to be, and detect
   drift from it.

**Run your review.**

8. [Write your checklist](08-write-your-checklist/): the questions your team asks of every board,
   bound to the engine.
9. [Read the verdicts](09-read-the-verdicts/): why a question nobody answered must not score as a
   pass.
10. [Compare revisions](10-compare-revisions/): what changed between rev A and rev B, structurally.
11. [Archive and gate](11-archive-and-gate/): keep the result, re-render it later, fail CI on it.

**Live with it.**

12. [Reconcile with the tools you already run](12-reconcile-existing-tools/): import your existing
    DRC or ERC report and see where the two tools agree, differ, and cannot see each other's work.
13. [Drive it in the browser](13-drive-it-in-the-browser/): the same catalog and the same verdicts,
    against the drawing instead of a terminal.

## Running this on your own board

The tutorial project is laid out the way a real review project is laid out, so each step maps to the
same step on your own design by changing which files it points at.

| Rung | In the tutorial | On your project |
|---|---|---|
| 1 | `make stats` | point `designs/<name>/design.yaml` at your netlist, and list your board and schematic exports under `companions` |
| 2 | `make check` | same command, your design folder |
| 4 | the bundled `conventions.yaml` | your team's rail names and naming rules |
| 5 | the bundled `profiles/can.yaml` | one file per bus your team designs with |
| 6 | the bundled `params/` | a seeded PartSpec per part worth checking |
| 7 | `designs/gateway/intent.yaml` | one per design, since each board has its own architecture |
| 8 | the bundled `review.yaml` | your team's checklist |

The split that matters is that conventions, profiles, and parameters describe the *team*, so they
sit at the project root and are shared by every design. Intent describes one *board*, so it sits
beside that board.

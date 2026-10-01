---
title: "Tutorials"
description: "One board, carried from first read to a house checklist gating CI."
---

The [guide](../guide/) documents each feature on its own, so reach for it when you already know the
name of the thing you need. These pages take the other approach and carry one board from first
read all the way to a house checklist running in CI, adding one capability at a time.

Work through them in order the first time. After that they stand alone.

These rungs teach the TOOL and assume the domain. If the checks keep making sense mechanically while
the engineering behind them does not, [learn the domain](../learn/) covers the other axis in seven
levels of hardware knowledge, each tied back to the rules that encode it.

## The board

Every rung runs against `examples/tutorial-project` in the engine repo. It is a synthetic industrial
sample board plus the project files a team wraps around one. Every part, MPN, and datasheet value in
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

{{ includeFile "figures/tutorial-ladder.svg" }}

Rungs 1 to 3 evaluate the tool. Does it read my board, and what does it say?

1. [Read a design](01-read-a-design/) confirms the tool read your board the way you expect,
   before you trust anything downstream.
2. [Run the catalog](02-run-the-catalog/) runs the built-in rules, shows how to read a finding,
   and fails a build on one.
3. [See it](03-see-it/) draws the board, and gets a picture of a netlist that has no drawing.

Rungs 4 to 7 teach it your house, as four independent tiers, one per rung. Stop after any of them
and the ones you added still work.

4. [Your names](04-your-names/) says which nets are rails and what a legal name looks like here.
5. [Your interfaces](05-your-interfaces/) declares a bus once and checks every board against it.
6. [Part limits](06-part-limits/) compares the design against what the datasheet actually allows.
7. [Your architecture](07-your-architecture/) declares what the board is supposed to be, and
   detects drift from it.

Rungs 8 to 11 run your review.

8. [Write your checklist](08-write-your-checklist/) binds the questions your team asks of every
   board to the engine.
9. [Read the verdicts](09-read-the-verdicts/) explains why a question nobody answered must not
   score as a pass.
10. [Compare revisions](10-compare-revisions/) shows what changed between rev A and rev B,
    structurally.
11. [Archive and gate](11-archive-and-gate/) keeps the result, re-renders it later, and fails CI
    on it.

Rungs 12 and 13 are about living with it.

12. [Reconcile with the tools you already run](12-reconcile-existing-tools/) imports your existing
    DRC or ERC report and shows where the two tools agree, differ, and cannot see each other's work.
13. [Drive it in the browser](13-drive-it-in-the-browser/) puts the same catalog and the same
    verdicts on the drawing instead of a terminal.

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

Conventions, profiles, and parameters describe the *team*, so they
sit at the project root and are shared by every design. Intent describes one *board*, so it sits
beside that board.

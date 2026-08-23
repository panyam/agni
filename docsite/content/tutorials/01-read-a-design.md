---
title: "1. Read a design"
description: "Confirm the tool read your board the way you expect, before trusting anything downstream."
playground: viewer
---

Every finding in every later rung rests on one thing: that the tool read your board correctly. A
schematic that half-loaded still produces a report, and that report looks exactly like a real one.
So the first thing to do with a new design is confirm what was read, not check it.

Run these from `examples/tutorial-project`.

## The board

This is the design every rung runs against. Drag to pan, scroll to zoom.

<agni-viewer src="{{.Site.PathPrefix}}/static/designs/gateway-schematic.svg"
             caption="Sample Board: 12V in, 3V3 and 1V8 rails, an MCU, a CAN transceiver, an I2C EEPROM, and a crystal"></agni-viewer>

A small industrial gateway. Power comes in from a connector at 12 V, a buck regulator drops it to
3.3 V and an LDO drops that to 1.8 V. An MCU sits on both rails and talks to a CAN transceiver, an
I2C EEPROM, and a crystal.

It is deliberately imperfect. Each flaw is a real defect a reviewer would flag, and each one is
there so some part of the tool has something true to report rather than a contrived one. You will
meet them as you go.

This drawing is rendered by Agni from the same file the commands below read. It is not a screenshot
of another tool.

## What was read

{{ agniRun "content/tutorials/runs/01-stats-designs-gateway-gateway-edn.yaml" }}

You already know roughly how many parts and nets your board has. Compare against that number first.
A component count that is half what you expect means a library did not resolve, and every rule that
quantifies over parts will quietly under-report. Nothing downstream will tell you, because a rule
that finds no members produces no findings, which is indistinguishable from a rule that found
nothing wrong.

`sections` counts source instances and `components` counts distinct reference designators. They
differ when one part is drawn as several gates across sheets, which is normal. `multi-section` is
how many parts that applies to.

## Did anything fail to load

`stats` tells you what came through. `validate` tells you whether it holds together:

{{ agniRun "content/tutorials/runs/01-validate-designs-gateway-gateway-edn.yaml" }}

It takes a directory too, and that form earns its keep when you have just been handed a folder of
exports and want to know which of them the tool can actually read.

A file with an extension nothing claims is reported as *skipped*, not failed, so a silently skipped
file never becomes a missing report.

## The failure that costs the most

The tutorial board ships a second view of itself, `gateway.kicad_sch`, whose symbols live in a
separate library file rather than being embedded. That is normal practice and it is the setup for
the most common bad read there is.

{{ agniRun "content/tutorials/runs/01-stats-kicad-sch.yaml" }}

Nineteen components and **zero nets**. The parts were found, their symbols were not, so no pins
resolved, so nothing is connected to anything. Point `--symbol-path` at the library and the same
file reads correctly:

{{ agniRun "content/tutorials/runs/01-stats-kicad-sch-symbols.yaml" }}

The flag takes a directory and searches its whole subtree, so pointing it at a library root is
enough. A KiCad project's `sym-lib-table` is picked up automatically.

Now the part worth sitting with. Run the checks on the broken read and it does not error, it does
not warn you that it read nothing, and it does not stay quiet:

```
agni check designs/gateway/gateway.kicad_sch
```

```
findings by rule:
  bulk-cap               1
  cap-voltage            1
  crystal-load-caps      1
  ...
94 finding(s) total
```

Ninety-four findings, against nine on the same board read correctly. Every one of them is an
artefact of the bad read. Nothing in that output says "I could not resolve your symbols". It looks
like a board in serious trouble, and a reader who skipped `stats` would spend an afternoon on it.

That is why this rung is first.

## A summary you can share

`agni intake` produces a description of the design that is safe to hand to someone who should not
see the design itself. It carries counts, classes, rail voltages, and the parts list, and it
structurally cannot carry a net name or a connection.

{{ agniRun "content/tutorials/runs/01-intake-params.yaml" }}

Read the class summary as a second opinion on the read. `unclassified: 0` means every part was
recognized as something. A large unclassified count is the same warning as a low component count,
arriving from a different direction.

The rails section prints nominal voltages and withholds the net names, following the pattern of
the whole command: the shape of the design crosses the boundary, the design does not.

`--params` is what populates the MPN column and the datasheet-gap list. Without a parameter set
those columns stay empty by design, since part numbers come from the seeded corpus rather than being
lifted out of the design.

## Then read the gaps

With `--params` attached, the tail of the intake names every part on the board with no seeded
datasheet:

```
## Datasheet gaps (MPN on board, no seeded spec)
- ACME-CAN-XCVR
- ACME-CAP-100N
- ACME-CAP-10U
...
```

That list is the work queue for [rung 6](../). Every part on it is one whose real limits nothing can
currently check against.

## Next

[Run the catalog](../02-run-the-catalog/), now that you trust the read.

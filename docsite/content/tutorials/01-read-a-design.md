---
title: "1. Read a design"
description: "Confirm the tool read your board the way you expect, before trusting anything downstream."
---

Every finding in every later rung rests on one thing: that the tool read your board correctly. A
schematic that half-loaded still produces a report, and that report looks exactly like a real one.
So the first thing to do with a new design is not to check it. It is to confirm what was read.

Run these from `examples/tutorial-project`.

## What was read

```
agni stats designs/gateway/gateway.edn
```

```
design:              GATEWAY
source format:       edif-2.0.0
libraries:           2
components:          19 (unique ref_des)
sections:            19 (source instances)
multi-section:       0 (one ref_des, several sections)
nets:                15
```

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

```
agni validate designs/gateway/gateway.edn
```

```
FILE                         FORMAT  STATUS  DETAIL
designs/gateway/gateway.edn  edif    ok      19 comps, 15 nets

1 passed, 0 failed, 0 skipped (no reader)
```

It takes a directory too, which is the useful form when you have just been handed a folder of
exports and want to know which of them the tool can actually read.

A file with an extension nothing claims is reported as *skipped*, not failed, so a silently skipped
file never becomes a missing report.

## The failure that costs the most

The tutorial board ships a second view of itself, `gateway.kicad_sch`, whose symbols live in a
separate library file rather than being embedded. That is normal practice and it is the setup for
the most common bad read there is.

```
agni stats designs/gateway/gateway.kicad_sch
```

```
source format:       kicad-sch
libraries:           0
components:          19 (unique ref_des)
sections:            19 (source instances)
multi-section:       0 (one ref_des, several sections)
nets:                0
```

Nineteen components and **zero nets**. The parts were found, their symbols were not, so no pins
resolved, so nothing is connected to anything. Point `--symbol-path` at the library and the same
file reads correctly:

```
agni stats designs/gateway/gateway.kicad_sch --symbol-path designs/gateway/symbols
```

```
source format:       kicad-sch
libraries:           1
components:          19 (unique ref_des)
sections:            19 (source instances)
multi-section:       0 (one ref_des, several sections)
nets:                15
```

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

```
agni intake designs/gateway/gateway.edn --params params
```

```
## Aggregates
- Components: 19 | Sections: 19 | Nets: 15

## Class summary (query-derived)
| Class | Ref count |
|-------|-----------|
| capacitor | 6 |
| ic | 5 |
| resistor | 3 |
| test_point | 2 |
| clock | 1 |
| connector | 1 |
| diode | 1 |
| tvs | 1 |
| unclassified | 0 |

## Rails (nominal only; net names withheld)
- 1.8V
- 3.3V
- 12V
```

Read the class summary as a second opinion on the read. `unclassified: 0` means every part was
recognized as something. A large unclassified count is the same warning as a low component count,
arriving from a different direction.

The rails section prints nominal voltages and withholds the net names, which is the pattern the
whole command follows: the shape of the design crosses the boundary, the design does not.

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

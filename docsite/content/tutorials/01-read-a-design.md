---
title: "1. Read a design"
description: "Confirm the tool read your board the way you expect, before trusting anything downstream."
playground: viewer
---

Every finding in every later rung assumes the tool read your board correctly. A
schematic that half-loaded still produces a report, and that report looks exactly like a real one.
So the first thing to do with a new design is confirm what was read, not check it.

## The board

This rung runs on a real board, Antmicro's open-source baseboard for the NVIDIA Jetson AGX Thor
module, drawn in KiCad 9 and published under Apache-2.0. It is not in the repo, because it is
someone else's design, so fetch it first:

```
git clone https://github.com/panyam/agni
cd agni
make samples-oracle
cd tools/samples/boards/jetson-agx-thor-baseboard
```

Run every command on this page from that folder. Here is one of its fifteen sheets, the power
entry. Drag to pan, scroll to zoom.

<agni-viewer src="{{.Site.PathPrefix}}/static/designs/jetson-power.svg"
             caption="The Power sheet: the 24 V input J2 with its TVS D2, three LTC4412 ideal-diode controllers selecting between the input and two USB-C supplies, a 3.3 V standby LDO, and the power LEDs"></agni-viewer>

A carrier board takes a compute module through one large connector and breaks its interfaces out to
the outside world. This one carries USB-C, an SFP cage, two M.2 sockets, two camera connectors and an
FMC+ expansion header, and it makes every rail those need from one DC input. It has 1123 parts on
fifteen sheets.

This drawing is rendered by Agni from the same files the commands below read. It is not a screenshot
of another tool.

## What was read

{{ agniRun "content/tutorials/runs/01-stats-jetson.yaml" }}

You already know roughly how many parts and nets your board has. Compare against that number first.
A component count that is half what you expect means a library or a sheet did not resolve, and every
rule that quantifies over parts will quietly under-report. Nothing downstream will tell you, because
a rule that finds no members produces no findings, which is indistinguishable from a rule that found
nothing wrong.

`sections` counts source instances and `components` counts distinct
{{ explainable "reference-designator" "reference designators" }}. They differ when one part is
drawn as several units, which is normal. `multi-section` is how many parts that applies to.

The folder holds one `design.yaml`, which names the root schematic as the design's entry and the
`.kicad_pcb` as the same board's copper. That is why naming the folder reads the whole design.

## Did anything fail to load

`stats` tells you what came through. `validate` tells you whether it holds together, file by file:

{{ agniRun "content/tutorials/runs/01-validate-jetson.yaml" }}

Every file reads. The board and the project file count 1387 nets against the schematic's 1355,
because each view of a board names and splits nets its own way, and the design's count is the
schematic's, since it is the entry. The list is the folder rather than the design, so it includes each sub-sheet read on
its own, the project file, the board, and `usb.kicad_sch`, which sits in the folder although the root
never includes it. That form earns its keep when you have just been handed a folder of exports and
want to know which of them the tool can actually read.

A file with an extension nothing claims is reported as *skipped*, not failed, so a silently skipped
file never becomes a missing report.

## The failure that costs the most

The board is fifteen files. Someone working on its power tree opens `som_power.kicad_sch` and points
the tool at the file in front of them:

{{ agniRun "content/tutorials/runs/01-stats-som-power.yaml" }}

The design's own name, a hundred and sixteen components and sixty-four nets. Nothing says that this
is one sheet of fifteen. A sub-sheet is a complete KiCad file with its own parts and wires, and the
connections it makes to other sheets are by name, so read alone it is a small, plausible design.

Now run the checks on it. It does not error, and it does not stay quiet:

{{ agniRun "content/tutorials/runs/01-check-som-power.yaml" }}

The design read whole, for comparison:

{{ agniRun "content/tutorials/runs/01-check-jetson-total.yaml" }}

More findings from one sheet than from the whole board. All 372 `unconnected-pin` findings are pins of
the module connector J1, whose signals go to the other fourteen sheets, which this read never opened. The
`exposure-declared` warnings are the one clue, saying the design has no J10, no J11, no J13. Nothing in
the output says "you read one sheet of a hierarchy". It looks like a board in serious trouble, and a
reader who skipped `stats` would spend an afternoon on it.

Name the design, the folder or its root schematic, and the hierarchy is walked from the top.

That is why this rung is first.

## A summary you can share

`agni intake` produces a description of the design that is safe to hand to someone who should not
see the design itself. It carries counts, classes, rail voltages, and the parts list, and it
structurally cannot carry a net name or a connection.

{{ agniRun "content/tutorials/runs/01-intake-jetson.yaml" }}

Read the class summary as a second opinion on the read. A large `unclassified` count is the same
warning as a low component count, arriving from a different direction. Here it is 23, and none of
them is a part a rule needs a class for. They are thirteen standoffs, the three mounts for the module and the
M.2 cards, six buttons, and a coin-cell holder.

The rails section prints nominal voltages and withholds the net names, following the pattern of
the whole command, which lets the structure of the design cross the boundary and keeps the design
itself back.

## Then read the gaps

The intake's parts table carries each part's MPN, because this board's schematic records them. What
it cannot say yet is which parts have a datasheet behind them. That needs a parameter set, a
project's `params/` or `--params <dir>`, and with one attached the intake ends with the parts that
have none. That list is the work queue for [rung 6](../06-part-limits/).

## Next

[Run the catalog](../02-run-the-catalog/), now that you trust the read.

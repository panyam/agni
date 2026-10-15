---
title: "7. Your architecture"
description: "Declare what the board is supposed to be, and detect when it stops being that."
---

The three tiers so far describe your team. Naming, buses, and parts are the same across every board
you build. This one describes one board, and it is the only tier that can catch a
design drifting from what it was meant to be.

The engine has no built-in opinion about your architecture. It cannot know that a rail was supposed
to be 1.8 V, because a netlist records what is connected and never records what anyone intended. So
you declare it, and the declaration becomes checkable.

The declaration lives in the design's own descriptor, under `intent:`, rather than at the project
root.

## The declaration

The `intent:` section of `designs/jetson/design.yaml` says two things about this board.

```yaml
intent:
  components:
    J1:  {exposure: internal}   # Jetson AGX Thor module, board-to-board
    J8:  {exposure: internal}   # FMC+ expansion power, mates with J18's accessory
    J18: {exposure: internal}   # FMC+ expansion mezzanine
    J13: {exposure: internal}   # M.2 key-M, NVMe card
    J14: {exposure: internal}   # M.2 key-E, wireless card
    J10: {exposure: internal}   # CSI camera FFC, cabled to a camera inside the product
    J11: {exposure: internal}   # CSI camera FFC
    J15: {exposure: internal}   # fan
    J4:  {exposure: internal}   # Tag-Connect debug pads, used in manufacturing
    J20: {exposure: internal}   # Tag-Connect debug pads
  nets:
    +5V:      {nominal: 5.0, domain: main}
    +3V3:     {nominal: 3.3, domain: io}
    +1V8:     {nominal: 1.8, domain: logic}
    +5V_SOM:  {nominal: 5.0, domain: module}
    +5V_AON:  {nominal: 5.0, domain: standby}
    +3V3_AON: {nominal: 3.3, domain: standby-io}
```

Read it as the sentences you would say describing the board to a colleague. These ten connectors join
this board to other parts of the same product, and every other connector takes a cable from the
outside world. These are the supplies, at these voltages, each in a named domain.

## Which connectors face the field

The first half matters more than it looks. Every rule about protection asks whether a net reaches the
outside world, and a netlist cannot answer that. A module socket and a USB-C receptacle are both
connectors, and the same part number can be the cable entry on one carrier and the socket for a
daughterboard on another. So every connector counts as exposed until the design says otherwise.

Without the section:

{{ agniRun "content/tutorials/runs/07-check-intent-off.yaml" }}

With it:

{{ agniRun "content/tutorials/runs/07-check-intent.yaml" }}

Hundreds of findings disappear. Nearly all of them were `esd-protection`, on nets reaching the module
socket J1, whose pins carry most of the board's signals, the FMC+ header J18, and behind those the
M.2 sockets and the two camera connectors. Rung 5's PCIe profile
loses its `esd` findings for the same reason. None of those findings was wrong about the netlist, and
all of them were wrong about the board. Ten lines of intent turned a protection report nobody would
read into one worth reading, and the ones that remain are the connectors a user really can plug a
cable into, which are exactly the ones a reviewer wants to see.

## Rails and their domains

The second half declares each supply's voltage and the domain it belongs to. A domain carries one
voltage, so two rails in it must agree, and each rail's name must agree with what was declared:

{{ agniRun "content/tutorials/runs/07-verdicts-domains.yaml" }}

Today the board and the plan agree, and each pass says what was compared. The tier earns its keep the
day they stop agreeing. Here is the plan edited, on a scratch copy, to say the logic domain runs at
3.3 V while the board still runs it at 1.8 V:

{{ agniRun "content/tutorials/runs/07-check-drift.yaml" }}

Nothing structural is wrong with the board, and no rule from any other tier has anything to say. The
only reason this is catchable is that somebody wrote down what was intended and the two disagree.

This tier finds divergence between the board and the description of the board rather than defects in
the usual sense, and that divergence creeps in over months as a design is edited by people who did not
write the original plan.

## A tier can depend on another tier

`modules` is the third thing a declaration can say: the board has two regulators, or a CAN block made
of these nets. A module written in terms of a device class (`class: regulator`) is only as good as the
classification underneath it, and on this board nothing classifies as a regulator, because "this is a
regulator" comes off a part's datasheet and the project seeds one part. So a module declared by class
would report every regulator missing. If you plan to declare modules by class, seed those parts in
`params/` first, or the intent tier reports absences that are really gaps in a different tier.

## Without the declaration

A checklist question about intent, such as "each rail sits at its declared voltage", cannot be
answered by a design that never stated its intent. It reads `needs-design-intent`, not `pass`, and
the reason names the `intent:` section it needs. A checklist that reported `pass` there instead could
not be trusted.

## All four tiers

That is the last of them. Names, interfaces, part limits and intent are each a file or a section, and
each changed what the catalog could see as well as adding checks of its own. The next rungs bind your
team's review questions to all of it.

## Next

[Write your checklist](../08-write-your-checklist/), which binds the team's review questions to the
engine so the mechanical ones answer themselves.

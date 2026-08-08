---
title: "netclass-via-drill"
description: "A net's via is drilled smaller than the drill its own net class declares."
---

A net's via is drilled smaller than the drill its own net class declares.

![a via drilled below the drill its net class declares is flagged; one meeting it is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/netclass-via-drill.svg)

## What this checks

The project's `net_settings` states, per net class, the via drill that class's nets should use. This
compares each net's smallest routed via drill against that declared drill and reports the nets that
fall short.

There is no number in this rule. The limit comes entirely from the design.

## How it differs from `hole-size`

`hole-size` asks **can this be drilled**: it compares via drills against a universal mechanical-drill
floor (0.2mm). A finding there means the fab cannot make the hole, or will silently upsize it.

This rule asks **is this what you asked for**. A class's declared drill is usually sized for the
current the class carries or for the process the board was quoted against. A via below it is often
perfectly drillable, and still discards the reason the number was chosen.

## Resolving the declared drill

A net can belong to several classes at once, and this is where a naive implementation goes wrong.

KiCad does not pick one of a net's classes and use it wholesale. It fills each constraint from the
highest-priority class that states **that** constraint, and the `Default` class supplies whatever is
left over. `Default` applies to every net, including nets in no class at all. So a net in a
high-priority class that declares only a track width still takes its via drill from the next class
down, and a net in no class still has a drill it is expected to meet.

Comparing a net's vias against every class it belongs to would fail nets that correctly obey the
class that won. The rule resolves the cascade first, then compares once, and the finding names the
class the limit came from.

## Hardware context (for software readers)

- **Via**: a plated hole connecting copper on different layers. The **drill** is the hole diameter
  before plating.
- **Why the declared drill matters**: a bigger drill carries more current and plates more reliably.
  A class that declares one has usually done that sizing deliberately.

## Absence is not a pass

The rule declares `CapNetClassDefs`. A design that declares no net-class definitions has no limit to
compare against, so the rule reports not-applicable rather than running over zero comparisons and
reading clean. Only a KiCad project read supplies definitions.

The rule is also silent on a net with no vias, since there is nothing to measure.

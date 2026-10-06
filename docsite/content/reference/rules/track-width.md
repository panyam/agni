---
title: "track-width"
description: "A routed track is narrower than the minimum the board declares, or the loosest common fabrication floor (0.127mm) when it declares none."
---

### Remedy

Widen the track to the fab's minimum, or move the board to a process quoted for the width you need. Below the floor a trace will not etch reliably at mainstream yields.

### What it means

A routed copper segment narrower than the minimum track width the board declares, or, when it
declares none, than 0.127mm (5mil), the minimum trace width of the loosest mainstream fabrication
capability (the corpus JLCPCB rule set).

![a trace narrower than the fab floor is flagged; an adequate width is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/track-width.svg)

### Why engineers want it

Track width is the first constraint every fab publishes and
every DRC ships. Below the floor the fab rejects the board or
etches it unreliably.

### Impact

At best the fab rejects the board at order time. At worst the trace etches into intermittent opens
and fails under current.

### Scope note

The 0.127mm default is deliberately the loosest published floor, so the rule fires on defects
rather than on deliberate tight routing under a capable fab's own rules. A board that declares its own minimum is held to that instead, because the declaration is the designer saying which process the board is routed for. Today that is a KiCad project's `board.design_settings.rules`, read whichever of the project's files names the design, and every finding names which of the two it was measured against (agni issue 933). An HDI board
routed at 0.1mm and declaring 0.0969mm is therefore checked against 0.0969mm, and
`netclass-track-width` separately checks the width each of the project's net classes declares. `check.Available` gates
the rule behind the board-geometry tier (a netlist-only design reports not-applicable, not
a silent pass).

### Query structure

select nets with any segment below the floor.

    select N in board.nets where count(S in N.segments where width(S) < floor) >= 1

where floor is the declared minimum track width, else 0.127mm.

Reads: board.copper, board.rules. Tier P.

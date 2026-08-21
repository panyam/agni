---
title: "crystal-load-caps"
description: "A passive crystal has an oscillator terminal with no load capacitor to ground."
---

### Remedy

Fit a load capacitor from each oscillator terminal to ground, sized from the crystal's specified load capacitance and the stray capacitance of the layout rather than copied from another design.

### What it means

A passive quartz crystal (a two-terminal resonator, ref-des `Y`/`XTAL`) needs a load
capacitor from each of its two oscillator terminals to ground. This rule flags a crystal
terminal that carries no capacitor.

### Why engineers want it

A crystal is specified for a nominal LOAD CAPACITANCE (`C_L`), and the two load caps plus
the board stray capacitance are what present that load. With a load cap missing the
oscillator sees the wrong load: it may fail to start, start only sometimes across
temperature, or run off its rated frequency. Everything clocked from it drifts with it, so
the defect surfaces as flaky UART/USB/CAN timing rather than an obvious "no clock".

### Impact

Oscillator no-start or off-frequency operation; timing-dependent field failures that pass
bench bring-up.

![Crystal with a load cap missing on one terminal is flagged; a cap on both terminals is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/crystal-load-caps.svg)

### Scope note

The rule quantifies over crystal COMPONENTS, not nets, because "which terminals" and "is
this an active oscillator" are both cross-net facts about the one part. A crystal that
connects to a non-ground POWER RAIL is treated as an ACTIVE oscillator (a packaged XO with
a Vdd pin, which supplies its own load internally and takes no external caps) and is
skipped entirely, so the rule never demands load caps of the wrong device. Ground-named
terminals (the grounded case pins of a 3- or 4-pin crystal) are not signal terminals and
are excluded. An unresolved external net is skipped (the cap may live on an unread sheet),
matching the decoupling-present / bulk-cap external-skip convention. The load-cap VALUE
(does `2*(C_L - C_stray)` match the crystal's spec) is a datasheet-joined refinement
(WS10), out of scope here: this rule checks PRESENCE, not value.

### For software readers

Think of the crystal as a component with a hard runtime dependency that the type system
can't express: it only works when two specific companion parts (the load caps) are wired
to it. This is a static "is the required dependency present on this node" check, the same
shape as decoupling-present ("does this power rail have its bypass cap"). The active-XO skip
is a guard clause: an active oscillator is a different type that satisfies the dependency
internally, and we detect it structurally (it has a power pin) rather than by trusting a
name.

### Query structure

select crystals; for each, gather non-ground terminal nets and whether it has a power pin;
skip if powered; require a capacitor on each terminal net.

    for Y in components where class(Y) == crystal:
      terms   = nets(Y) where not ground(net)
      powered = any net(Y) is a non-ground power rail
      if powered: continue
      for N in terms where not external(N):
        if not exists P in N.connections where class(P) == capacitor: FIRE(Y, N)

Reads: component.class, net.attributes (external), net.names (the ground / power-rail
skip), on_net. Tier R.

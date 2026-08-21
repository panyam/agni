---
title: "resonator-redundant-load-caps"
description: "A ceramic resonator with integrated load capacitors also has an external load cap to ground on a terminal."
---

### Remedy

Remove the external load capacitors. A resonator of the built-in-cap family carries its own, so the schematic should show the resonator alone.

### What it means

A ceramic resonator of the built-in-cap family (Murata CERALOCK and kin, ref-des `Y`) already
contains its load capacitors, so its oscillator terminals connect straight to the driver and
its center pin to ground. No external load caps are needed. This rule flags a resonator
terminal that carries an external load capacitor to ground: the "double load" mistake.

### Why engineers want it

The load a crystal or resonator sees sets whether it starts and at what frequency. A ceramic
resonator with integrated caps is specified for that load already built in. Add external load
caps to ground and the total load roughly doubles, well above spec, so the oscillator starts
slowly, starts only over part of the temperature range, or runs off-frequency. Everything
clocked from it drifts with it. It is a common carry-over when a crystal footprint (which does
need the caps) is reused for an integrated-cap resonator, and it usually oscillates on the
bench and fails in the field.

### Impact

Slow-start / no-start over temperature or off-frequency operation from an over-loaded
resonator; timing-dependent field failures that pass bench bring-up.

![A ceramic resonator with an external load cap to ground is flagged; a resonator wired with no external caps is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/resonator-redundant-load-caps.svg)

### Scope note

The rule quantifies over ceramic-resonator COMPONENTS. The `ceramic_resonator` class is
datasheet-seeded (WS10-015): the classifier cannot tell a resonator from a crystal by tokens,
so an un-subtyped clock candidate is NOT treated as a resonator and this rule stays silent on
it (a crystal genuinely needs its caps, which crystal-load-caps checks). A capacitor is counted
as an external LOAD cap only when it sits on a resonator terminal net AND also reaches ground;
a coupling or series cap between two signals (not touching ground) is not a load cap and is not
flagged. Ground-named nets are the resonator's center/case pin, not signal terminals. An
unresolved external net is skipped (its wiring may live on an unread sheet), matching the
crystal-load-caps external-skip convention. This rule checks the PRESENCE of a redundant cap,
not its value; the exact integrated-cap value is a datasheet-joined refinement (WS10), out of
scope here.

### For software readers

This is the mirror of crystal-load-caps. A crystal is a component with a required companion
dependency (load caps) that must be present; a built-in-cap resonator is a variant that
satisfies that dependency internally, so wiring the companion parts again is a redundant, and
harmful, override of a default. The check is "this node has a dependency it should NOT have,"
the negative of "this node is missing a dependency it must have." We detect the resonator
variant from its declared class (datasheet-seeded), not from a name, exactly as the active-XO
skip in crystal-load-caps detects the self-satisfying variant structurally.

### Query structure

select ceramic resonators; for each non-ground, resolved terminal net, fire if a capacitor on
that net also reaches ground.

    ground_refs = { component | some pin on a ground net }
    for Y in components where class(Y) == ceramic_resonator:
      for N in nets(Y) where not ground(N) and not external(N):
        if exists C in N.connections where class(C) == capacitor and C in ground_refs:
          FIRE(Y, N, C)

Reads: component.class, net.attributes (external), net.names (the ground skip and cap-to-ground
test), on_net. Tier R.

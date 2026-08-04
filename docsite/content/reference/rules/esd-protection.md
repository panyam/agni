---
title: "esd-protection"
description: "An externally-exposed signal net (on a connector) has no TVS device."
---

### What it means

A signal net a connector exposes to the outside world should carry a TVS
(ESD clamp) device.

### Why engineers want it

ESD protection on external interfaces is a standard review and
compliance item (IEC 61000-4-2 exists because fingers do). The clamp must be a deliberate
component on the net; internal chip protection is designed for handling, not for system-level
strikes.

### Impact

Field failures that never reproduce in the lab: flaky USB ports, dead data lines
after winter, latent damage that surfaces weeks later.

![External connector net without a TVS clamp is flagged; the same net with a TVS to ground is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/esd-protection.svg)

### Scope note

A signal is treated as protected two ways: a discrete TVS clamps it (within a 2-hop series reach),
OR an IC on the net carries a datasheet ESD rating at or above the credit floor (IC-integrated ESD,
WS3-073, the common automotive posture). The IC path needs a seeded PartSpec (`--params`), so on a
design read with no datasheets it is silent and the rule behaves as before. The IC path credits ONLY a
SYSTEM-level rating (IEC 61000-4-2), NOT a handling model (HBM/CDM): the rating's test model is a
declared `esd_test_model` attribute (WS3-077), and an unstated or handling rating never credits —
crediting an HBM handling rating on a harness input would hide a real ESD gap, since handling ratings
are for assembly, not field strikes. One refinement remains a follow-up: matching the rating to the
connector-facing PIN, not the whole part (deferred with WS3-077 — the PartSpec carries no structured
pin applicability, and the residue it would address is largely correct findings anyway).

![an IC with no ESD rating is still flagged; an IC carrying a datasheet ESD rating credits the net as fine]({{.Site.PathPrefix}}/static/images/catalog/rules/esd-protection-ic-rated.svg)

"External" is approximated as: the net has a connector-class member, no
power pins, no rail facts (global / power_driven), no ground name, and no power-rail NAME
(VCC/VDD/VBUS/12V-style); rails have their own rules (input-protection, bulk-cap), and the
name check is the only rail evidence a directionless netlist (EDIF) carries. A DEBUG / TEST /
edge-card / programming connector is excluded: it classifies as `test_connector` (WS3-066), a
distinct class from `connector`, so a bench interface (a JTAG header, a SAMTEC edge card) is not
treated as harness exposure; the debug-connector patterns are lexicon defaults a project can
extend. Severity is info because the approximation still cannot know
whether a plain header is a real external interface or an unlabeled internal one; the corpus results
are documented in the shipping PR. Since WS3-011 the clamp may sit one series
hop away (connector -> series R -> clamped node, the classic ESD topology): the TVS
existence check runs over the net's 2-hop reach. True ordering ("the clamp is
connector-side of the IC") remains unmodeled; reach makes the existence check
topology-tolerant, not directional.

### Query structure

select external signal nets, require a TVS member.

    select N in nets where exists C in N.connections where class(C) == connector
      and not exists P in N.connections where pin_dir(P) in {power_in, power_out}
      and not ground_name(N) and not rail_name(N) and not global(N) and not intentional_nc(N)
      and not exists G in N.connections where class(G) == tvs

Reads: component.class, net.attributes, net.names (the ground/rail-name skips), on_net,
pin.electrical_type, pin.no_connect (the no-connect skip). Tier R.

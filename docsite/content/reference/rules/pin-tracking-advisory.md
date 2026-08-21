---
title: "pin-tracking-advisory"
description: "Two pins of one part sit outside a tracking bound their datasheet recommends between them."
---

### Remedy

Restore the recommended ordering between the two terminals where the design allows it, or record that the loss of margin is accepted. Unlike the required bound, this costs performance rather than the part.

### What it means

A component joined to a seeded datasheet spec has two pins whose difference breaks a tracking bound
the datasheet **recommends** between them, stated with a verb like "should" or a phrase like "for
best operation".

### Why engineers want it

The vendor is telling you how the part works best, not where it breaks. A level shifter whose enable
"should be at least 1 V higher than the reference for best translator operation" will still function
below that, with degraded translation margin. That is worth a reviewer's attention and is not worth
failing a build over, so it stands as a separate rule from `pin-tracking-violated` rather
than the same rule at the same severity.

Keeping the two apart is what lets a team gate CI on datasheet **requirements** while still seeing
the recommendations in the report. Folding them together would either raise advice to an error or
demote a stress requirement to a warning.

### Everything else matches pin-tracking-violated

The bound is a signed value on `subject - reference`; the two evidence tiers are the same, with
connectivity settling the shared-net case exactly and the rail-gated name tier as the fallback; and
the same inputs are skipped rather than guessed. See that rule's page for the detail.

One difference: a relation with **no recorded modality** is handled by `pin-tracking-violated`, not
here, so it is reported once rather than by both rules.

### Query structure

    select C in components where spec(C) has pins and relations
      for R in tracking relations of spec(C) where recommended(R)
        S = terminal(C, subject_pin(R)); T = terminal(C, reference_pin(R))   -- may refuse
        if net(S) == net(T): diff = 0                                        -- connectivity
        else: require rail(net(S)) and rail(net(T)); diff = nominal(net(S)) - nominal(net(T))
        diff outside bound(R) -> finding

Reads: param.pin, param.pin_relation, net.role, net.nominal_voltage, net.name, on_net. Tier R.

---
title: "pin-tracking-advisory"
description: "Two pins of one part sit outside a tracking bound their datasheet recommends between them."
---

### Remedy

Restore the recommended ordering between the two terminals where the design allows it, or record that the loss of margin is accepted. Unlike the required bound, this costs performance rather than the part.

### What it means

A component joined to a seeded datasheet spec has two pins, and the datasheet **recommends** a
tracking bound between them. The datasheet states the bound with a verb like "should" or a phrase
like "for best operation", and the difference between the two pins breaks it.

![a datasheet recommending OE at least 1 V above VREF, missed by the rails, is flagged as advice; rails that meet it are fine]({{.Site.PathPrefix}}/static/images/catalog/rules/pin-tracking-advisory.svg)

### Why engineers want it

The vendor is telling you how the part works best, not where it breaks. Take a level shifter whose
enable "should be at least 1 V higher than the reference for best translator operation". It will
still function below that, with degraded translation margin. That is worth a reviewer's attention
and is not worth failing a build over, so it is a separate rule from `pin-tracking-violated` rather
than the same rule at the same severity.

Keeping the two apart lets a team gate CI on datasheet **requirements** while still seeing
the recommendations in the report. Folding them together would either raise advice to an error or
demote a stress requirement to a warning.

### Everything else matches pin-tracking-violated

The bound is a signed value on `subject - reference`; the two evidence tiers are the same, with
connectivity settling the shared-net case exactly and the rail-gated name tier as the fallback; and
the same inputs are skipped rather than guessed. See that rule's page for the detail.

One thing differs. `pin-tracking-violated` handles a relation with **no recorded modality**, and
this rule does not, so it is reported once rather than by both rules.

### Query structure

    select C in components where spec(C) has pins and relations
      for R in tracking relations of spec(C) where recommended(R)
        S = terminal(C, subject_pin(R)); T = terminal(C, reference_pin(R))   -- may refuse
        if net(S) == net(T): diff = 0                                        -- connectivity
        else: require net.rail(net(S)) and net.rail(net(T)); diff = nominal(net(S)) - nominal(net(T))
        diff outside bound(R) -> finding

Reads: param.pin, param.pin_relation, net.role, net.nominal_voltage, net.name, on_net. Tier R.

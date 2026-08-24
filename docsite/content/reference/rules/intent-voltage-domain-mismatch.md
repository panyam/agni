---
title: "intent/voltage-domain-mismatch"
description: "A declared voltage domain's rail is absent or named for a different nominal voltage."
---

### Remedy

Add the declared rail, or reconcile its name with the voltage the domain declares. A rail named for one voltage and declared at another will mislead every reader after you.

### What it means

The design intent declares voltage domains: named rails pinned to a nominal voltage ("these rails are
the 3.3V domain"). This rule fails when a declared rail is absent from the design, or present but its
name declares a different nominal than the domain does (a rail on the wrong domain).

### Why engineers want it

The power tree is where a wrong assumption is most expensive. A rail wired to the wrong domain, or a
domain rail that never got routed, can put an over-voltage on a part or leave a block unpowered. The
declaration states the intended domains so the check flags a rail that drifted off its declared
voltage.

### Impact

A power rail is missing or named for a voltage other than its declared domain, so the power tree does
not match the intended architecture: an unpowered block, or an over/under-voltage on a supply pin.

![A declared rail absent, and a rail whose name declares the wrong voltage, are flagged; a rail on its declared nominal is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/voltage-domain-mismatch.svg)

### Scope note

A present rail whose name carries no parseable voltage token (e.g. `VDD_CORE`) is left alone: the
name-derived nominal is the only voltage evidence a netlist carries, and refusing to guess is the
contract. So the rule verifies presence for every declared rail and voltage only for those whose name
encodes one. The domains come from the declaration, never derived from the netlist.

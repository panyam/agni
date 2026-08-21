---
title: "nc-pin-connected"
description: "A pin marked no-connect is wired into a net with other members."
---

### Remedy

Disconnect the pin and mark it no-connect. Where the connection is deliberate (a factory test pad, a vendor-documented exception), cite the datasheet section that allows it in a schematic note.

### What it means

A pin whose electrical type is NO_CONNECT (the symbol author's "leave this
alone") sits on a net that connects it to at least one other pin.

### Why engineers want it

It is the no-connect column of every ERC connection matrix (KiCad
reports it as "connected NC pin", an error). NC pins exist because vendors reserve pads: internal
test points, factory-trim pins, pads with no bond wire. A wire landing on one is usually a
misplaced connection, and occasionally a genuine misuse of the part.

### Impact

Best case, a signal routed to a dead pad silently does nothing (a missing connection
in disguise). Worst case, the net disturbs an internal node the datasheet says to leave floating.

![NC pin wired into a net is flagged; the same NC pin left unwired is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/nc-pin-connected.svg)

### Scope note

Fires only when the net has two or more members: a lone NC pin on its own
stub net is the intentional case (single-pin-net skips it for the same reason). Cross-sheet
(external) nets are skipped. The pairing is NC ↔ anything, so one flagged net reports once,
on the net.

### Query structure

select multi-member nets carrying a NO_CONNECT-typed pin.

    select N in nets where count(N.connections) >= 2
      and exists P in N.connections where pin_dir(P) == no_connect

Reads: net.attributes, net.pin_count, on_net, pin.electrical_type. Tier R.

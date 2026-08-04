---
title: "unconnected-pin"
description: "A pin lands on no net and is not marked no-connect."
---

### What it means

A pin the part type declares, on a placed component, that appears in no
net's connections and is not marked NO_CONNECT.

### Why engineers want it

unconnected-component catches the part wired to nothing; this
catches the pin wired to nothing on a part that is otherwise connected — the far more common
slip. Every ERC ships it.

### Impact

A floating enable, a missed feedback divider, one gate input of four left open:
parts that mostly work and fail in ways that read as component defects.

![A declared pin left on no net is flagged; the same pin marked NO_CONNECT is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/unconnected-pin.svg)

### Where it can and cannot fire (conservative on purpose)

Three guards, each learned
from a real source:
- **The source must have a no-connect channel** (any NO_CONNECT-typed pin or nc-marker net
  name anywhere in the design). Where none exists, an unwired pin carries no NC flag
  because the format cannot express one, not because the designer missed it: a bare EDIF
  netlist export lists every library pin of every part, and firing there produced 1836
  findings on one real board (unused gates, spare TVS channels, unwired connector pads —
  all normal). No channel, no rule.
- **A direction-unknown (UNSPECIFIED) pin is skipped**, the same trade floating-input makes.
- **A source with no part-pin data yields no pins** and is silent by construction.

On KiCad, placed bare pins land on synthesized per-pin stub nets (the miss surfaces as
single-pin-net instead); what can still fire there is a typed pin of an unplaced unit of a
multi-unit part, in designs that use no-connect markers elsewhere — a real unused-unit
signal. The hunting ground is sources with typed pins, an NC vocabulary, and no stub
synthesis (EDIF schematic exports, xschem/gEDA with symbol libraries).

### Query structure

gate on the channel, then select the pins on no net, excluding
declared no-connects and direction-unknown pins.

    select P in pins where nc_channel(design)
      and pin_dir(P) not in {no_connect, unspecified} and not on_net(P)

Reads: pin.electrical_type, pin.on_net. Tier R.

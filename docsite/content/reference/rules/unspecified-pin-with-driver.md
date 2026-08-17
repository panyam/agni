---
title: "unspecified-pin-with-driver"
description: "A pin with no declared electrical type sits on a driven net."
---

### What it means

A net has at least one hard driver (output / power-out pin, including a
power symbol's virtual pin), and one of its other member pins declares no electrical type at
all.

### Why engineers want it

It is the "unspecified" column of the ERC connection matrix (KiCad
warns on unspecified against everything). An untyped pin is a library gap: the matrix cannot
say whether the pairing is legal, and the place that matters most is a driven net, where the
untyped pin might itself be a fighting driver or a supply pin shorted to a signal.

### Impact

If the untyped pin turns out to be an output, this is output-output-conflict the
matrix could not see; if a supply pin, a rail-to-signal short. Fixing the symbol (typing the
pin) either clears the warning or upgrades it to the real error.

![A driver joined to an untyped J1 pin is flagged; the same driver joined to a declared passive R1 pin is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/unspecified-pin-with-driver.svg)

### Scope note

"Unspecified" means the symbol DECLARES the pin and types it as nothing. A
KiCad passive pin is typed and never fires this (the resistor-on-every-driven-net false
positive that kept this row out of the catalog until PASSIVE entered the direction
vocabulary), and a pin the read never saw a symbol for (a board footprint's pads, a
sub-sheet component in a root-only hierarchy read) is a read gap, not an authoring gap, and
is skipped via pin.declared. Gated on a driver being present: a wholly untyped read (a bare
EDIF netlist with no direction data) has no drivers in evidence and stays silent by
construction. Cross-sheet (external) nets are skipped. Virtual power-symbol pins carry
power types by construction and are excluded as subjects.

### Query structure

select the driven nets carrying a declared-but-untyped real pin.

    select N in nets where count(distinct component(P) : P in N, hard_driver(P)) >= 1
      and exists P in N.connections
        where pin_dir(P) == unspecified and declared(P) and not virtual(P)

Reads: net.attributes, on_net, pin.electrical_type. Tier R.

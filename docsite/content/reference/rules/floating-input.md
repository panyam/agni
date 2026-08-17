---
title: "floating-input"
description: "An input pin sits on a net with no driver and no pull, so its level is undefined."
---

### What it means

A net that carries one or more input pins and nothing else that could set its
level: no driver, no pull resistor, no rail. The input is left floating.

### Why engineers want it

Unused gate inputs, unconnected enable/mode pins, and forgotten pulls are
a standard bring-up surprise. A floating CMOS input is not a benign "0"; it drifts to the switching
threshold, draws shoot-through current, and couples noise, so the part misbehaves in ways that move
with temperature and a fingertip.

### Impact

Non-deterministic logic and excess current. It is a real defect, but the check must be
conservative to stay quiet on legitimate nets.

![Input-only net is flagged as floating; the same input pulled to a rail is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/floating-input.svg)

### Scope note (conservative on purpose)

Fires only when every pin on the net is an input (or an
explicit no-connect): the moment a driver, a passive (a pull), or a power pin is present the net is
considered set and the rule stays quiet. A net with any un-typed (UNSPECIFIED) pin is skipped too,
since the reader could not classify it. This trades recall for near-zero false positives.

### A passive member exempts the net

A resistor on the net is the classic pull (the net
is set); a cap or ferrite means a path this netlist-local rule cannot follow. Claiming
"floating" past a passive is guesswork, so ANY passive-class member (resistor, capacitor,
inductor, ferrite, fuse, test point) silences the rule for that net, which also absorbs
libraries that type passive pins INPUT (the Mentor EDIF corpus does for capacitors, which
made cap-plus-input nets read as all-input the moment pin keys joined correctly, WS1-025).

### A diode terminal is not a logic input

Some libraries type a diode/LED/TVS terminal (an anode or cathode) INPUT, the same way they
type passive pins. But a diode-only node's level is set by the diode network (a clamp, a
steering pair, a diode-OR), it is not an undriven logic input. So a diode-family pin is
excluded from the "is there a logic input" count. The exclusion is PER-PIN, not per-net:
a real IC input that merely carries a clamp diode still fires (the input is genuinely
floating), only a net whose input pins are all diode terminals goes quiet. (Without this, a
pair of steering-diode cathodes tied together read as an all-input net and false-fired; on
one real industrial netlist that was 36 findings, every one a diode network.)

### Query structure

select all-input nets that carry no passive member and at least one non-diode logic input.

    select N in nets where not exists P in N.connections where class(P) in passives
      and any(input(P) and class(P) not in {diode, led, tvs})
      and all(input(P) or no_connect(P))

Reads: component.class (the passive exemption and the diode-terminal exclusion),
net.attributes (the external skip), net.pin_count, on_net, pin.electrical_type. Tier R.

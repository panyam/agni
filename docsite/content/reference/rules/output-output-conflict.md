---
title: "output-output-conflict"
description: "Two or more driving pins (outputs / power sources) share a net and fight each other."
---

### What it means

A net with two or more pins that actively drive it — signal outputs or power
sources. A net may have at most one hard driver; two of them fight.

### Why engineers want it

It is one of the ERC pin-type-matrix's flagship errors (output ↔ output).
It happens when two blocks are wired to the same node by mistake, a bus is drawn without tri-state
enables, or two regulators land on one rail. The symptom is contention current: the net sits between
levels while both drivers source into each other.

### Impact

Contention burns power and can destroy the output stages; the logic level is undefined.

![Two component outputs on one net are flagged; one output driving one input is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/output-output-conflict.svg)

### Scope note

Counts hard drivers only — outputs and power-outputs. Bidirectional (INOUT) pins are
not counted: a shared bus of bidirectional pins is legal, and flagging it is the false positive that
makes engineers mute the rule. Pin roles come from the reader's electrical types; a design whose
reader does not type pins does not fire. This is a row of the connection matrix (rule_pin_matrix.go);
the pairing subsumes output ↔ output, output ↔ power-out, and power-out ↔ power-out.

### Drivers are counted per COMPONENT, not per pin

A power device commonly drives one net
through several paralleled output pins (the real corpus: one driver IC, six output pads on
one net) — one driver electrically. Two-plus driving pins of the SAME component never fire;
two different components' outputs do. This surfaced the moment EDIF pin keys joined
physically (WS1-025) and is why the count lives behind the driving_components FFI: distinct-
count is not (yet) an AST shape.

### A wired-OR bus is not contention

An open-drain bus (a shared interrupt, inhibit, or reset line) has several devices whose outputs
only pull to one level, with a pull resistor setting the idle level. That is intentional, not two
drivers fighting. EDIF types open-drain pins plain "output", so the tell is the PULL RESISTOR: a
multi-driver net that carries a resistor and NO power-source (POWER_OUT) driver is treated as a
wired-OR bus and stays quiet. The resistor is matched by PRESENCE, not by where it pulls to, because
a real pull runs through several elements to an auto-named rail or to ground. The power-source
exclusion is load-bearing: two power sources on a rail fight even with a bleeder resistor present, so
a POWER_OUT driver keeps the rule firing.

### Query structure

select the nets driven by two or more distinct components, excluding wired-OR buses.

    select N in nets where count(distinct component(P) : P in N, hard_driver(P)) >= 2
      and not (has_resistor(N) and not any(power_out(P) : P in N))

Reads: on_net (the net's members), pin.electrical_type, component.class (the resistor pull). Tier R.

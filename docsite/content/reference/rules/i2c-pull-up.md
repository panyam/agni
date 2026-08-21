---
title: "i2c-pull-up"
description: "An I2C net (SDA/SCL) reaches no rail through a pull-up resistor."
---

### Remedy

Fit a pull-up resistor from each of SDA and SCL to the bus rail, sized from the bus capacitance and the clock rate the design actually runs at rather than from a habitual value.

### What it means

Every I2C net (its two lines are SDA and SCL) must have a pull-up resistor
to its rail. Open-drain pins can only pull low; the pull-up returns the line high.

### Why engineers want it

Open-drain is a shared-bus trick: many parts can pull the line low,
and a single pull-up returns it high. The pull-up is external and easy to forget, especially
when every chip on the bus has a weak internal pull-up that a designer wrongly assumes is
enough. TI and others publish app notes on it precisely because it is a recurring field failure.

### Impact

A missing pull-up means the bus is stuck low and no device on it communicates.

![An I2C line with no pull-up resistor is flagged; a line with a pull-up to the rail is fine]({{.Site.PathPrefix}}/static/images/catalog/rules/i2c-pull-up.svg)

### Scope note

This is the presence check, which needs no datasheet. The
pull-up value being in range is a datasheet-joined rule (a separate, Tier-X check). Resistor
identity is the shared component.class fact: the ref-des prefix convention (R, RN) refined by
part-type data when the source carries it.

**The check follows the resistor's OTHER end.** It is not enough for a resistor to touch the bus,
because a series termination or bus-isolation resistor does that and holds nothing high. The rule
walks from the bus across resistors, never through ground, and passes only when it arrives at a
rail. Until issue 375 it asked the membership question instead, so a bus with a series resistor and
no pull-up at all passed at error severity.

The walk is bounded at `check.PullUpReachHops` crossings. One is a pull-up sitting directly on the
bus. Two is a bus segment separated from its pull-up by a series resistor, which is an ordinary
topology and the reason a one-hop test would report a false positive on it. Past three the
accumulated series resistance is comparable to the pull-up, so the node no longer returns high in
the time the bus needs and there is nothing worth crediting.

Resistors only, and ground is never crossed: a resistor to ground is a pull-DOWN, and counting it
would pass exactly the bus this rule exists to catch.

The SDA/SCL name match is at a **token boundary**, not a substring (WS3-037): `SDA`, `SCL`,
`I2C_SCL`, and `SCL0` match; `SPI_SCLK` (an SPI clock), `SCLK`, and `MCLK` do NOT, because `SCL`
abutting another letter is a different signal. Generalizing this presence check to an
arbitrary profile-declared signal set (CS, reset, boot straps) is the remaining WS3-037 work.

### Query structure

select the I2C nets that have no resistor member.

    select N in nets where is_i2c(N) and not exists P in N.connections where is_resistor(P)

Reads: net name (pattern), on_net, component.class (resistor). Tier R.

---
title: "i2c-pull-up"
description: "An I2C net (SDA/SCL) has no pull-up resistor."
---

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

**What the check actually tests today is narrower than the sentence at the top of this page.** It
asks whether a resistor is connected to the net, not whether that resistor reaches a rail, so a
series termination or bus-isolation resistor satisfies it and a genuinely missing pull-up passes.
That is a false pass on an error-severity rule and it is tracked as issue 375. The rail terminus
needs the resistor's OTHER net, and the general form of that question is a path query, which is the
subject of the topology-pattern design in issue 374.

The SDA/SCL name match is at a **token boundary**, not a substring (WS3-037): `SDA`, `SCL`,
`I2C_SCL`, and `SCL0` match; `SPI_SCLK` (an SPI clock), `SCLK`, and `MCLK` do NOT, because `SCL`
abutting another letter is a different signal. Generalizing this presence check to an
arbitrary profile-declared signal set (CS, reset, boot straps) is the remaining WS3-037 work.

### Query structure

select the I2C nets that have no resistor member.

    select N in nets where is_i2c(N) and not exists P in N.connections where is_resistor(P)

Reads: net name (pattern), on_net, component.class (resistor). Tier R.

## i2c-pull-up-split-rail

### What it means

An I2C net reaches TWO different rails through its pull-up resistors. One part pulls the bus to one
supply and another pulls it to a second.

### Why engineers want it

Two pull-ups to two rails tie those rails together through the bus. Whenever one supply is up and the
other is not, current flows from the live rail through its pull-up, along the bus, and back through
the other pull-up into a rail that is meant to be off. That rail gets partially fed by the bus, which
is a sequencing fault: it can hold a supply above its off threshold, back-feed a regulator, or bring a
device up in a state its reset never sees.

It also leaves the bus idling at whichever rail wins, which may exceed the input rating of a device
sitting on the lower supply.

The failure mode is the awkward one. On the bench every supply comes up at once and nothing is
observed; the fault appears as a board that fails on one particular power-up order, or after a
brown-out, which is the hardest kind of report to act on.

### Impact

A supply is fed through a signal bus during sequencing, and the bus idles at a level one side may not
tolerate. Neither shows up in a functional test that powers everything simultaneously.

### Fires versus fine

```
  fires                                 fine

  +3V3      +1V8                        +1V8
   |         |                           |
  R1        R2                          R1
   |         |                           |
   +----+----+                           |
        |                                |
       SDA                              SDA
                                        (a bus that crosses domains
                                         gets a level translator, and
                                         one pull-up per side of it)
```

### Scope

- Only nets matching the I2C naming convention at a token boundary (SDA / SCL).
- A net whose pull-ups all land on ONE rail is `i2c-redundant-pull-up`, not this. The two conditions
  are disjoint, so a bus is reported once.
- Rail identity is the rail NET, so two rails that happen to carry the same voltage are still two
  rails. That is deliberate: the sequencing question is about which supply, not which voltage.
- What counts as a rail comes from the naming lexicon, so a project whose supplies the built-in
  vocabulary does not recognise reports nothing here. Declare them in `conventions.yaml`; the
  `rail-not-classified` rule is the tripwire that says when this is happening.
- Severity is `warning`, because a bus deliberately pulled to one domain with a translator elsewhere
  on the net can look like this to a netlist.

### The query structure

Enumerate I2C nets; walk out through resistors up to three crossings, collecting every distinct
resistor whose far side is a rail, with the rail each landed on; fire when the set of rails has more
than one member.

### For software readers

Two connection pools onto one socket, each configured against a different endpoint. Nothing is wrong
with either pool's own configuration, and the bug only appears when one endpoint is down: traffic
takes the path through the other one. Reachability tells you the socket is connected; only counting
the pools and comparing their endpoints tells you it is connected to two things.

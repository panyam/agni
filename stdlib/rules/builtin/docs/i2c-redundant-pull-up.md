## i2c-redundant-pull-up

### What it means

An I2C net reaches one rail through more than one resistor. The bus is pulled up twice.

The check counts DISTINCT resistors that take the net to a rail, within three series crossings. Two
routes through one resistor are one pull-up; two resistors are two, however they are drawn.

### Why engineers want it

Resistors in parallel are one smaller resistor. Two 2.2k pull-ups are an effective 1.1k, so the bus
sinks roughly twice the current it was sized for. An open-drain device can only pull the line low by
sinking that current, and every device has a maximum it will sink while still holding its output
below VOL, so a bus pulled too hard reads as one that works with some parts and not others, or one
whose low level sits high enough that a receiver reads it as neither state.

It arrives the same way almost every time. A module carries its own termination so it works
standalone, the board it plugs into already pulls the same bus, and nothing on either schematic is
wrong on its own.

### Impact

The bus is over-pulled by whatever factor the parallel combination gives. It usually still works, on
the bench, with the parts that happen to be fitted, which is what makes it a warning worth reading
rather than a fault worth stopping for.

### Fires versus fine

```
  fires                                 fine

  +3V3      +3V3                        +3V3
   |         |                           |
  R1 2k2    R2 2k2                      R1 2k2
   |         |                           |
   +----+----+                           |
        |                                |
       SDA                              SDA
```

### Scope

- Only nets matching the I2C naming convention at a token boundary (SDA / SCL), the same predicate
  `i2c-pull-up` uses. A bus named some other way is out of scope rather than passing.
- A net with NO pull-up is not this rule's subject. That is `i2c-pull-up`, which is an error, and
  reporting the same absence in three places would treble one defect.
- Pull-ups to DIFFERENT rails are `i2c-pull-up-split-rail`. The two rules fire on disjoint
  conditions, so a bus is named once, by the rule whose remedy applies to it.
- Ground is never crossed and a resistor to ground is not counted: that is a pull-down, and counting
  it would report the very bus these rules exist to catch as over-pulled.
- Severity is `warning`, not `error`, because a jumper-selectable or DNP termination is legal by
  design and the netlist cannot see the fit status.

### The query structure

Enumerate I2C nets; for each, walk out through resistors up to three crossings, collecting every
distinct resistor whose far side is a rail; fire when there is more than one and they all land on the
same rail.

A reachability question cannot express this. `reaches` reports a rail as reached however many
resistors reach it, so a doubled bus and a correct one are the same answer.

### For software readers

A resource with two owners, each of which believes it is the only one, discovered by counting the
handles rather than by asking whether the resource is held. A `defer close()` in both the caller and
the callee: harmless in the sense that the thing does get closed, wrong in the sense that the
accounting is off, and invisible to any check that asks only whether it was closed.

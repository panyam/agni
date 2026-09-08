## io-map-far-end

### What it checks

A map row may declare where the net goes, not only where it starts:

```yaml
  - {net: PMIC_PG, device: U101, pin: PTC11, to: {device: U7000, pin: PG}}
```

This rule follows the net from the declared pin to the declared far end, through the series parts
between them, and reports the rows where it does not arrive.

### For hardware engineers

A net can be on the right pin at one end and land on the wrong part at the other, and no per-net
check can see it. `intent/io-map-pin-mismatch` confirms the MCU end; this confirms the other end is
the device the architecture says it drives.

The walk crosses series pass elements (resistors, inductors, ferrite beads, fuses), because a
resistor in the middle of a signal splits the net without breaking the connection. It never crosses a
capacitor, which is a DC block, and it never continues through a rail or a plane, which touches
almost everything and would join everything to everything.

A pass reports the ROUTE it walked, not just the verdict:

```
PMIC_PG -> [R3] -> MCU_NRST
```

so a reviewer can see the series resistor and decide whether that path is the intended one, without
opening the schematic. A failure says where each end actually sits.

### Coverage is most of the value here

The far-end columns of a real map are filled sparsely. In the one we measured, the downstream part,
its pin number and its pin name were filled on roughly 45%, 35% and 6% of rows.

**Every row gets a verdict, including the rows declaring no far end**, which report as
`not-considered` with that reason. A rule that reported only the rows carrying a far end would show a
clean result whose denominator was the empty ones, and a green answer would mean no more than that
the columns were blank.

### The three outcomes, kept apart

A path question has three answers and collapsing any two of them is a defect in the tool:

- **A route was found.** Reported with the route.
- **No route within the radius.** A real answer about the design: these two pins do not join.
- **The question could not be asked.** An endpoint resolved to a pin that sits on no net, so nothing
  was walked. This reports as `not-considered` and never as a no-route, because a pin spelled wrong
  in a declaration and two pins genuinely unconnected send a reader to opposite places.

### Declaring it

`to` takes a device and a pin, and both are required. A `to` naming a device and no pin is rejected
when the declaration loads, rather than reaching the rule as half a question it would have to invent
a reading of.

The far-end pin is resolved the same way the near pin is, so it may be a package designator or the
part type's functional name.

### Fixing a finding

Read the route before editing anything. A net that arrives at the wrong device usually diverges at a
recognisable point, and the reported path shows where. Then either route it to the declared pin or
amend the far end the map declares, depending on which document is current.

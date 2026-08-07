## esd

### What it checks

For each of the interface's signal nets that leaves the board through a connector, whether anything
clamps it. A net is reported when it is connector-facing and no TVS, no Zener, and no ESD-rated IC
sits within two series crossings of it.

The check is silent on an interface the design does not carry, and silent on an interface whose lines
are all on-board. Both are the honest answer, not a pass.

### For hardware engineers

Any line that leaves the board is an ESD path into whatever drives it. Touching a connector shell can
discharge kilovolts, and an unclamped bus pin takes that energy directly. The damage is often not a
clean failure: the part degrades, and the symptom is a port that works for months and then does not,
which is expensive to find in the field.

A TVS is the intended answer. It sits beside the line rather than in it, idle at normal voltage, and
conducts hard when the line spikes so the surge goes to ground instead of into the transceiver.

Distance matters and is why the check has a radius. Every series element between the connector and
the clamp is impedance the surge pushes through before the clamp conducts, so the pin's voltage
spikes first. A TVS several parts downstream is protecting what is behind it, not the pin. The check
looks two series crossings out, the same radius the design-wide `esd-protection` rule uses.

### Scope: the design decides, not the protocol

The requirement applies to whichever of the interface's nets are actually connector-facing, not to a
fixed list of lines. That distinction is load-bearing. On CAN, `_CANH` and `_CANL` leave the board
while `_TXD` and `_RXD` run to the MCU and never do, so a check applied to every declared signal would
report two lines that were never exposed.

Because the scope is read from the board, declaring this requirement on a profile is safe even when
that bus is usually on-board: with no connector on the net, the requirement selects nothing and says
nothing.

### Why a Zener counts here

A Zener clamp is not adequate ESD protection. It is slower than a TVS and not characterized for surge
energy, and using one where a TVS belongs is a real finding.

This requirement still treats a Zener-clamped net as satisfied, and that is deliberate rather than an
oversight. The catalog rule `esd-clamp-not-tvs` reports exactly that case, so the two checks partition
connector-facing nets between them: this one covers "nothing is clamping it", the other covers "the
wrong thing is clamping it". Reporting the Zener case in both places would double-count one defect and
make a fix look half-done.

### Relation to the design-wide rule

`esd-protection` asks the same question across the whole board. This requirement asks it per
interface, scoped by the profile's signal naming and gated by the profile's presence machinery, so a
review item binds `profile: CAN` and reports against that interface alone.

They agree by construction rather than by two hand-written guard stacks kept in sync: both read the
same `external_signal_net` scope, and this requirement's three protection clauses are the same three
exemptions at the same radius (`check.ProtectionReachHops`, interpolated into the generated datalog
rather than written as a number).

### Declaring it

```yaml
requirements:
  - {type: esd}
```

No params. Shipped on the profiles whose buses leave the board (CAN, LIN, A2B, SGMII, PCIe) and
deliberately absent from the on-board ones (eMMC, SPI-NOR), where no line is ever connector-facing.

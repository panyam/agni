## io-map-pin-mismatch

### What it checks

Every row of the declared IO map names a net, a device and a pin. This rule looks the pin up in the
netlist and reports the rows where the pin carries something other than the net the map assigns to
it.

The map is the declaration; the netlist is the design. Neither is derived from the other, which is
what makes the comparison worth anything.

### For hardware engineers

On any board carrying a large MCU or SoC, the pin assignment is decided long before the schematic
exists. Someone works out which peripheral lands on which pin, firmware is written against that
decision, and the schematic is drawn from it.

Then a pin assignment moves. Layout wanted a swap to clean up a crossing, or firmware hit a mux
conflict and needed the peripheral somewhere else. The map is updated and one net does not get
redrawn.

**Nothing about the resulting board is electrically wrong**, which is why no other rule finds it.
Every net is properly driven, every rail is in range, every bus is terminated. The board simply
disagrees with a document that, until now, nothing had ever read. It surfaces at bring-up as a
peripheral that does not respond, and the time is spent looking at the peripheral rather than at the
pin it is on.

### The pin may be spelled either way

A map is authored in the vocabulary a datasheet and a firmware header use, the functional pin NAME
(`PTC11`, `SDA`). A netlist answers in package DESIGNATORS (`41`, `12`). An author should not have to
know which one the checker wants, so both are tried.

Both go through the shared identifier comparison, so these agree:

| in the map | in the design | why |
|---|---|---|
| `PTE7` | `PTE07` | a zero-padded index, compared numerically |
| `TXD[1]` | `TXD1` | a bracketed index |
| `adc0_se12` | `ADC0_SE12` | letter case |
| `ADC0_SE12` + an invisible character | `ADC0_SE12` | pasted from a PDF or a spreadsheet |

A match that needed any of that says so in the verdict. A row that agreed outright says nothing
extra, so a note always means something really was inferred.

### The answers that are not pass or fail

**A name matching several pins is `inconclusive`, never a fail.** A part type may declare one name on
several pins, and a device with four `GND` pins is ordinary. The rule cannot decide which pin the map
meant, and the row may well be correct, so it says exactly that and names the candidates. Naming the
package designator instead makes the row decidable.

**A pin the device does not declare is a fail.** The map names something the part does not have, by
either spelling, which is a defect in one document or the other.

**A device the design does not carry is `not-considered`.** A missing part is what the module and
subsystem forms report; firing here as well would put one defect under two review items.

**A device whose part type carried no pin list is `not-considered`**, naming that. It is a gap in the
READ rather than anything about the design, and it must not read as a wrong pin.

**A declared net the design does not have is `not-considered` here**, because
`intent/io-map-net-absent` reports it. Reading a silence here as "the pin is fine" would be wrong;
the row is answered next door.

### A net name is compared more strictly than a pin name

A pin name is compared loosely on purpose, because vendor tables are inconsistent with themselves. A
NET name is not: it is the design's own identifier, so `DDR_CK_P` and `DDR_CK_T_P` are two nets
rather than two spellings of one. Net comparison accepts an exact or a normalized match and refuses a
fuzzy one.

### Declaring it

```yaml
io_map:
  - net: ADC_BATT_SENSE
    device: U101
    pin: PTC11
    function: ADC0_S17             # carried, NOT yet evaluated
    to: {device: U7000, pin: PG}   # optional far end
```

`net`, `device` and `pin` are required. A row missing any of them states nothing checkable.

**`function` is carried and nothing reads it.** Deciding whether a selected function is legal on a
pin means reading the part's alternate-function table, a pad-by-mode grid that the datasheet contract
has no shape for. That is agni issue 667, and it is a different artifact from the pin FUNCTION table
(one row per pin, name and number and I/O type), which is issue 188 and is done.

Every verdict on a row that declares one says outright that it was not evaluated, so filling the
column in can never read as having it verified.

### Fixing a finding

Either the net moved and the map did not, or the map moved and the schematic did not. Both happen and
they look identical from here. The firmware was written against the map, so establish which document
is current before editing either.

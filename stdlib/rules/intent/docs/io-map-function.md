## io-map-function

### What it checks

A map row may say which peripheral function the pin is switched to:

```yaml
  - {net: BLE_UART_RX, device: U1, pin: PTB3, function: LPUART1_RX}
```

This rule looks the pin up in the part's datasheet entry and reports the rows whose function is not
one the datasheet lists for that pin.

### For hardware engineers

A microcontroller pin can carry several peripherals, and a mux setting picks one. The datasheet's
pin-mux table lists which ones each pin offers. On the NXP KW47, pin PTB3 can be `ADC0_B13`, `PTB3`,
`LPSPI1_SOUT`, `LPUART1_RX`, `TPM1_CH3` or `FLEXIO0_D29`, and nothing else. A map row putting
`LPI2C1_SDA` on PTB3 describes a configuration the part cannot be set to, since that function lives on
PTB4.

The netlist cannot catch this, because the copper is fine. It surfaces when the firmware's pin-mux
code is written, or at bring-up as a peripheral that never sees the pin.

A pass names the function it matched and cites the datasheet page the table came from. A failure
lists every function the datasheet does give the pin, which is usually enough to spot the intended
one (the same peripheral under another instance number, or the neighbouring pin).

### How names are compared

Through `core/ident`, the same comparison the pin lookup uses. A vendor's table spells its own
functions inconsistently, and one slot can name two signals (`ADC0_A10/CMP0_IN0`), so each side is
split and canonicalized before comparing. A match that needed a token dropped passes with a note
saying so, rather than passing silently.

### When it cannot decide

Each of these reports the row `not-considered`, with the reason, and never as a pass:

- the row declares no function;
- the part has no part number, or the datasheet corpus has no entry for it;
- the entry has pins but the design pin does not resolve to exactly one of them;
- the entry lists no alternate functions for that pin, because no pin-mux table was read from its
  document.

The last is common. The alternate-function table comes from a recipe that reads the vendor's pin-mux
table (see the datasheet layer's architecture page), and a part whose document has no recipe yet has
pins without functions.

### Fixing a finding

Read the listed functions first. Then either move the function to a pin the datasheet lists it on,
which is a schematic and firmware change, or pick a function this pin does offer and update the map.

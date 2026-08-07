package profiles

import _ "embed"

//go:embed builtins/can.yaml
var canYAML []byte

// CAN is the CAN bus interface: the differential pair CANH/CANL on the bus side, TXD/RXD on the
// controller side, and a termination requirement (a 120Ω resistor across CANH/CANL at each bus end).
// CANH is the anchor. Its requirements include `termination` — a requirement TYPE that did not exist
// for SPI-NOR/eMMC — so adding it cost a registered compiler (termination.go) plus this declaration,
// no change to Compile: the WS3-045 point that widening interface coverage is DATA, not engine code.
//
// v0 checks presence + dangling + host-completeness + termination. It does NOT model bit-timing, the
// optional split-termination stabilizing cap, or bus-length rules; those are datasheet/geometry
// concerns, not netlist presence.
var CAN = mustParse(canYAML)

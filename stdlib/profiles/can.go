package profiles

import _ "embed"

//go:embed builtins/can.yaml
var canYAML []byte

// CAN is the CAN bus interface, anchored on CANH. The bus side is the differential pair CANH/CANL,
// the controller side is TXD/RXD, and each bus end wants a 120Ω resistor across CANH/CANL. The
// `termination` requirement is compiled by termination.go with no change to Compile (WS3-045).
//
// v0 checks presence, dangling, host-completeness, termination and ESD. It does NOT model bit-timing,
// the optional split-termination stabilizing cap, or bus length, which need a datasheet or geometry
// rather than a netlist.
var CAN = mustParse(canYAML)

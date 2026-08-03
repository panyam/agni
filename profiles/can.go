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
//
// TODO(WS3-045): the equivalent YAML declaration lives in builtins/can.yaml and is held identical to
// this literal by TestBuiltinsMatchYAML. Both forms are kept for now so the Go and YAML shapes can be
// compared side by side; once the YAML loader path is proven, make the YAML authoritative and delete
// this literal.
var CAN = Profile{
	Name:        "CAN",
	HostAttrKey: "interface",
	HostAttrVal: "CAN",
	Signals: []Signal{
		{Name: "CANH", Suffix: "_CANH", Anchor: true},
		{Name: "CANL", Suffix: "_CANL"},
		{Name: "TXD", Suffix: "_TXD"},
		{Name: "RXD", Suffix: "_RXD"},
	},
	Requirements: []Requirement{
		{Type: "signal-missing"},
		{Type: "host-incomplete"},
		{Type: "termination", Params: map[string]string{"high": "_CANH", "low": "_CANL"}},
		{Type: "signal-dangling"},
	},
}

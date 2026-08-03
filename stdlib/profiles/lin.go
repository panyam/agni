package profiles

import _ "embed"

//go:embed builtins/lin.yaml
var linYAML []byte

// LIN is the LIN (Local Interconnect Network) bus interface: a single-wire bus line with a pull-up to
// VBAT (1kΩ master / 30kΩ slave), plus TXD/RXD on the controller side. LIN is the anchor and the
// pulled-up line. It reuses the existing requirement types (signal-missing, host-incomplete,
// missing-pullup, signal-dangling) — no new compiler — so like eMMC it is a pure data-value profile.
//
// v0 checks presence + the bus pull-up + dangling + host-completeness. It does NOT model the
// master/slave role, the INH/wake pins, or dominant-timeout (datasheet/strap concerns).
//
// TODO(WS3-045): held identical to builtins/lin.yaml by TestBuiltinsMatchYAML; flip to
// YAML-authoritative with the other built-ins.
var LIN = Profile{
	Name:        "LIN",
	HostAttrKey: "interface",
	HostAttrVal: "LIN",
	Signals: []Signal{
		{Name: "LIN", Suffix: "_LIN", PullUp: true, Anchor: true},
		{Name: "TXD", Suffix: "_TXD"},
		{Name: "RXD", Suffix: "_RXD"},
	},
	Requirements: []Requirement{
		{Type: "signal-missing"},
		{Type: "host-incomplete"},
		{Type: "missing-pullup"},
		{Type: "signal-dangling"},
	},
}

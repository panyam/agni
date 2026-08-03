package profiles

import _ "embed"

//go:embed builtins/sgmii.yaml
var sgmiiYAML []byte

// SGMII is the Serial Gigabit Media-Independent Interface between a MAC and a PHY: differential TX
// (TXP/TXN) and RX (RXP/RXN) SerDes pairs. v0 is a data-value profile checking the pairs are present,
// host-complete, and not dangling; it does NOT model AC-coupling, on-die termination, or the optional
// MDIO/MDC management (datasheet concerns). See TestBuiltinsMatchYAML / builtins/sgmii.yaml.
var SGMII = Profile{
	Name: "SGMII", HostAttrKey: "interface", HostAttrVal: "SGMII",
	Signals: []Signal{
		{Name: "TXP", Suffix: "_TXP", Anchor: true},
		{Name: "TXN", Suffix: "_TXN"},
		{Name: "RXP", Suffix: "_RXP"},
		{Name: "RXN", Suffix: "_RXN"},
	},
	Requirements: []Requirement{{Type: "signal-missing"}, {Type: "host-incomplete"}, {Type: "signal-dangling"}},
}

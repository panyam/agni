package profiles

import _ "embed"

//go:embed builtins/pcie.yaml
var pcieYAML []byte

// PCIE is PCI Express: differential TX (PET) and RX (PER) lane pairs, a reference-clock pair, and
// PERST# reset. v0 is a data-value profile checking these are present, host-complete, and not
// dangling; it does NOT model the TX AC-coupling caps, per-lane multiplicity, or lane width
// (datasheet/topology concerns). See TestBuiltinsMatchYAML / builtins/pcie.yaml.
var PCIE = Profile{
	Name: "PCIe", HostAttrKey: "interface", HostAttrVal: "PCIe",
	Signals: []Signal{
		{Name: "PETP", Suffix: "_PETP", Anchor: true},
		{Name: "PETN", Suffix: "_PETN"},
		{Name: "PERP", Suffix: "_PERP"},
		{Name: "PERN", Suffix: "_PERN"},
		{Name: "REFCLKP", Suffix: "_REFCLKP"},
		{Name: "REFCLKN", Suffix: "_REFCLKN"},
		{Name: "PERST", Suffix: "_PERST"},
	},
	Requirements: []Requirement{{Type: "signal-missing"}, {Type: "host-incomplete"}, {Type: "signal-dangling"}},
}

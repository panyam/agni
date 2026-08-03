package profiles

import _ "embed"

//go:embed builtins/spi_nor.yaml
var spinorYAML []byte

// SPINOR is the SPI-NOR flash interface: chip-select (needs a pull-up), clock, and the four IO
// lines. Signals are matched by net-name suffix; CS is the anchor, since a SPI bus in use always
// has a chip-select. A standard SPI-NOR pinout is not customer-specific, so this profile is shared.
//
// TODO(WS3-045): the equivalent YAML declaration lives in builtins/spi_nor.yaml and is held identical
// to this literal by TestBuiltinsMatchYAML. Both forms are kept for now so the Go and YAML shapes can
// be compared side by side; once the YAML loader path is exercised in the field, make the YAML
// authoritative (var SPINOR = mustParse(spinorYAML)) and delete this literal.
var SPINOR = Profile{
	Name: "SPI_NOR",
	// A component that declares interface=SPI_NOR gets the precise host-anchored completeness check
	// (WS3-042); an un-annotated design (like the ACME EVT) still gets the convention path.
	HostAttrKey: "interface",
	HostAttrVal: "SPI_NOR",
	Signals: []Signal{
		{Name: "CS", Suffix: "_CS", PullUp: true, Anchor: true},
		{Name: "SCLK", Suffix: "_SCLK"},
		{Name: "IO0", Suffix: "_IO0"},
		{Name: "IO1", Suffix: "_IO1"},
		{Name: "IO2", Suffix: "_IO2"},
		{Name: "IO3", Suffix: "_IO3"},
	},
	Requirements: []Requirement{
		{Type: "signal-missing"},
		{Type: "host-incomplete"},
		{Type: "missing-pullup"},
		{Type: "signal-dangling"},
	},
}

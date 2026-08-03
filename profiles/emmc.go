package profiles

import _ "embed"

//go:embed builtins/emmc.yaml
var emmcYAML []byte

// EMMC is the eMMC (embedded MultiMediaCard) interface: a clock, a command line (needs a pull-up),
// eight data lines, and a reset. It has the SAME requirement shape as SPI-NOR on a differently-shaped
// bus (WS3-046). CMD is the anchor and the pulled-up line.
//
// v0 checks presence + CMD pull-up + dangling only. The DAT-line boot-mode strap and an active-low
// RST_N spelling are datasheet/strap concerns (ACME asks 62/68), deferred out of v0.
//
// TODO(WS3-045): the equivalent YAML declaration lives in builtins/emmc.yaml and is held identical to
// this literal by TestBuiltinsMatchYAML. Both forms are kept for now so the Go and YAML shapes can be
// compared side by side; once the YAML loader path is proven, make the YAML authoritative and delete
// this literal.
var EMMC = Profile{
	Name:        "eMMC",
	HostAttrKey: "interface",
	HostAttrVal: "eMMC",
	Signals: []Signal{
		{Name: "CMD", Suffix: "_CMD", PullUp: true, Anchor: true},
		{Name: "CLK", Suffix: "_CLK"},
		{Name: "DAT0", Suffix: "_DAT0"},
		{Name: "DAT1", Suffix: "_DAT1"},
		{Name: "DAT2", Suffix: "_DAT2"},
		{Name: "DAT3", Suffix: "_DAT3"},
		{Name: "DAT4", Suffix: "_DAT4"},
		{Name: "DAT5", Suffix: "_DAT5"},
		{Name: "DAT6", Suffix: "_DAT6"},
		{Name: "DAT7", Suffix: "_DAT7"},
		{Name: "RST", Suffix: "_RST"},
	},
	Requirements: []Requirement{
		{Type: "signal-missing"},
		{Type: "host-incomplete"},
		{Type: "missing-pullup"},
		{Type: "signal-dangling"},
	},
}

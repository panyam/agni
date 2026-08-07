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
var EMMC = mustParse(emmcYAML)

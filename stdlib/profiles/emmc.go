package profiles

import _ "embed"

//go:embed builtins/emmc.yaml
var emmcYAML []byte

// EMMC is the eMMC (embedded MultiMediaCard) interface, with a clock, a command line (needs a
// pull-up), eight data lines, and a reset. It has the SAME requirement shape as SPI-NOR on a
// differently-shaped bus (WS3-046). CMD is the anchor and the pulled-up line.
//
// It is judged one bus at a time, keyed on the CMD net's stem, and a bus carrying a card detect line
// is a removable SD card rather than an eMMC, so it is left alone (agni issue 1246).
//
// v0 checks presence + CMD pull-up + dangling only. The DAT-line boot-mode strap and an active-low
// RST_N spelling are datasheet/strap concerns, deferred out of v0.
var EMMC = mustParse(emmcYAML)

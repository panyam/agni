package profiles

import _ "embed"

//go:embed builtins/spi_nor.yaml
var spinorYAML []byte

// SPINOR is the SPI-NOR flash interface: chip-select (needs a pull-up), clock, and the four IO
// lines. Signals are matched by net-name suffix; CS is the anchor, since a SPI bus in use always
// has a chip-select. A standard SPI-NOR pinout is not customer-specific, so this profile is shared.
var SPINOR = mustParse(spinorYAML)

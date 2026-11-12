package profiles

import _ "embed"

//go:embed builtins/sd.yaml
var sdYAML []byte

// SD is a removable SD card interface: a command line, a clock, four data lines and a card detect,
// with pull-ups on the command and data lines, which idle open-drain while the card is identified. It
// is judged one bus at a time, keyed on the CMD net's stem, and a bus carrying an eighth data line or
// a reset is an eMMC, so it is left to the eMMC profile (agni issue 1294).
var SD = mustParse(sdYAML)

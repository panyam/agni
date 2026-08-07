package profiles

import _ "embed"

//go:embed builtins/a2b.yaml
var a2bYAML []byte

// A2B is the Automotive Audio Bus (ADI): a single twisted-pair line (A2B_P/A2B_N) carrying data and
// power, master→slave daisy-chained. v0 is a data-value profile checking the bus pair is present,
// host-complete, and not dangling; it does NOT model the coupling/termination network or the
// downstream I2S/I2C (datasheet/topology concerns).
var A2B = mustParse(a2bYAML)

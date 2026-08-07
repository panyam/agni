package profiles

import _ "embed"

//go:embed builtins/pcie.yaml
var pcieYAML []byte

// PCIE is PCI Express: differential TX (PET) and RX (PER) lane pairs, a reference-clock pair, and
// PERST# reset. v0 is a data-value profile checking these are present, host-complete, and not
// dangling; it does NOT model the TX AC-coupling caps, per-lane multiplicity, or lane width
// (datasheet/topology concerns).
var PCIE = mustParse(pcieYAML)

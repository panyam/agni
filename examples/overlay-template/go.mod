// TODO: rename this module to your own path, e.g. github.com/yourorg/agni-overlay.
module github.com/panyam/agni/examples/overlay-template

go 1.26.4

// TODO: pin a published engine version once you copy this module OUT of the agni repo,
// e.g. require github.com/panyam/agni v1.2.3
require github.com/panyam/agni v0.0.0

require google.golang.org/protobuf v1.36.11 // indirect

// TODO: DELETE this replace when you host the overlay outside the agni repo. It only exists so
// the in-repo template builds against the engine working tree (no release tag needed here).
replace github.com/panyam/agni => ../..

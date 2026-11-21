// TODO: rename this module to your own path, e.g. github.com/yourorg/agni-overlay.
module github.com/panyam/agni/examples/extension-template

go 1.27.4

// TODO: pin a published engine version once you copy this module OUT of the agni repo,
// e.g. require github.com/panyam/agni v1.2.3
require github.com/panyam/agni v0.0.0

require (
	github.com/andybalholm/brotli v1.2.7 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.6.5 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/klauspost/compress v1.20.2 // indirect
	github.com/pierrec/lz4/v4 v4.1.27 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/stangelandcl/ppmd v0.1.1 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	go4.org v0.0.0-20260112195520-a5071408f32f // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

// TODO: DELETE this replace when you host the overlay outside the agni repo. It only exists so
// the in-repo template builds against the engine working tree (no release tag needed here).
replace github.com/panyam/agni => ../..

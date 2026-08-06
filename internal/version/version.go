// Package version reports the engine build's identity, so an artifact the engine writes can name
// what produced it.
//
// It exists because a check-result document is meant to outlive the run that made it (WS3-103): a
// report read weeks later, or compared against another tool's, is only interpretable once you know
// which build produced it. Every other consumer of a version string is cosmetic; this one is
// load-bearing.
package version

import (
	"runtime/debug"
	"sync"
)

// stamped is the release identity a distribution build sets, e.g.
//
//	go build -ldflags "-X github.com/panyam/agni/internal/version.stamped=v0.4.1"
//
// It wins over the VCS metadata below, because a tagged release is a stronger claim than the commit
// it happened to be cut from.
var stamped string

var resolve = sync.OnceValue(func() string {
	if stamped != "" {
		return stamped
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		// A `go run` build, or a module consumed as a dependency: the module version is the only
		// identity available, and "(devel)" for a local build is itself honest.
		if v := info.Main.Version; v != "" {
			return v
		}
		return "unknown"
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	if dirty {
		// An uncommitted tree is not a reproducible build. A document that claims a bare commit for
		// one would be quietly wrong, and quietly wrong provenance is worse than none.
		return rev + "+dirty"
	}
	return rev
})

// Version returns the engine build identity: the ldflags-stamped release if a distribution build set
// one, else the VCS revision (suffixed "+dirty" when the tree had uncommitted changes), else the
// module version, else "unknown". It never returns the empty string, so a caller writing provenance
// always has something to write. Resolved once and cached.
func Version() string { return resolve() }

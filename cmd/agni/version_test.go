package main

import (
	"testing"

	"github.com/panyam/agni/internal/version"
)

// TestRootReportsVersion asserts `agni --version` is wired, and to the same source. Cobra only
// renders the flag when Version is non-empty, so an unset field silently drops the flag entirely.
func TestRootReportsVersion(t *testing.T) {
	root := rootCmd()
	if root.Version == "" {
		t.Fatal("root Version is empty, so --version is not registered at all")
	}
	if root.Version != version.Version() {
		t.Errorf("--version reports %q but provenance records %q", root.Version, version.Version())
	}
}

package version

import "testing"

// TestVersionIsNeverEmpty pins the one property callers depend on. A results document stamps this
// into its provenance unconditionally. An empty string would produce a document that looks complete
// and names no producer build. That is worse than a document that admits it does not know.
func TestVersionIsNeverEmpty(t *testing.T) {
	if Version() == "" {
		t.Error("Version() is empty; a document would then record a producer with no build identity")
	}
}

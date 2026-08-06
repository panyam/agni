package version

import "testing"

// TestVersionIsNeverEmpty pins the one property callers depend on. A results document stamps this
// into its provenance unconditionally, so an empty string would produce a document that names no
// producer build while looking complete — worse than one that admits it does not know.
func TestVersionIsNeverEmpty(t *testing.T) {
	if Version() == "" {
		t.Error("Version() is empty; a document would then record a producer with no build identity")
	}
}

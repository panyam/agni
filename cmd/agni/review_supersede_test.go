package main

import (
	"strings"
	"testing"

	"github.com/panyam/agni/stdlib/profiles"
)

// The review's profile index must track the catalog (WS3-056). reviewClosures reports an interface as
// evaluating when ANY profile under its name is in use, and unions all of their nets for scoping. If
// the index kept the built-in while the catalog superseded its rules, the gate would clear on a profile
// whose rules are not in the run, and an item scoped by it would score a clean pass on an interface
// nothing checked. That is the WS3-090 twin disagreement, and it is silent by construction, which is
// why it is asserted here rather than left to the end-to-end review output.
func TestReviewIndexDropsSupersededBuiltinProfile(t *testing.T) {
	p, err := profiles.Load(strings.NewReader("override: SPI_NOR\nsuffixes: {IO0: _DQ0}\n"))
	if err != nil {
		t.Fatalf("Load naming map: %v", err)
	}
	_, byName, err := composeReviewInputsFrom([]profiles.Profile{p}, "")
	if err != nil {
		t.Fatalf("composeReviewInputsFrom: %v", err)
	}
	got := byName["SPI_NOR"]
	if len(got) != 1 {
		t.Fatalf("byName[SPI_NOR] has %d profiles, want 1 (the overlay replaces the built-in)", len(got))
	}
	// The surviving profile must be the OVERLAY's reading, identified by the re-bound suffix.
	if !hasSignalSuffix(got[0], "_DQ0") {
		t.Errorf("byName[SPI_NOR] kept the built-in profile, not the overlay (no _DQ0 signal)")
	}
	// An interface the overlay says nothing about is untouched.
	if len(byName["CAN"]) != 1 {
		t.Errorf("byName[CAN] has %d profiles, want the built-in left alone", len(byName["CAN"]))
	}
}

// Two overlay profiles sharing one name never reach the index: they compile to identical rule names,
// which catalog composition rejects as a duplicate before any of this runs. Pinned because the review
// index code reads as though it has to handle the case, and a future reader would otherwise be right
// to wonder what happens. Composition, not the index, is where that is decided.
func TestTwoOverlayProfilesOfOneNameAreRejectedAtComposition(t *testing.T) {
	a, err := profiles.Load(strings.NewReader("override: SPI_NOR\nsuffixes: {IO0: _DQ0}\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	b, err := profiles.Load(strings.NewReader("override: SPI_NOR\nsuffixes: {IO0: _QD0}\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("two overlay profiles of one name composed cleanly, want a duplicate-name rejection")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "duplicate rule name") {
			t.Errorf("panic = %v, want a duplicate rule name composition error", r)
		}
	}()
	composeReviewInputsFrom([]profiles.Profile{a, b}, "") //nolint:errcheck // panics by design
}

func hasSignalSuffix(p profiles.Profile, suffix string) bool {
	for _, s := range p.Signals {
		if s.Suffix == suffix {
			return true
		}
	}
	return false
}

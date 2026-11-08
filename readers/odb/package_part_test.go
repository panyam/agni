package odb

import (
	"slices"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestAMechanicalPinIsNoPinOfThePart reads two packages whose PIN records carry an electrical type.
// The fiducial's one pin is M, so its part type has no pins. The resistor's pins are E and U, so
// both stay (agni issue 1437).
func TestAMechanicalPinIsNoPinOfThePart(t *testing.T) {
	e, err := parseEDA([]byte("UNITS=MM\n" +
		"PKG FID 1.0 -0.5 -0.5 0.5 0.5\n" +
		"PIN 1 S 0 0 0 M S\n" +
		"PKG R0402 1.0 -0.5 -0.5 0.5 0.5\n" +
		"PIN 1 S -0.5 0 0 E S\n" +
		"PIN 2 S 0.5 0 0 U S\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(e.pkgs) != 2 {
		t.Fatalf("read %d packages, want 2", len(e.pkgs))
	}
	prov := func() *ir.Provenance { return nil }
	for i, want := range [][]string{nil, {"1", "2"}} {
		var got []string
		for _, p := range packagePart(e.pkgs[i], prov).GetPins() {
			got = append(got, p.GetDesignator())
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s pins %v, want %v", e.pkgs[i].name, got, want)
		}
	}
}

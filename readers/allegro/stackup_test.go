package allegro

import (
	"math"
	"testing"
)

// A 16.6 cross-section row writes its thickness in metres as a bare number, or as the length the
// user typed with its unit, as Clay R6's "8 MIL" (agni 1601).
func TestA166ThicknessReadsInMetresOrItsUnit(t *testing.T) {
	for in, want := range map[string]float64{
		"4.000000000000e-005": 40e-6,
		"8 MIL":               203.2e-6,
		"0.2 mm":              200e-6,
		"35 UM":               35e-6,
		"0 MIL":               0,
		"":                    0,
		"8 FURLONGS":          0,
	} {
		if got := metres(in); math.Abs(got-want) > 1e-12 {
			t.Errorf("metres(%q) = %g, want %g", in, got, want)
		}
	}
}

// A 17.2 double is stored high word first: 4.5 is 0x40120000 then 0.
func TestASwappedDoubleReadsHighWordFirst(t *testing.T) {
	b := []byte{0x00, 0x00, 0x12, 0x40, 0, 0, 0, 0}
	if got := swappedDouble(b, 0); got != 4.5 {
		t.Errorf("swappedDouble = %g, want 4.5", got)
	}
}

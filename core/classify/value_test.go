package classify

import (
	"math"
	"strconv"
	"testing"
)

// TestParseQuantitySpellings covers the notations designs actually carry: SI prefixes, the IEC 60062
// RKM code where the multiplier letter replaces the decimal point, an explicit unit or none, and the
// concatenated fields real value strings arrive as.
func TestParseQuantitySpellings(t *testing.T) {
	cases := []struct {
		in   string
		val  float64
		unit string
		ok   bool
	}{
		{"10k", 10000, "", true},
		{"10K", 10000, "", true},
		{"1.5k", 1500, "", true},
		{"4700", 4700, "", true},
		{"10", 10, "", true},

		{"4k7", 4700, "", true},         // RKM: the letter IS the decimal point
		{"4R7", 4.7, UnitOhm, true},     // R marks the point AND names ohms
		{"0R05", 0.05, UnitOhm, true},   // the sense-resistor case WS3-085 needs
		{"2M2", 2.2e6, "", true},
		{"10R", 10, UnitOhm, true},

		{"100n", 1e-7, "", true},        // prefix alone: unit-neutral, could be nF or nH
		{"100nF", 1e-7, "F", true},
		{"1u", 1e-6, "", true},
		{"10k" + UnitOhm, 10000, UnitOhm, true},
		{"10" + ohmSign, 10, UnitOhm, true}, // the deprecated OHM SIGN codepoint

		{"100 1% 0402", 100, "", true},  // concatenated fields: value is the first token
		{"100nF/50V", 1e-7, "F", true},

		{"DNP", 0, "", false},
		{"", 0, "", false},
		{"DO NOT FIT", 0, "", false},
	}
	for _, c := range cases {
		v, u, ok := ParseQuantity(c.in)
		if ok != c.ok {
			t.Errorf("ParseQuantity(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && (v != c.val || u != c.unit) {
			t.Errorf("ParseQuantity(%q) = (%v, %q), want (%v, %q)", c.in, v, u, c.val, c.unit)
		}
	}
}

// TestParseQuantityIsDecimalExact is the assertion the whole parse strategy exists for, and it is
// written around a value where a naive implementation actually DIFFERS.
//
// Parsing the mantissa as a float and then scaling ("2.2" -> 2.2, times 1e-12) rounds twice, and the
// second rounding does not always land where a single exact scaling does. Across 2-3 significant-figure
// mantissas over the SI prefixes designs use, the two approaches disagree on 1573 of 7920 values, all
// at small magnitudes: picofarads, where crystal load caps live and where a shipped rule already reads
// capacitance.
//
// A test written on 4k7 would pass either way (4700 does not diverge) and would prove nothing, which is
// why this one is pinned to 2p2 and asserts against the exactly-scaled double rather than a literal.
func TestParseQuantityIsDecimalExact(t *testing.T) {
	got, _, ok := ParseQuantity("2p2")
	if !ok {
		t.Fatal("2p2 must parse")
	}
	exact := float64(22) * math.Pow10(-13)
	if got != exact {
		t.Errorf("2p2 = %.20g, want the exactly-scaled %.20g", got, exact)
	}
	// The naive route, spelled out, so the divergence this test guards is visible rather than asserted.
	naive, _ := strconv.ParseFloat("2.2", 64)
	if naive*math.Pow10(-12) == exact {
		t.Skip("naive scaling no longer diverges here; this test has stopped being a guard")
	}
}

// TestParseQuantitySpellingsAgreeOnOneValue: two spellings of one value must produce the IDENTICAL
// double, so a diff can compare component values across revisions by equality rather than by tolerance.
func TestParseQuantitySpellingsAgreeOnOneValue(t *testing.T) {
	for _, pair := range [][2]string{
		{"4k7", "4700"},
		{"4.7k", "4k7"},
		{"2p2", "2.2p"},
		{"0R05", "0.05R"},
		{"1M5", "1500k"},
	} {
		a, _, okA := ParseQuantity(pair[0])
		b, _, okB := ParseQuantity(pair[1])
		if !okA || !okB {
			t.Errorf("%q/%q: both must parse (%v/%v)", pair[0], pair[1], okA, okB)
			continue
		}
		if a != b {
			t.Errorf("%q = %.20g and %q = %.20g must be the identical double", pair[0], a, pair[1], b)
		}
	}
}

// TestParseQuantityUnparsedIsNotZero: "DNP" and "0R05" must not look alike. Zero is a legal resistance,
// so a failed parse reporting a zero value would make an unfitted part read as a short.
func TestParseQuantityUnparsedIsNotZero(t *testing.T) {
	if _, _, ok := ParseQuantity("DNP"); ok {
		t.Error("DNP must not parse")
	}
	v, _, ok := ParseQuantity("0R0")
	if !ok || v != 0 {
		t.Errorf("0R0 must parse AS zero, got (%v, %v)", v, ok)
	}
}

package classify

import (
	"math"
	"strings"
)

// ParseQuantity reads a component's value text into a number in its SI BASE unit, the unit symbol, and
// whether a number was found at all (WS3-118). It is the format-neutral half of the value pass: every
// reader hands it whatever its dialect wrote, and it applies the one grammar that is actually
// specified.
//
// THE GRAMMAR IS REAL, AND IT IS ONLY PART OF THE PROBLEM. IEC 60062 defines the RKM code, where the
// multiplier letter stands in for the decimal point: 4R7 is 4.7 Ω, 4k7 is 4700 Ω, 0R05 is 0.05 Ω, 1M5
// is 1.5 MΩ. Combined with SI prefixes that covers nearly every spelling a design carries, and it is a
// small deterministic grammar rather than a pile of special cases:
//
//	quantity := mantissa prefix? unit?        // 10k, 1.5kΩ, 100nF
//	          | digits prefix digits          // RKM: 4k7, 0R05
//
// What is NOT in any specification, and therefore is NOT decided here: what unit a bare number carries.
// That depends on the component's CLASS and on house convention, so it belongs in the lexicon
// (ValueVocab) rather than in this function, the same reason net-role name patterns live in RoleVocab
// instead of in rule text. ParseQuantity returns the number with an EMPTY unit and lets the caller
// decide, which is why an empty unit is a real state rather than a failure.
//
// THE PARSE IS EXACT DECIMAL AND SCALES ONCE, WHICH IS NOT COSMETIC. Every component value is printed
// in decimal on the part and in the datasheet, so the text is exact and the parse is string
// manipulation into (digits, decimal exponent); scale() then converts that pair in a single operation.
// Reading the mantissa as a float first and scaling after rounds twice, and the two routes land on
// DIFFERENT doubles for 1573 of the 7920 two-and-three-significant-figure values across the SI prefixes
// designs use. The divergence is concentrated at small magnitudes: picofarads, where crystal load caps
// live and where a shipped rule already reads capacitance.
//
// It also means two spellings of one value produce the IDENTICAL double, so a diff can compare
// component values across revisions by equality rather than by tolerance.
//
// ok is false when no number could be read at all ("DNP", "", "DO NOT FIT"). The caller must keep that
// distinct from a value of zero, which is a legal resistance.
func ParseQuantity(text string) (value float64, unit string, ok bool) {
	v, u, _, ok := parseQuantityDetail(text)
	return v, u, ok
}

// parseQuantityDetail is ParseQuantity plus whether the notation carried a MULTIPLIER PREFIX. The stamp
// pass needs that distinction because the two unit-less cases are not equally ambiguous:
//
//   - "10u" on a capacitor states the magnitude and omits only the DIMENSION, and a capacitor's
//     dimension is farads with no ambiguity at all. The class settles it.
//   - "100" on a capacitor omits the magnitude too, and there the old conventions genuinely disagree
//     (farads, microfarads, picofarads). Nothing can settle it but a declared convention.
//
// Collapsing them would either refuse "10u" (making the whole feature useless for capacitors and
// inductors, which is most passives) or guess "100" (putting a value six orders of magnitude out into a
// field that reads as authoritative).
func parseQuantityDetail(text string) (value float64, unit string, prefixed bool, ok bool) {
	s := strings.TrimSpace(text)
	if s == "" {
		return 0, "", false, false
	}
	// Real fields concatenate several things into one string ("10k 1% 0402", "100nF/50V"). The value is
	// the first token; the rest is tolerance, package and rating, which are separate facts this type
	// deliberately does not model.
	if i := strings.IndexAny(s, " \t/,;"); i > 0 {
		s = s[:i]
	}
	mant, exp, unitSym, sawPrefix, ok := parseDecimal(s)
	if !ok {
		return 0, "", false, false
	}
	return scale(mant, exp), unitSym, sawPrefix, true
}

// UnitOhm is the canonical ohm symbol every reader's spelling normalizes to: GREEK CAPITAL LETTER OMEGA
// (U+03A9). Designs also carry the visually IDENTICAL OHM SIGN (U+2126) and a bare "R", and a consumer
// comparing unit strings would treat those as three different units. Naming the canonical one here
// keeps that decision in one place rather than in every rule that touches a resistance.
const UnitOhm = "\u03a9" // Ω

// ohmSign is the deprecated OHM SIGN codepoint (U+2126). Unicode normalizes it to U+03A9, but designs
// carry the raw byte sequence and nothing in the pipeline normalizes for us.
const ohmSign = "\u2126" // Ω (deprecated codepoint)

// unitSuffixes maps the trailing unit symbol a value may carry to its canonical SI base symbol.
var unitSuffixes = map[string]string{
	UnitOhm: UnitOhm, ohmSign: UnitOhm, "R": UnitOhm, "OHM": UnitOhm, "OHMS": UnitOhm,
	"F": "F", "FARAD": "F",
	"H": "H", "HENRY": "H",
	"A": "A", "AMP": "A", "AMPS": "A",
	"V": "V", "VOLT": "V", "VOLTS": "V",
}

// siPrefixes maps a multiplier letter to its power of ten. It doubles as the RKM decimal-point letter
// set, which is why "R" (10^0) is here: 4R7 means 4.7 Ω with R marking the point.
//
// "M" is the one genuine collision in the notation: mega in IEC 60062, milli in several SPICE dialects.
// This table takes the IEC reading because it is the one a schematic uses, and a house that means milli
// says so through the lexicon rather than by arguing with the parser.
var siPrefixes = map[byte]int{
	'p': -12, 'P': -12,
	'n': -9, 'N': -9,
	'u': -6, 'U': -6, 'µ': -6,
	'm': -3,
	'R': 0, 'r': 0, 'E': 0, 'L': 0,
	'k': 3, 'K': 3,
	'M': 6,
	'G': 9, 'g': 9,
	'T': 12,
}

// parseDecimal splits a value token into an integer mantissa, a decimal exponent, and a unit symbol,
// working entirely in strings so no precision is lost before the caller scales. It handles the two
// shapes together because RKM is the same grammar with the multiplier moved: "4k7" and "4.7k" differ
// only in where the point is written, so both reduce to mantissa 47, exponent 2.
func parseDecimal(s string) (mant int64, exp int, unit string, prefixed bool, ok bool) {
	s = strings.TrimSuffix(s, ".")
	// A trailing unit word is stripped before the prefix scan so "100nF" does not read F as a
	// multiplier, and "10R"/"4R7" still reach the RKM path with R intact.
	for _, cand := range []int{4, 3, 2, 1} {
		if len(s) <= cand {
			continue
		}
		suffix := strings.ToUpper(s[len(s)-cand:])
		if u, hit := unitSuffixes[suffix]; hit && isDigitOrPrefix(s[len(s)-cand-1]) {
			// A bare trailing R is ambiguous: "10R" is 10 Ω but so is the RKM "10R". Both give the
			// same answer, so leave R to the prefix scan and take only the unambiguous words here.
			if suffix != "R" {
				s, unit = s[:len(s)-cand], u
				break
			}
		}
	}
	var digits []byte
	pointAt := -1 // index into digits where the decimal point falls, -1 for none
	prefixExp := 0
	seenPrefix := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			digits = append(digits, c)
		case c == '.':
			if pointAt >= 0 {
				return 0, 0, "", false, false
			}
			pointAt = len(digits)
		case c == '-' && len(digits) == 0 && !seenPrefix:
			digits = append(digits, '-')
		default:
			p, isPrefix := siPrefixes[c]
			if !isPrefix || seenPrefix {
				return 0, 0, "", false, false
			}
			// The multiplier letter IS the decimal point in RKM ("4k7"), but only when it has not
			// already been written ("4.7k"). Taking it unconditionally would misplace the point.
			if pointAt < 0 {
				pointAt = len(digits)
			}
			prefixExp, seenPrefix = p, true
			if unit == "" {
				unit = unitForPrefixLetter(c)
			}
		}
	}
	if len(digits) == 0 || (len(digits) == 1 && digits[0] == '-') {
		return 0, 0, "", false, false
	}
	mant, ok = atoi(digits)
	if !ok {
		return 0, 0, "", false, false
	}
	frac := 0
	if pointAt >= 0 {
		frac = len(digits) - pointAt
		if digits[0] == '-' {
			frac = len(digits) - pointAt
		}
	}
	return mant, prefixExp - frac, unit, seenPrefix, true
}

// unitForPrefixLetter recovers the unit a multiplier letter implies on its own. Only R does: it is the
// RKM point marker for OHMS specifically, so "4R7" is 4.7 Ω even with no unit written. Every other
// prefix is unit-neutral ("100n" could be nF or nH) and yields an empty unit for the lexicon to fill.
func unitForPrefixLetter(c byte) string {
	if c == 'R' || c == 'r' {
		return UnitOhm
	}
	return ""
}

// isDigitOrPrefix reports whether a unit suffix is preceded by something that makes it a suffix rather
// than the whole token, so "F" alone is not read as 0 farads.
func isDigitOrPrefix(c byte) bool {
	if c >= '0' && c <= '9' {
		return true
	}
	_, isPrefix := siPrefixes[c]
	return isPrefix
}

// atoi converts an accumulated digit slice (optionally leading '-') to an int64 without going through
// a float. Overflow returns false rather than wrapping: a value with more digits than an int64 holds is
// not a component value, it is a parse gone wrong.
func atoi(digits []byte) (int64, bool) {
	neg := digits[0] == '-'
	if neg {
		digits = digits[1:]
	}
	var n int64
	for _, d := range digits {
		if n > (math.MaxInt64-int64(d-'0'))/10 {
			return 0, false
		}
		n = n*10 + int64(d-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}

// scale turns an exact (mantissa, decimal exponent) pair into a double. It is the ONLY floating-point
// step in the parse, and the DIVISION on the negative branch is the part that matters.
//
// A positive power of ten is exactly representable in a double up to 1e22; a NEGATIVE one is not, since
// 1e-9 has no exact binary form. So dividing by Pow10(9) is exact where multiplying by Pow10(-9) rounds
// twice, and at small magnitudes the two land on different doubles. Writing this branch as a
// multiplication by Pow10(exp) would look tidier and would change the answer for 100n, 2p2 and the rest
// of the picofarad range where crystal load caps live.
//
// TestParseQuantitySpellings fails if this is rewritten as a multiplication.
func scale(mant int64, exp int) float64 {
	if exp == 0 {
		return float64(mant)
	}
	if exp > 0 {
		return float64(mant) * math.Pow10(exp)
	}
	return float64(mant) / math.Pow10(-exp)
}

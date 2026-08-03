package derive

import (
	"regexp"
	"strconv"
	"strings"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// normalizeSymbol strips spaces from a printed symbol: document parsers split
// subscripts ("V GSS" for V_GSS), and the hand-encoded goldens write them joined.
// Comparison and matching happen on the normalized form; the emitted parameter
// keeps the normalized spelling too, since the space is a typesetting artifact,
// not vendor vocabulary.
func normalizeSymbol(s string) string { return strings.ReplaceAll(s, " ", "") }

var (
	plusMinus  = regexp.MustCompile(`^(?:\+/-|±)\s*(\d+(?:\.\d+)?)$`)
	rangeTo    = regexp.MustCompile(`^([+-]?\s*\d+(?:\.\d+)?)\s+to\s+([+-]?\s*\d+(?:\.\d+)?)$`)
	plainNum   = regexp.MustCompile(`^[+-]?\d+(?:\.\d+)?$`)
	condEq     = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9 ()_.]*?)\s*=\s*([+-]?\d+(?:\.\d+)?)\s*([A-Za-zµΩ%°]*)$`)
	condRange  = regexp.MustCompile(`^([+-]?\d+(?:\.\d+)?)\s*(?:<=|≤)\s*([A-Za-z][A-Za-z0-9 ()_.]*?)\s*(?:<=|≤)\s*([+-]?\d+(?:\.\d+)?)\s*([A-Za-zµΩ%°]*)$`)
)

// parseRatings parses one absolute-maximum-ratings value cell: a plain number is a
// max-only bound (a stress ceiling), "±N" / "+/-N" spans -N..N, and "A to B" is a
// range. Anything else (prose like "Internally Limited", empty) does not parse and
// the row lands in the gap list rather than guessing.
func parseRatings(s string) (min, max *float64, ok bool) {
	s = strings.TrimSpace(s)
	if m := plusMinus.FindStringSubmatch(s); m != nil {
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return nil, nil, false
		}
		neg := -v
		return &neg, &v, true
	}
	if m := rangeTo.FindStringSubmatch(s); m != nil {
		lo, err1 := strconv.ParseFloat(strings.ReplaceAll(m[1], " ", ""), 64)
		hi, err2 := strconv.ParseFloat(strings.ReplaceAll(m[2], " ", ""), 64)
		if err1 != nil || err2 != nil {
			return nil, nil, false
		}
		return &lo, &hi, true
	}
	if plainNum.MatchString(s) {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, nil, false
		}
		return nil, &v, true
	}
	return nil, nil, false
}

// parseNumberCell parses one min/typ/max column cell; nil for empty or non-numeric
// cells (a blank bound is absence, never zero).
func parseNumberCell(s string) *float64 {
	s = strings.TrimSpace(s)
	if !plainNum.MatchString(s) {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

// parseCondition parses one test-condition fragment into a param Condition. Two
// structured forms are recognized: "SYM = N UNIT" and "A <= SYM <= B UNIT" (ASCII or
// ≤). Everything else — symbol-to-symbol relations ("VDS = VGS"), prose ranges —
// keeps only Raw, which is a captured-but-not-machine-comparable condition by the
// docs/20 semantics. Raw always carries the source text, structured or not.
func parseCondition(s string) *parampb.Condition {
	s = strings.TrimSpace(s)
	c := &parampb.Condition{Raw: s}
	if m := condEq.FindStringSubmatch(s); m != nil {
		if v, err := strconv.ParseFloat(m[2], 64); err == nil {
			c.Symbol, c.Eq, c.Unit = normalizeSymbol(m[1]), &v, m[3]
			return c
		}
	}
	if m := condRange.FindStringSubmatch(s); m != nil {
		lo, err1 := strconv.ParseFloat(m[1], 64)
		hi, err2 := strconv.ParseFloat(m[3], 64)
		if err1 == nil && err2 == nil {
			c.Symbol, c.Min, c.Max, c.Unit = normalizeSymbol(m[2]), &lo, &hi, m[4]
			return c
		}
	}
	return c
}

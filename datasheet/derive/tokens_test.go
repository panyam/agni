package derive

import "testing"

func f(v float64) *float64 { return &v }

func eqPtr(a, b *float64) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || *a == *b
}

func TestParseRatings(t *testing.T) {
	cases := []struct {
		in       string
		min, max *float64
		ok       bool
	}{
		{"50", nil, f(50), true},
		{"0.22", nil, f(0.22), true},
		{"+/-20", f(-20), f(20), true},
		{"±20", f(-20), f(20), true},
		{"± 20", f(-20), f(20), true},
		{"-55 to +150", f(-55), f(150), true},
		{"-55 to 150", f(-55), f(150), true},
		{"- 55 to +150", f(-55), f(150), true},
		{"Internally Limited", nil, nil, false},
		{"", nil, nil, false},
	}
	for _, tc := range cases {
		min, max, ok := parseRatings(tc.in)
		if ok != tc.ok || !eqPtr(min, tc.min) || !eqPtr(max, tc.max) {
			t.Errorf("parseRatings(%q) = %v,%v,%v want %v,%v,%v", tc.in, min, max, ok, tc.min, tc.max, tc.ok)
		}
	}
}

func TestParseNumberCell(t *testing.T) {
	cases := []struct {
		in   string
		want *float64
	}{
		{"3.5", f(3.5)}, {"0.8", f(0.8)}, {"-2", f(-2)}, {"+150", f(150)},
		{"", nil}, {"~", nil}, {"1.0", f(1)},
	}
	for _, tc := range cases {
		if got := parseNumberCell(tc.in); !eqPtr(got, tc.want) {
			t.Errorf("parseNumberCell(%q) = %v want %v", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeSymbol(t *testing.T) {
	cases := [][2]string{
		{"V GSS", "VGSS"},
		{"RDS(on)", "RDS(on)"},
		{"R DS(on)", "RDS(on)"},
		{"TJ, TSTG", "TJ,TSTG"},
	}
	for _, tc := range cases {
		if got := normalizeSymbol(tc[0]); got != tc[1] {
			t.Errorf("normalizeSymbol(%q) = %q want %q", tc[0], got, tc[1])
		}
	}
}

// parseCondition covers the two structured forms; everything else stays raw-only,
// which the schema represents honestly (MachineComparable then excludes it).
func TestParseCondition(t *testing.T) {
	cases := []struct {
		in             string
		symbol         string
		eq, min, max   *float64
		unit           string
		structured     bool
	}{
		{"VGS = 10 V", "VGS", f(10), nil, nil, "V", true},
		{"ID = 0.22 A", "ID", f(0.22), nil, nil, "A", true},
		{"TJ = 125C", "TJ", f(125), nil, nil, "C", true},
		{"V GS = 4.5 V", "VGS", f(4.5), nil, nil, "V", true},
		{"0 <= IOUT <= 800 mA", "IOUT", nil, f(0), f(800), "mA", true},
		{"0 ≤ IOUT ≤ 800 mA", "IOUT", nil, f(0), f(800), "mA", true},
		{"VDS = VGS", "", nil, nil, nil, "", false},
		{"over the junction temperature range", "", nil, nil, nil, "", false},
	}
	for _, tc := range cases {
		c := parseCondition(tc.in)
		if c.Raw != tc.in {
			t.Errorf("parseCondition(%q): raw must always carry the source text, got %q", tc.in, c.Raw)
		}
		structured := c.Eq != nil || c.Min != nil || c.Max != nil
		if structured != tc.structured {
			t.Errorf("parseCondition(%q) structured = %v, want %v (%+v)", tc.in, structured, tc.structured, c)
			continue
		}
		if !tc.structured {
			continue
		}
		if c.Symbol != tc.symbol || !eqPtr(c.Eq, tc.eq) || !eqPtr(c.Min, tc.min) || !eqPtr(c.Max, tc.max) || c.Unit != tc.unit {
			t.Errorf("parseCondition(%q) = %+v, want sym=%s eq=%v min=%v max=%v unit=%s",
				tc.in, c, tc.symbol, tc.eq, tc.min, tc.max, tc.unit)
		}
	}
}

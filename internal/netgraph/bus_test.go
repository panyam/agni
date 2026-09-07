package netgraph

import "testing"

// TestExpandBusName: a range bus expands to prefix+index members in the written direction; a scalar
// or non-bus name yields nil. Both dialect spellings of the range are accepted, because one helper
// serves every reader: xschem and gEDA write `DATA[7:0]`, KiCad writes `AN[0..7]` and only that.
func TestExpandBusName(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"DATA[1:0]", []string{"DATA1", "DATA0"}},
		{"A[0:2]", []string{"A0", "A1", "A2"}},
		{"AN[0..7]", []string{"AN0", "AN1", "AN2", "AN3", "AN4", "AN5", "AN6", "AN7"}},
		{"D[3..0]", []string{"D3", "D2", "D1", "D0"}},
		{"IRQ-[1..7]", []string{"IRQ-1", "IRQ-2", "IRQ-3", "IRQ-4", "IRQ-5", "IRQ-6", "IRQ-7"}},
		{"D[3]", nil},     // scalar index, not a range bus
		{"D[0...3]", nil}, // three dots is neither dialect
		{"PLAIN", nil},
	}
	for _, tc := range cases {
		got := ExpandBusName(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("ExpandBusName(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("ExpandBusName(%q) = %v, want %v", tc.in, got, tc.want)
				break
			}
		}
	}
	for _, name := range []string{"D[3:0]", "AN[0..7]"} {
		if !IsBusName(name) {
			t.Errorf("IsBusName(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"D[3]", "PLAIN", "D[a..b]"} {
		if IsBusName(name) {
			t.Errorf("IsBusName(%q) = true, want false", name)
		}
	}
}

package netgraph

import "testing"

// TestExpandBusName: a range bus expands to prefix+index members, MSB-first, honoring the written
// direction; a scalar or non-bus name yields nil.
func TestExpandBusName(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"DATA[1:0]", []string{"DATA1", "DATA0"}},
		{"A[0:2]", []string{"A0", "A1", "A2"}},
		{"D[3]", nil}, // scalar index, not a range bus
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
	if IsBusName("D[3]") || !IsBusName("D[3:0]") {
		t.Error("IsBusName: range has a colon, scalar does not")
	}
}

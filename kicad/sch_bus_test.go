package kicad

import (
	"bytes"
	"testing"
)

// TestCollectBuses: the reader records NAMED buses with their member set (WS1-034). The alias fixture
// yields the bus_alias "DATA" with its explicit members; the resolved fixture yields the range-bus
// "DATA[1:0]" with the range-expanded members. The bare bus/bus_entry wire geometry is not flagged on
// its own (the label/alias names the bus).
func TestCollectBuses(t *testing.T) {
	cases := []struct {
		fixture     string
		wantKind    string
		wantLabel   string
		wantMembers []string
	}{
		{"bus.kicad_sch", "bus_alias", "DATA", []string{"DATA0", "DATA1"}},
		{"bus_resolved.kicad_sch", "bus", "DATA[1:0]", []string{"DATA1", "DATA0"}},
	}
	for _, tc := range cases {
		d, err := ReadSchematic(bytes.NewReader(readFixture(t, tc.fixture)), tc.fixture)
		if err != nil {
			t.Fatalf("%s: %v", tc.fixture, err)
		}
		bs := d.GetInputDiagnostics().GetUnmodeledBuses()
		if len(bs) != 1 {
			t.Fatalf("%s: unmodeled buses = %d, want 1", tc.fixture, len(bs))
		}
		b := bs[0]
		if b.GetKind() != tc.wantKind || b.GetLabel() != tc.wantLabel {
			t.Errorf("%s: bus = {kind:%q label:%q}, want {%q %q}", tc.fixture, b.GetKind(), b.GetLabel(), tc.wantKind, tc.wantLabel)
		}
		if got := b.GetMembers(); !equalStrings(got, tc.wantMembers) {
			t.Errorf("%s: members = %v, want %v", tc.fixture, got, tc.wantMembers)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

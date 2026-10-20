package kicad

import (
	"bytes"
	"slices"
	"testing"
)

// TestCollectBuses checks that the reader records NAMED buses with their member set (WS1-034). The
// resolved fixture yields the range-bus "DATA[1:0]" with the range-expanded members. The alias fixture
// only DECLARES `bus_alias "DATA"` and draws no bus that uses it, so it records nothing: a declaration
// is a member list, not a bus (agni issue 939). The bare bus/bus_entry wire geometry is not flagged on
// its own.
func TestCollectBuses(t *testing.T) {
	cases := []struct {
		fixture     string
		wantKind    string
		wantLabel   string
		wantMembers []string
	}{
		{"bus.kicad_sch", "", "", nil},
		{"bus_resolved.kicad_sch", "bus", "DATA[0..1]", []string{"DATA0", "DATA1"}},
	}
	for _, tc := range cases {
		d, err := ReadSchematic(bytes.NewReader(readFixture(t, tc.fixture)), tc.fixture)
		if err != nil {
			t.Fatalf("%s: %v", tc.fixture, err)
		}
		bs := d.GetInputDiagnostics().GetUnmodeledBuses()
		if tc.wantLabel == "" {
			if len(bs) != 0 {
				t.Errorf("%s: an alias declaration was recorded as a bus: %v", tc.fixture, bs)
			}
			continue
		}
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

// TestColonLabelIsARangeLabelAndNoBus pins KiCad's reading of `DATA[1:0]`, a plain scalar net, so the
// reader records no bus for it and records the label as range-spelled and not read as a bus, which is
// what bus-spelling-not-a-bus reports (agni issue 605). The fixture is bus_resolved with only the
// spelling changed; kicad-cli exports no net named by either spelling, and the member nets alike.
func TestColonLabelIsARangeLabelAndNoBus(t *testing.T) {
	src := bytes.ReplaceAll(readFixture(t, "bus_resolved.kicad_sch"), []byte(`"DATA[0..1]"`), []byte(`"DATA[1:0]"`))
	d, err := ReadSchematic(bytes.NewReader(src), "bus_resolved.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	if bs := d.GetInputDiagnostics().GetUnmodeledBuses(); len(bs) != 0 {
		t.Errorf("a colon-spelled label is no KiCad bus, got buses %v", bs)
	}
	rs := d.GetInputDiagnostics().GetRangeLabels()
	if len(rs) != 1 || rs[0].GetLabel() != "DATA[1:0]" || rs[0].GetReadsAsBus() {
		t.Errorf("range labels = %v, want DATA[1:0] not read as a bus", rs)
	}

	d, err = ReadSchematic(bytes.NewReader(readFixture(t, "bus_resolved.kicad_sch")), "bus_resolved.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	rs = d.GetInputDiagnostics().GetRangeLabels()
	if len(rs) != 1 || rs[0].GetLabel() != "DATA[0..1]" || !rs[0].GetReadsAsBus() {
		t.Errorf("range labels = %v, want DATA[0..1] read as a bus", rs)
	}
	if !slices.Contains(d.GetInputDiagnostics().GetSupplied(), "range_labels") {
		t.Error("the reader looked at every label, so it must declare range_labels")
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

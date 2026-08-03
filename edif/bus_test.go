package edif

import (
	"bytes"
	"os"
	"testing"
)

// wantMembers is the DATA[0..7] member set an (array DATA 8) port expands to (WS1-034 Phase 2).
var wantMembers = []string{"DATA[0]", "DATA[1]", "DATA[2]", "DATA[3]", "DATA[4]", "DATA[5]", "DATA[6]", "DATA[7]"}

// TestEDIFArrayBusDetected: an EDIF `array` bus port is recorded as an unmodeled-bus diagnostic with
// the array name as the label and its member set expanded (WS1-034). bus.edn declares the bus but joins
// no member nets, so the members are named but unresolved — the bus-not-modeled rule fires there.
func TestEDIFArrayBusDetected(t *testing.T) {
	d, err := Read(bytes.NewReader(readBusFixture(t, "bus.edn")), "bus.edn")
	if err != nil {
		t.Fatal(err)
	}
	bs := d.GetInputDiagnostics().GetUnmodeledBuses()
	if len(bs) != 1 {
		t.Fatalf("unmodeled buses = %d, want 1", len(bs))
	}
	if bs[0].GetKind() != "edif_array" || bs[0].GetLabel() != "DATA" {
		t.Errorf("bus = {kind:%q label:%q}, want edif_array / DATA", bs[0].GetKind(), bs[0].GetLabel())
	}
	if got := bs[0].GetMembers(); !eq(got, wantMembers) {
		t.Errorf("members = %v, want %v", got, wantMembers)
	}
}

// TestEDIFArrayBusResolved: bus_resolved.edn joins each of the 8 array members into a per-member net
// named DATA[i], so the reader produces a design where every expanded member is a net. Combined with the
// resolution gate (rule_bus_not_modeled_test.go), the bus-not-modeled finding is silent there — the
// resolution-aware behavior PR 286 gave KiCad, now for EDIF.
func TestEDIFArrayBusResolved(t *testing.T) {
	d, err := Read(bytes.NewReader(readBusFixture(t, "bus_resolved.edn")), "bus_resolved.edn")
	if err != nil {
		t.Fatal(err)
	}
	bs := d.GetInputDiagnostics().GetUnmodeledBuses()
	if len(bs) != 1 {
		t.Fatalf("unmodeled buses = %d, want 1", len(bs))
	}
	if got := bs[0].GetMembers(); !eq(got, wantMembers) {
		t.Fatalf("members = %v, want %v", got, wantMembers)
	}
	nets := map[string]bool{}
	var have []string
	for _, n := range d.GetNets() {
		nets[n.GetName()] = true
		have = append(have, n.GetName())
	}
	for _, m := range wantMembers {
		if !nets[m] {
			t.Errorf("member %q is not a net; the resolved fixture should form every member net (have %v)", m, have)
		}
	}
}

func readBusFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func eq(a, b []string) bool {
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

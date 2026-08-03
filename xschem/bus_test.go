package xschem

import (
	"bytes"
	"testing"
)

// TestXschemBusDetected: a `lab=DATA[7:0]` bus-range label is recorded as an unmodeled-bus diagnostic
// (WS1-034 Phase 1). A scalar indexed net (no colon) would not match the bus-range pattern.
func TestXschemBusDetected(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "bus.sch")), "bus.sch")
	if err != nil {
		t.Fatal(err)
	}
	bs := d.GetInputDiagnostics().GetUnmodeledBuses()
	if len(bs) != 1 {
		t.Fatalf("unmodeled buses = %d, want 1", len(bs))
	}
	if bs[0].GetKind() != "xschem_bus_label" || bs[0].GetLabel() != "DATA[7:0]" {
		t.Errorf("bus = {kind:%q label:%q}, want xschem_bus_label / DATA[7:0]", bs[0].GetKind(), bs[0].GetLabel())
	}
}

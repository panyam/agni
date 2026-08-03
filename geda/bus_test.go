package geda

import (
	"bytes"
	"slices"
	"testing"
)

// TestGedaBusDetected: a lone gEDA `U` bus is recorded as a resolution-aware unmodeled-bus diagnostic
// with its netname as the label and the range-expanded member set (WS1-034). Its members are unmodeled
// (no member nets exist), so the bus-not-modeled rule fires — and, per lepton-netlist, the bus is
// graphical-only, so it must NOT alias to a phantom net (the pre-Phase-2 bug).
func TestGedaBusDetected(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "bus.sch")), "bus.sch")
	if err != nil {
		t.Fatal(err)
	}
	bs := d.GetInputDiagnostics().GetUnmodeledBuses()
	if len(bs) != 1 {
		t.Fatalf("unmodeled buses = %d, want 1", len(bs))
	}
	if bs[0].GetKind() != "geda_bus" || bs[0].GetLabel() != "DATA[7:0]" {
		t.Errorf("bus = {kind:%q label:%q}, want geda_bus / DATA[7:0]", bs[0].GetKind(), bs[0].GetLabel())
	}
	wantMembers := []string{"DATA7", "DATA6", "DATA5", "DATA4", "DATA3", "DATA2", "DATA1", "DATA0"}
	if got := bs[0].GetMembers(); !slices.Equal(got, wantMembers) {
		t.Errorf("members = %v, want %v", got, wantMembers)
	}
	// The bus is graphical-only (lepton-netlist): it must not become a net.
	for _, n := range d.GetNets() {
		if n.GetName() == "DATA[7:0]" {
			t.Errorf("bus DATA[7:0] must not alias to a net (lepton-netlist emits no such net)")
		}
	}
}

// TestGedaBusResolved: bus_resolved.sch ripples DATA[1:0]'s two members off to per-member `netname=`
// nets DATA0/DATA1. Cross-checked against lepton-netlist, which emits exactly DATA0, DATA1, OUT (and no
// DATA[1:0]): the members resolve, so the reader forms them as nets and does NOT emit a bus net —
// combined with the resolution gate (rule_bus_not_modeled_test.go) the finding is silent here.
func TestGedaBusResolved(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "bus_resolved.sch")), "bus_resolved.sch")
	if err != nil {
		t.Fatal(err)
	}
	bs := d.GetInputDiagnostics().GetUnmodeledBuses()
	if len(bs) != 1 {
		t.Fatalf("unmodeled buses = %d, want 1", len(bs))
	}
	if got := bs[0].GetMembers(); !slices.Equal(got, []string{"DATA1", "DATA0"}) {
		t.Fatalf("members = %v, want [DATA1 DATA0]", got)
	}
	var names []string
	for _, n := range d.GetNets() {
		names = append(names, n.GetName())
	}
	for _, want := range []string{"DATA0", "DATA1"} {
		if !slices.Contains(names, want) {
			t.Errorf("member %q should be a net (have %v)", want, names)
		}
	}
	if slices.Contains(names, "DATA[1:0]") {
		t.Errorf("bus DATA[1:0] must not alias to a net (lepton-netlist emits no such net)")
	}
}

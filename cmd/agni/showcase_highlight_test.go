package main

import (
	"testing"

	"github.com/panyam/agni/readers/formats"
)

// TestShowcaseFaithfulWireNets guards that the showcase board's wires carry solved net names on
// the FAITHFUL geometry (WS1-022), so a net-subject finding highlights on the real schematic
// without switching to an auto-layout. Net resolution keys wires by uuid, so this fails if the
// hand-authored fixture drops its wire uuids. Power rails +3V3/GND are tapped at power-symbol
// pins with no drawn wire, so they carry no wire net and are intentionally not asserted.
func TestShowcaseFaithfulWireNets(t *testing.T) {
	l := &formats.Loader{}
	g, err := l.FaithfulGeometry("testdata/conformance/showcase.fires.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, sh := range g.GetSheets() {
		for _, w := range sh.GetWires() {
			if w.GetNet() != "" {
				got[w.GetNet()] = true
			}
		}
	}
	for _, net := range []string{"SCL", "USB_D+", "USB_D-", "VBUS"} {
		if !got[net] {
			t.Errorf("wire net %q missing on faithful geometry; a net-subject highlight would not paint (are the wire uuids present?)", net)
		}
	}
}

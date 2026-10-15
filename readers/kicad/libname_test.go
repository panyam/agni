package kicad

import (
	"bytes"
	"testing"
)

// KiCad files a placed symbol's definition under lib_name, not lib_id, when the embedded copy
// differs from the library's, so lib_id "power:GND" can sit beside a lib_symbols entry named
// "GND_3". Keyed on lib_id alone, every such placement resolved nothing: its pins and its net
// anchor were lost from the netlist and the drawing alike.
func TestPlacedSymbolResolvesThroughLibName(t *testing.T) {
	data := readFixture(t, "libname.kicad_sch")
	d, err := ReadSchematic(bytes.NewReader(data), "libname.kicad_sch")
	if err != nil {
		t.Fatalf("read netlist: %v", err)
	}
	at := map[string]string{}
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			at[c.GetComponentRef()+"."+c.GetPinRef()] = n.GetName()
		}
	}
	if got := at["R1.1"]; got != "/SIG" {
		t.Errorf("R1.1 is on %q, want /SIG", got)
	}
	if got := at["R1.2"]; got != "GND" {
		t.Errorf("R1.2 is on %q, want GND, which the power symbol filed under lib_name names", got)
	}

	g, err := ReadSchematicGeometry(bytes.NewReader(data), "libname.kicad_sch")
	if err != nil {
		t.Fatalf("read geometry: %v", err)
	}
	defined := map[string]bool{}
	for _, sd := range g.GetSymbols() {
		defined[sd.GetCellRef()] = true
	}
	placed := 0
	for _, sh := range g.GetSheets() {
		for _, pl := range sh.GetPlacements() {
			placed++
			if !defined[pl.GetCellRef()] {
				t.Errorf("placement %q names cell %q, which no symbol definition carries", pl.GetRefDes(), pl.GetCellRef())
			}
		}
	}
	if placed != 2 {
		t.Fatalf("read %d placements, want 2", placed)
	}
}

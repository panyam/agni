package common

import (
	"sort"
	"testing"
)

// The i2c-sensor fixture is one design in two files, and they are hand-authored separately. The
// netlist is what every analysis reads and the schematic is what a finding gets drawn on, and they
// are joined BY NAME: a highlight for net SDA lands on the wire the .eds calls SDA and nowhere else.
// Nothing in the engine holds the two together, so a ref-des renamed in one and not the other
// produces a drawing that is silently pointing at the wrong part, which is exactly the failure a
// reader trusts the picture not to have.

func TestTheDrawnSheetCoversEveryPartInTheNetlist(t *testing.T) {
	d, err := ReadFixture("i2c-sensor.edn")
	if err != nil {
		t.Fatalf("netlist: %v", err)
	}
	g, err := ReadSchematicFixture("i2c-sensor.eds")
	if err != nil {
		t.Fatalf("schematic: %v", err)
	}
	drawn := map[string]bool{}
	for _, s := range g.GetSheets() {
		for _, p := range s.GetPlacements() {
			drawn[p.GetRefDes()] = true
		}
	}
	var missing []string
	for _, c := range d.GetComponents() {
		if !drawn[c.GetRefDes()] {
			missing = append(missing, c.GetRefDes())
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("in the netlist and not on the sheet: %v", missing)
	}
	if len(drawn) != len(d.GetComponents()) {
		t.Errorf("sheet draws %d parts, netlist has %d; a part drawn that is not in the netlist "+
			"points somewhere the analysis never looked", len(drawn), len(d.GetComponents()))
	}
}

// A wire's name is the join key a finding's highlight resolves through, so a net the netlist has and
// the sheet spells differently is a highlight that silently lands nowhere.
func TestEveryNetlistNetIsDrawnUnderItsOwnName(t *testing.T) {
	d, err := ReadFixture("i2c-sensor.edn")
	if err != nil {
		t.Fatalf("netlist: %v", err)
	}
	g, err := ReadSchematicFixture("i2c-sensor.eds")
	if err != nil {
		t.Fatalf("schematic: %v", err)
	}
	drawn := map[string]bool{}
	for _, s := range g.GetSheets() {
		for _, w := range s.GetWires() {
			drawn[w.GetNet()] = true
		}
	}
	var missing []string
	for _, n := range d.GetNets() {
		if !drawn[n.GetName()] {
			missing = append(missing, n.GetName())
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("nets with no wire of that name on the sheet: %v", missing)
	}
}

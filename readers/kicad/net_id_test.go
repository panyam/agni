package kicad

import (
	"bytes"
	"testing"
)

// TestNetIDAgreesAcrossNetlistAndGeometry pins the load-bearing WS9 invariant: the netlist read
// stamps ir.Net.id and the geometry read stamps WireGeometry.net_id, from two INDEPENDENT
// netgraph.Build solves over the same inputs. Because the id is a pure function of connectivity, a
// wire's net_id must equal the ir.Net.id of the net it belongs to — otherwise a per-instance
// highlight would join to the wrong net (or nothing). This is the join no unit test of either side
// alone can prove.
func TestNetIDAgreesAcrossNetlistAndGeometry(t *testing.T) {
	data := readFixture(t, "wirenet.kicad_sch")
	d, err := ReadSchematic(bytes.NewReader(data), "wirenet.kicad_sch")
	if err != nil {
		t.Fatalf("read netlist: %v", err)
	}
	g, err := ReadSchematicGeometry(bytes.NewReader(data), "wirenet.kicad_sch")
	if err != nil {
		t.Fatalf("read geometry: %v", err)
	}

	idByName := map[string]string{}
	for _, n := range d.GetNets() {
		idByName[n.GetName()] = n.GetId()
	}
	checked := 0
	for _, sh := range g.GetSheets() {
		for _, w := range sh.GetWires() {
			name := w.GetNet()
			if name == "" {
				continue
			}
			id, ok := idByName[name]
			if !ok || id == "" {
				continue // a pinless (label-only) net carries no id on either side; nothing to join
			}
			if w.GetNetId() != id {
				t.Errorf("wire on net %q: geometry net_id %q != netlist ir.Net.id %q", name, w.GetNetId(), id)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no wire joined a pinned net; the fixture cannot exercise the invariant")
	}
}

package kicad

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestNetIDAgreesAcrossNetlistAndGeometry pins a WS9 invariant. The netlist read
// stamps ir.Net.id and the geometry read stamps WireGeometry.net_id, from two INDEPENDENT
// netgraph.Build solves over the same inputs. Because the id is a pure function of connectivity, a
// wire's net_id must equal the ir.Net.id of the net it belongs to, otherwise a per-instance
// highlight would join to the wrong net (or nothing). No unit test of either side alone can prove
// this join.
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

// TestEveryNamedWireCarriesItsNetsID holds every schematic fixture to the case the page's name
// fallback exists for (agni issue 1458). The viewer lights a wire with no net_id by its name
// (HighlightSpec.netIdNames), which is right for a name-only drawing and wrong on a KiCad sheet,
// where a stray id-less wire would light under any net sharing its name. A KiCad wire may go
// without an id only when its net is pinless, which the netlist leaves out or gives no id.
func TestEveryNamedWireCarriesItsNetsID(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.kicad_sch"))
	if err != nil || len(files) < 10 {
		t.Fatalf("expected the schematic fixtures, got %d (%v)", len(files), err)
	}
	checked, inSubSheets := 0, 0
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		// The hierarchy reads take a sub-sheet's ids through its instance, which the single-file
		// reads never do, and a file with no sub-sheets reads as its root alone.
		name := filepath.Base(path)
		open := func(rel string) ([]byte, error) { return os.ReadFile(filepath.Join("testdata", rel)) }
		d, _, err := ReadSchematicHierarchyNets(name, data, open)
		if err != nil {
			continue // a fixture that exists to be refused has nothing to join
		}
		g, err := ReadSchematicHierarchy(name, data, open)
		if err != nil {
			t.Fatalf("%s: geometry: %v", name, err)
		}
		pinned := map[string]bool{}
		for _, n := range d.GetNets() {
			if n.GetId() != "" {
				pinned[n.GetName()] = true
			}
		}
		for i, sh := range g.GetSheets() {
			for _, w := range sh.GetWires() {
				if isBus(w) || w.GetNet() == "" {
					continue
				}
				if !pinned[w.GetNet()] {
					continue
				}
				checked++
				if i > 0 {
					inSubSheets++
				}
				if w.GetNetId() == "" {
					t.Errorf("%s: a wire on net %q has no net_id, and the netlist gives that net one", name, w.GetNet())
				}
			}
		}
	}
	if checked == 0 || inSubSheets == 0 {
		t.Fatalf("checked %d wires on pinned nets, %d of them on a sub-sheet; the fixtures cannot exercise the invariant", checked, inSubSheets)
	}
	t.Logf("%d wires on pinned nets over %d fixtures, %d on a sub-sheet", checked, len(files), inSubSheets)
}

func isBus(w *geom.WireGeometry) bool {
	k := w.GetKind()
	return k == geom.WireGeometry_KIND_BUS || k == geom.WireGeometry_KIND_BUS_ENTRY
}

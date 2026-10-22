package kicad

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// flaggedPins is each pin, "REF.PIN", on a net marked no_connect, once each. A board can carry one
// pad number several times (Jetson's U30 has seven thermal-via pads numbered 57, each on its own
// unconnected net), so pins are counted once.
func flaggedPins(d *ir.Design) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range d.GetNets() {
		if n.GetAttributes()[netgraph.AttrNoConnect] != "true" {
			continue
		}
		for _, c := range n.GetConnections() {
			p := c.GetComponentRef() + "." + c.GetPinRef()
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// agni issue 1023: KiCad writes a schematic pin's no-connect flag onto its pad's pin type
// ("passive+no_connect"), so Jetson read from its board alone flags the same pins its schematic
// read flags. Before, the board read flagged none.
func TestABoardReadFlagsThePinsItsSchematicLeavesOpen(t *testing.T) {
	dir := filepath.Join("..", "..", "tools", "samples", "boards", "jetson-agx-thor-baseboard")
	pcb, err := os.Open(filepath.Join(dir, "jetson-agx-thor-baseboard.kicad_pcb"))
	if err != nil {
		t.Fatalf("the samples corpus is missing (make samples): %v", err)
	}
	defer pcb.Close()
	board, err := Read(pcb, "jetson-agx-thor-baseboard.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.ReadFile(filepath.Join(dir, "jetson-agx-thor-baseboard.kicad_sch"))
	if err != nil {
		t.Fatal(err)
	}
	sch, _, err := ReadSchematicHierarchyNets("jetson-agx-thor-baseboard.kicad_sch", root, func(rel string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	})
	if err != nil {
		t.Fatal(err)
	}
	fromBoard, fromSchematic := flaggedPins(board), flaggedPins(sch)
	if len(fromBoard) < 100 {
		t.Fatalf("the board read flags %d pins; Jetson's board marks hundreds of pads no_connect", len(fromBoard))
	}
	if strings.Join(fromBoard, " ") != strings.Join(fromSchematic, " ") {
		t.Errorf("the board read flags %d pins and the schematic read %d; they should be the same pins", len(fromBoard), len(fromSchematic))
	}
	onSchematic := map[string]bool{}
	for _, p := range fromSchematic {
		onSchematic[p] = true
	}
	for _, p := range fromBoard {
		if !onSchematic[p] {
			t.Errorf("the board read flags %s and the schematic read does not", p)
		}
	}
	t.Logf("board flags %d pins, schematic %d", len(fromBoard), len(fromSchematic))
}

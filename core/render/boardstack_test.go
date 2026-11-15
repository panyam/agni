package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/kicad"
)

func jetsonCutBoard(t *testing.T) *geom.BoardGeometry {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "examples", "cross-probe-project", "designs", "jetson-cut", "jetson-cut.kicad_pcb"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, err := kicad.ReadBoardGeometry(f, "jetson-cut.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestALayerRowStatesItsThicknessFromTheStack(t *testing.T) {
	rows := BoardLayers(jetsonCutBoard(t))
	for name, want := range map[string]string{"F.Cu": "0.035 mm", "In2.Cu": "0.018 mm", "In7.Cu": "0.018 mm", "B.Cu": "0.035 mm", "Edge": "", "F.SilkS": "thickness unknown"} {
		if got := boardRow(t, rows, name).GetThickness(); got != want {
			t.Errorf("%s thickness = %q, want %q", name, got, want)
		}
	}
}

func TestALayerTheStackGivesNoThicknessReadsAsUnknown(t *testing.T) {
	b := jetsonCutBoard(t)
	for _, l := range b.GetStackup().GetLayers() {
		if l.GetLayer() == "In5.Cu" {
			l.Thickness = nil
		}
	}
	rows := BoardLayers(b)
	if got := boardRow(t, rows, "In5.Cu").GetThickness(); got != "thickness unknown" {
		t.Errorf("In5.Cu thickness = %q, want %q", got, "thickness unknown")
	}
	if got := boardRow(t, rows, "In2.Cu").GetThickness(); got != "0.018 mm" {
		t.Errorf("In2.Cu thickness = %q, want 0.018 mm", got)
	}
}

func TestBoardStackListsTheDielectricsAndTheTotal(t *testing.T) {
	v := BoardStack(jetsonCutBoard(t))
	if got := len(v.GetRows()); got != 28 {
		t.Fatalf("%d stack rows, want the 28 agni stackup prints", got)
	}
	first, cu, d1 := v.GetRows()[0], v.GetRows()[3], v.GetRows()[4]
	if first.GetName() != "F.SilkS" || first.GetThickness() != "thickness unknown" || first.ThicknessMm != nil {
		t.Errorf("first row = %v, want F.SilkS with thickness unknown", first)
	}
	if cu.GetName() != "F.Cu" || cu.GetThickness() != "0.035 mm" || cu.GetThicknessMm() != 0.035 {
		t.Errorf("fourth row = %v, want F.Cu at 0.035 mm", cu)
	}
	if d1.GetName() != "dielectric 1" || d1.GetKind() != "dielectric" || d1.GetThickness() != "0.12 mm" || d1.GetMaterial() != "PR2116" {
		t.Errorf("fifth row = %v, want dielectric 1, 0.12 mm of PR2116", d1)
	}
	if v.GetSummary() != "1.711 mm, as the file states" || v.GetDisagrees() {
		t.Errorf("summary = %q (disagrees %v), want the stated 1.711 mm", v.GetSummary(), v.GetDisagrees())
	}
}

func TestBoardStackSaysWhenTheLayersMissTheStatedThickness(t *testing.T) {
	b := jetsonCutBoard(t)
	stated := int64(2_000_000)
	b.Stackup.Thickness = &stated
	v := BoardStack(b)
	if !v.GetDisagrees() || !strings.Contains(v.GetSummary(), "the layers sum to 1.711 mm, and the file states a board thickness of 2 mm") {
		t.Errorf("summary = %q (disagrees %v), want Check's sentence", v.GetSummary(), v.GetDisagrees())
	}
}

func TestABoardWithNoStackHasNoStackView(t *testing.T) {
	b := innerBoard(t)
	b.Stackup = nil
	if v := BoardStack(b); v != nil {
		t.Errorf("BoardStack = %v, want nil", v)
	}
	for _, r := range BoardLayers(b) {
		if r.GetThickness() != "" {
			t.Errorf("%s thickness = %q on a board with no stack", r.GetName(), r.GetThickness())
		}
	}
}

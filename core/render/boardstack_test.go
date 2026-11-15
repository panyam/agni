package render

import (
	"math"
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

func blindViaBoard(t *testing.T) *geom.BoardGeometry {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "blind-via.kicad_pcb"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, err := kicad.ReadBoardGeometry(f, "blind-via.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEachDrillSpanJoinsOnlyTheLayersItsHolesReach(t *testing.T) {
	v := BoardStack(blindViaBoard(t))
	got := map[string][]string{}
	for _, sp := range v.GetDrills() {
		if len(sp.GetXy()) != 2*len(sp.GetDiameter()) || len(sp.GetNet()) != len(sp.GetDiameter()) {
			t.Fatalf("span %s..%s has %d coordinates, %d diameters and %d nets", sp.GetLayerFrom(), sp.GetLayerTo(), len(sp.GetXy()), len(sp.GetDiameter()), len(sp.GetNet()))
		}
		got[sp.GetLayerFrom()+".."+sp.GetLayerTo()] = sp.GetNet()
	}
	want := map[string][]string{"F.Cu..B.Cu": {"THRU", "THRU"}, "F.Cu..In1.Cu": {"BLIND"}, "In1.Cu..In2.Cu": {"BURIED"}}
	if len(got) != len(want) {
		t.Fatalf("spans = %v, want %v", got, want)
	}
	for span, nets := range want {
		if strings.Join(got[span], ",") != strings.Join(nets, ",") {
			t.Errorf("span %s holds %v, want %v", span, got[span], nets)
		}
	}
	for _, sp := range v.GetDrills() {
		if sp.GetLayerFrom() == "F.Cu" && sp.GetLayerTo() == "In1.Cu" && (sp.GetXy()[0] != 8e6 || sp.GetXy()[1] != -3e6 || sp.GetDiameter()[0] != 0.2e6) {
			t.Errorf("blind via at %v width %v, want (8 mm, -3 mm) in geom's Y-up frame, and 0.2 mm", sp.GetXy(), sp.GetDiameter())
		}
	}
}

func TestTheJetsonCutsViasAllRunThroughTheBoard(t *testing.T) {
	v := BoardStack(jetsonCutBoard(t))
	if len(v.GetDrills()) != 1 {
		t.Fatalf("%d drill spans, want one through span", len(v.GetDrills()))
	}
	sp := v.GetDrills()[0]
	if sp.GetLayerFrom() != "F.Cu" || sp.GetLayerTo() != "B.Cu" || len(sp.GetDiameter()) < 100 {
		t.Errorf("span %s..%s with %d holes, want F.Cu..B.Cu with the cut's vias", sp.GetLayerFrom(), sp.GetLayerTo(), len(sp.GetDiameter()))
	}
}

func TestTheOutlineFillsTheBoardFromItsSeparateEdges(t *testing.T) {
	tris := BoardStack(blindViaBoard(t)).GetOutlineTris()
	if len(tris) == 0 || len(tris)%6 != 0 {
		t.Fatalf("%d outline values, want whole triangles", len(tris))
	}
	area := 0.0
	for i := 0; i < len(tris); i += 6 {
		area += math.Abs((tris[i+2]-tris[i])*(tris[i+5]-tris[i+1])-(tris[i+4]-tris[i])*(tris[i+3]-tris[i+1])) / 2
	}
	if want := 20e6 * 10e6; math.Abs(area-want) > want*1e-9 {
		t.Errorf("outline triangles cover %g, want the 20 mm by 10 mm board, %g", area, want)
	}
}

func TestAnOutlineThatDoesNotCloseFillsItsBox(t *testing.T) {
	b := blindViaBoard(t)
	b.Outline.Paths = b.Outline.Paths[:3]
	tris := BoardStack(b).GetOutlineTris()
	area := 0.0
	for i := 0; i+5 < len(tris); i += 6 {
		area += math.Abs((tris[i+2]-tris[i])*(tris[i+5]-tris[i+1])-(tris[i+4]-tris[i])*(tris[i+3]-tris[i+1])) / 2
	}
	if want := 20e6 * 10e6; math.Abs(area-want) > want*1e-9 {
		t.Errorf("open outline fills %g, want its 20 mm by 10 mm box, %g", area, want)
	}
}

func TestStackRowsCarryTheirStyleColoursAndTheBoardUnit(t *testing.T) {
	v := BoardStack(blindViaBoard(t), WithStyle(DarkStyle))
	if v.GetUnitsPerMm() != 1e6 {
		t.Errorf("units per mm = %g, want KiCad's 1e6 nanometres", v.GetUnitsPerMm())
	}
	want := map[string]string{"F.Mask": DarkStyle.SolderMask, "F.Cu": DarkStyle.CopperFront, "dielectric 2": DarkStyle.Dielectric, "B.Cu": DarkStyle.CopperBack}
	seen := 0
	for _, r := range v.GetRows() {
		if c, ok := want[r.GetName()]; ok {
			seen++
			if r.GetColor() != c {
				t.Errorf("%s colour = %q, want %q", r.GetName(), r.GetColor(), c)
			}
		}
	}
	if seen != len(want) {
		t.Errorf("found %d of the %d rows to check", seen, len(want))
	}
}

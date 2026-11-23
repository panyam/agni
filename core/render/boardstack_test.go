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
	f, err := os.Open(filepath.Join("..", "..", "examples", "cross-probe-project", "boards", "jetson-cut", "jetson-cut.kicad_pcb"))
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
	if want := 20e6*10e6 - j1Hole; math.Abs(area-want) > want*1e-9 {
		t.Errorf("outline triangles cover %g, want the 20 mm by 10 mm board less J1's hole, %g", area, want)
	}
}

// j1Hole is the area cut for J1's 1 mm drill on the blind-via board, a 16-sided ring of radius
// 0.5 mm (agni 1607).
var j1Hole = 8 * 0.5e6 * 0.5e6 * math.Sin(math.Pi/8)

func TestAnOutlineThatDoesNotCloseFillsItsBox(t *testing.T) {
	b := blindViaBoard(t)
	b.Outline.Paths = b.Outline.Paths[:3]
	tris := BoardStack(b).GetOutlineTris()
	area := 0.0
	for i := 0; i+5 < len(tris); i += 6 {
		area += math.Abs((tris[i+2]-tris[i])*(tris[i+5]-tris[i+1])-(tris[i+4]-tris[i])*(tris[i+3]-tris[i+1])) / 2
	}
	if want := 20e6*10e6 - j1Hole; math.Abs(area-want) > want*1e-9 {
		t.Errorf("open outline fills %g, want its 20 mm by 10 mm box less J1's hole, %g", area, want)
	}
}

// covered says whether a point lies inside one of the triangles of a flat x,y triangle list.
func covered(tris []float64, x, y float64) bool {
	for i := 0; i+5 < len(tris); i += 6 {
		ax, ay, bx, by, cx, cy := tris[i], tris[i+1], tris[i+2], tris[i+3], tris[i+4], tris[i+5]
		d1 := (x-bx)*(ay-by) - (ax-bx)*(y-by)
		d2 := (x-cx)*(by-cy) - (bx-cx)*(y-cy)
		d3 := (x-ax)*(cy-ay) - (cx-ax)*(y-ay)
		neg := d1 < 0 || d2 < 0 || d3 < 0
		pos := d1 > 0 || d2 > 0 || d3 > 0
		if !(neg && pos) {
			return true
		}
	}
	return false
}

// The slabs leave out a cutout in the outline and a drill of 1 mm or more, and keep a via's (agni
// 1607). The KiCad reader turns the file's y down, so the blind-via board runs from y 0 to -10 mm. It
// gains a 2 by 1 mm slot at (10, -7). J1 has a 1 mm drill at (16, -5), and the through via at (4, -3)
// a 0.3 mm one.
func TestTheSlabsLeaveTheBoardsHolesOut(t *testing.T) {
	b := blindViaBoard(t)
	slot := &geom.Polyline{Points: []*geom.Point{{X: 9e6, Y: -6.5e6}, {X: 11e6, Y: -6.5e6}, {X: 11e6, Y: -7.5e6}, {X: 9e6, Y: -7.5e6}, {X: 9e6, Y: -6.5e6}}}
	b.Outline.Paths = append(b.Outline.Paths, slot)
	tris := BoardStack(b).GetOutlineTris()
	for _, c := range []struct {
		what string
		x, y float64
		want bool
	}{
		{"the slot", 10e6, -7e6, false},
		{"J1's hole", 16e6, -5e6, false},
		{"the through via", 4e6, -3e6, true},
		{"the board beside the slot", 10e6, -8.5e6, true},
	} {
		if got := covered(tris, c.x, c.y); got != c.want {
			t.Errorf("%s covered = %v, want %v", c.what, got, c.want)
		}
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

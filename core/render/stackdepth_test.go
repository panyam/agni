package render

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// mmTolerance is a hundredth of a micrometre. The depths are sums in floating point.
const mmTolerance = 1e-9

func stackRow(t *testing.T, v *geom.BoardStackView, name string) *geom.BoardStackRow {
	t.Helper()
	for _, r := range v.GetRows() {
		if r.GetName() == name {
			return r
		}
	}
	t.Fatalf("the stack has no row %q", name)
	return nil
}

// The Jetson cut's 28 rows stack from its top face down to its stated 1.711 mm (agni issue 1635).
// KiCad gives silk and paste no thickness, so they sit on the surfaces.
func TestTheCutsStackPlacesEveryRowInDepth(t *testing.T) {
	v := BoardStack(jetsonCutBoard(t))
	if len(v.GetRows()) != 28 {
		t.Fatalf("the cut's stack has %d rows, want 28", len(v.GetRows()))
	}
	if math.Abs(v.GetTotalMm()-1.711) > 1e-5 {
		t.Errorf("total %v mm, want 1.711", v.GetTotalMm())
	}
	silk := stackRow(t, v, "F.SilkS")
	if silk.GetTopMm() != 0 || silk.GetBottomMm() != 0 || !silk.GetThicknessDefaulted() {
		t.Errorf("F.SilkS sits at %v to %v, defaulted %v, want 0 to 0, defaulted", silk.GetTopMm(), silk.GetBottomMm(), silk.GetThicknessDefaulted())
	}
	cu := stackRow(t, v, "F.Cu")
	if cu.GetThicknessDefaulted() || math.Abs(cu.GetBottomMm()-cu.GetTopMm()-0.035) > mmTolerance {
		t.Errorf("F.Cu is %v mm thick, defaulted %v, want the file's 0.035", cu.GetBottomMm()-cu.GetTopMm(), cu.GetThicknessDefaulted())
	}
	if back := stackRow(t, v, "B.SilkS"); math.Abs(back.GetTopMm()-v.GetTotalMm()) > mmTolerance {
		t.Errorf("B.SilkS sits at %v, want the bottom face, %v", back.GetTopMm(), v.GetTotalMm())
	}
	prev := 0.0
	for _, r := range v.GetRows() {
		if math.Abs(r.GetTopMm()-prev) > mmTolerance {
			t.Errorf("%s starts at %v, want where the row above ends, %v", r.GetName(), r.GetTopMm(), prev)
		}
		prev = r.GetBottomMm()
	}
}

// A row the file gives no thickness draws at its kind's typical thickness, and says so.
func TestAStackRowWithNoThicknessDrawsAtItsKindsTypical(t *testing.T) {
	b := jetsonCutBoard(t)
	for _, l := range b.GetStackup().GetLayers() {
		if l.GetName() == "In1.Cu" {
			l.Thickness = nil
		}
	}
	v := BoardStack(b)
	in1 := stackRow(t, v, "In1.Cu")
	if !in1.GetThicknessDefaulted() || math.Abs(in1.GetBottomMm()-in1.GetTopMm()-TypicalMm["copper"]) > mmTolerance {
		t.Errorf("In1.Cu draws %v mm thick, defaulted %v, want copper's typical %v, defaulted", in1.GetBottomMm()-in1.GetTopMm(), in1.GetThicknessDefaulted(), TypicalMm["copper"])
	}
	if want := 1.711 - 0.018 + TypicalMm["copper"]; math.Abs(v.GetTotalMm()-want) > 1e-5 {
		t.Errorf("total %v mm, want %v", v.GetTotalMm(), want)
	}
}

// A blind via's holes run from the top of its first layer's row to the bottom of its last's, and a
// span the stack cannot place has no depths.
func TestADrillSpanRunsBetweenItsLayersRows(t *testing.T) {
	v := BoardStack(jetsonCutBoard(t))
	blind := &geom.BoardDrillSpan{LayerFrom: "F.Cu", LayerTo: "In1.Cu"}
	lost := &geom.BoardDrillSpan{LayerFrom: "F.Cu", LayerTo: "Nowhere"}
	v.Drills = []*geom.BoardDrillSpan{blind, lost}
	spanDepths(v)
	if math.Abs(blind.GetTopMm()-0.01) > mmTolerance || math.Abs(blind.GetBottomMm()-(0.01+0.035+0.12+0.018)) > mmTolerance {
		t.Errorf("F.Cu to In1.Cu runs %v to %v, want 0.01 to 0.183", blind.GetTopMm(), blind.GetBottomMm())
	}
	if lost.TopMm != nil || lost.BottomMm != nil {
		t.Errorf("a span to a layer the stack lacks runs %v to %v, want no depths", lost.TopMm, lost.BottomMm)
	}
	for _, sp := range BoardStack(jetsonCutBoard(t)).GetDrills() {
		if sp.TopMm == nil || sp.GetBottomMm() <= sp.GetTopMm() {
			t.Errorf("the cut's %s to %s span runs %v to %v, want depths it can draw", sp.GetLayerFrom(), sp.GetLayerTo(), sp.TopMm, sp.BottomMm)
		}
	}
}

// A part whose file states no height draws at TypicalBodyMm and says so. One that states a height
// draws at it.
func TestAPartBodyDrawsAtItsStatedOrTypicalHeight(t *testing.T) {
	b := jetsonCutBoard(t)
	stated := int64(2_500_000 / b.GetUnitNm())
	b.Placements[0].Height = &stated
	b.Placements[1].Height = nil
	bodies := map[string]*geom.BoardPartBody{}
	for _, body := range partBodies(b, resolveStyle(nil)) {
		bodies[body.GetRefDes()] = body
	}
	if got := bodies[b.Placements[0].GetRefDes()]; got == nil || got.GetDrawnHeightMm() != 2.5 || got.GetHeightDefaulted() {
		t.Errorf("a part stating 2.5 mm draws as %v", got)
	}
	if got := bodies[b.Placements[1].GetRefDes()]; got == nil || got.GetDrawnHeightMm() != TypicalBodyMm || !got.GetHeightDefaulted() {
		t.Errorf("a part stating no height draws as %v, want %v mm, defaulted", got, TypicalBodyMm)
	}
}

// TestBoardStackTwin writes the cut's stack rows, with their depths, to the twin fixture that the
// viewer's stack tests read (C27), so the viewer tests draw at the depths the engine serves.
// `go test ./core/render -run TestBoardStackTwin -update` rewrites it.
func TestBoardStackTwin(t *testing.T) {
	v := BoardStack(jetsonCutBoard(t))
	twin := &geom.BoardStackView{Rows: v.GetRows(), TotalMm: v.GetTotalMm(), UnitsPerMm: v.GetUnitsPerMm()}
	raw, err := protojson.Marshal(twin)
	if err != nil {
		t.Fatal(err)
	}
	// protojson varies its whitespace from build to build on purpose, so the file is indented by
	// encoding/json, whose output is stable, and the test can compare bytes.
	var compact, indented bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Indent(&indented, compact.Bytes(), "", "  "); err != nil {
		t.Fatal(err)
	}
	body := append(indented.Bytes(), '\n')
	path := filepath.Join("testdata", "jetson_cut_stack_twin.json")
	if *update {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	have, err := os.ReadFile(path) // #nosec G304 -- the package's own testdata
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	got := &geom.BoardStackView{}
	if err := protojson.Unmarshal(have, got); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(got, twin) || !bytes.Equal(have, body) {
		t.Errorf("%s is not the cut's stack; run `go test ./core/render -run TestBoardStackTwin -update`", path)
	}
}

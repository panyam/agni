package render

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// shapeBoard is a two-layer board with a 20 mm square GND pour on In1.Cu that has a 4 mm square hole,
// and a track on F.Cu.
func shapeBoard() *geom.BoardGeometry {
	sq := func(x0, y0, x1, y1 int64) *geom.Polyline {
		return &geom.Polyline{Points: []*geom.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}}
	}
	return &geom.BoardGeometry{
		UnitNm:  1,
		Layers:  []*geom.BoardLayer{{Number: 0, Name: "F.Cu", Kind: "signal"}, {Number: 1, Name: "In1.Cu", Kind: "power"}, {Number: 2, Name: "B.Cu", Kind: "signal"}},
		Outline: &geom.BoardOutline{Paths: []*geom.Polyline{sq(0, 0, 30_000_000, 30_000_000)}},
		Nets: []*geom.NetCopper{
			{Net: "GND", Shapes: []*geom.CopperShape{{Layer: "In1.Cu", Outline: sq(5_000_000, 5_000_000, 25_000_000, 25_000_000), Holes: []*geom.Polyline{sq(13_000_000, 13_000_000, 17_000_000, 17_000_000)}}}},
			{Net: "SIG", Segments: []*geom.TrackSegment{{A: &geom.Point{X: 1_000_000, Y: 2_000_000}, B: &geom.Point{X: 29_000_000, Y: 2_000_000}, Width: 250_000, Layer: "F.Cu"}}},
		},
	}
}

func TestACopperShapeDrawsAsOneEvenOddPathOnItsLayer(t *testing.T) {
	doc := BoardSVG(shapeBoard(), WithPickTargets())
	g := layerGroup(t, doc, "In1.Cu")
	if n := strings.Count(g, `class="pour"`); n != 1 {
		t.Fatalf("In1.Cu draws %d pours, want 1", n)
	}
	i := strings.Index(g, `class="pour"`)
	path := g[i : strings.Index(g[i:], "/>")+i]
	if !strings.Contains(path, `fill-rule="evenodd"`) || strings.Count(path, "M") != 2 || strings.Count(path, "Z") != 2 {
		t.Errorf("pour %s, want an even-odd path of two closed rings, the outline and its hole", path)
	}
	if !strings.Contains(path, `data-kind="net" data-net="GND"`) {
		t.Errorf("pour %s takes no net key, so a click on the plane picks nothing", path)
	}
	rows := BoardLayers(shapeBoard())
	if p := boardRow(t, rows, "Pours"); p.GetSvgSelector() != ".pour" {
		t.Errorf("the Pours row selects %q", p.GetSvgSelector())
	}
}

// The packed triangles of a pour cover its area less its hole's, so WebGL fills what SVG fills.
func TestACopperShapePacksAsTrianglesCoveringItLessItsHole(t *testing.T) {
	b := shapeBoard()
	ps := PackBoard(b)
	pour := boardRow(t, BoardLayers(b), "Pours").GetGroups()
	if len(pour) != 1 {
		t.Fatalf("the Pours row has groups %v, want the one on In1.Cu", pour)
	}
	if in1 := boardRow(t, BoardLayers(b), "In1.Cu").GetGroups(); len(in1) != 2 || in1[1] != pour[0] {
		t.Errorf("In1.Cu's groups %v lack its pour group %d, so hiding the layer leaves its plane drawn", in1, pour[0])
	}
	vert := func(i uint32) (float64, float64) {
		v := ps.GetVertices()
		return float64(int32(binary.LittleEndian.Uint32(v[i*8:]))), float64(int32(binary.LittleEndian.Uint32(v[i*8+4:])))
	}
	area := 0.0
	recs := ps.GetPrimitives()
	for r := 0; r+primRecordBytes <= len(recs); r += primRecordBytes {
		if uint32(recs[r+1]) != pour[0] || recs[r] != primTriangles {
			continue
		}
		first, count := binary.LittleEndian.Uint32(recs[r+4:]), binary.LittleEndian.Uint32(recs[r+8:])
		for k := first; k+2 < first+count; k += 3 {
			ax, ay := vert(k)
			bx, by := vert(k + 1)
			cx, cy := vert(k + 2)
			area += math.Abs((bx-ax)*(cy-ay)-(cx-ax)*(by-ay)) / 2
		}
	}
	want := 20e6*20e6 - 4e6*4e6
	if math.Abs(area-want) > want*1e-9 {
		t.Errorf("the pour's triangles cover %.0f nm², want %.0f", area, want)
	}
}

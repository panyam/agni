package render

import (
	"math"
	"regexp"
	"strconv"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// arcBoard holds one arc track: a quarter circle of 10 mm radius about (20, 20) mm, from (30, 20)
// to (20, 30), counter-clockwise, 0.5 mm wide on F.Cu.
func arcBoard(mid *geom.Point) *geom.BoardGeometry {
	return &geom.BoardGeometry{
		UnitNm: 1,
		Layers: []*geom.BoardLayer{{Number: 0, Name: "F.Cu", Kind: "signal"}},
		Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{
			{X: 0, Y: 0}, {X: 40e6, Y: 0}, {X: 40e6, Y: 40e6}, {X: 0, Y: 40e6}, {X: 0, Y: 0},
		}}}},
		Nets: []*geom.NetCopper{{Net: "SIG", Segments: []*geom.TrackSegment{{
			A: &geom.Point{X: 30e6, Y: 20e6}, Mid: mid, B: &geom.Point{X: 20e6, Y: 30e6}, Width: 500_000, Layer: "F.Cu",
		}}}},
	}
}

var ccwMid = &geom.Point{X: 20e6 + 7_071_068, Y: 20e6 + 7_071_068}

// svgArcCenter solves an SVG arc command's center from its two ends, its radius and its flags, as
// the SVG spec's endpoint-to-center conversion does for a circle.
func svgArcCenter(x1, y1, x2, y2, r float64, large, sweep bool) (float64, float64) {
	hx, hy := (x1-x2)/2, (y1-y2)/2
	d2 := hx*hx + hy*hy
	k := math.Sqrt(math.Max(0, (r*r-d2)/d2))
	if large == sweep {
		k = -k
	}
	return k*hy + (x1+x2)/2, -k*hx + (y1+y2)/2
}

// TestBoardSVGDrawsAnArcTrackAsAnArc reads back each half of the path the SVG draws for an arc and
// solves the circle the browser will draw. Both halves have to land on the arc's own center, with
// the flags the right way round, or the curve bulges to the wrong side.
func TestBoardSVGDrawsAnArcTrackAsAnArc(t *testing.T) {
	for _, tc := range []struct {
		name string
		mid  *geom.Point
	}{
		{"counter-clockwise quarter", ccwMid},
		{"clockwise three quarters", &geom.Point{X: 20e6 - 7_071_068, Y: 20e6 - 7_071_068}},
	} {
		b := arcBoard(tc.mid)
		svgOut := BoardSVG(b)
		m := regexp.MustCompile(`<path d="M ([-\d.]+) ([-\d.]+) A ([\d.]+) [\d.]+ 0 (\d) (\d) ([-\d.]+) ([-\d.]+) A [\d.]+ [\d.]+ 0 (\d) (\d) ([-\d.]+) ([-\d.]+)"`).FindStringSubmatch(svgOut)
		if m == nil {
			t.Fatalf("%s: no arc path in the board SVG", tc.name)
		}
		f := func(i int) float64 { v, _ := strconv.ParseFloat(m[i], 64); return v }
		fr := frameBoard(b)
		cx, cy, r := fr.tx(20e6), fr.ty(20e6), f(3)
		if !near(r, 10e6*fr.scale, 0.02) {
			t.Errorf("%s: radius %.2f px, want %.2f", tc.name, r, 10e6*fr.scale)
		}
		for i, h := range [][7]float64{{f(1), f(2), f(6), f(7), r, f(4), f(5)}, {f(6), f(7), f(10), f(11), r, f(8), f(9)}} {
			gx, gy := svgArcCenter(h[0], h[1], h[2], h[3], h[4], h[5] == 1, h[6] == 1)
			if !near(gx, cx, 0.05) || !near(gy, cy, 0.05) {
				t.Errorf("%s: half %d turns about (%.2f, %.2f) px, want the arc's center (%.2f, %.2f)", tc.name, i+1, gx, gy, cx, cy)
			}
		}
		if !near(f(6), fr.tx(tc.mid.X), 0.01) || !near(f(7), fr.ty(tc.mid.Y), 0.01) {
			t.Errorf("%s: the halves meet at (%s, %s), want the mid point", tc.name, m[6], m[7])
		}
	}
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// TestPackBoardPacksAnArcAsItsChords holds every vertex the packed transport sends for an arc to the
// annulus the track covers. The chord across a quarter circle runs 2.9 mm inside it.
func TestPackBoardPacksAnArcAsItsChords(t *testing.T) {
	ps := PackBoard(arcBoard(ccwMid))
	verts := decodeVerts(ps.GetVertices())
	copper := 0
	for _, rec := range decodeRecords(ps.GetPrimitives()) {
		if rec.group != groupBoardCopperFront {
			continue
		}
		copper++
		if rec.count < 6*10 {
			t.Errorf("the arc packed as %d vertices, want the quads of at least ten chords", rec.count)
		}
		for _, v := range verts[rec.firstVertex : rec.firstVertex+rec.count] {
			x, y := float64(int64(v[0])+ps.GetOriginX()), float64(int64(v[1])+ps.GetOriginY())
			if d := math.Hypot(x-20e6, y-20e6); d < 10e6-260_000 || d > 10e6+260_000 {
				t.Fatalf("vertex (%.0f, %.0f) is %.0f nm from the center, want within the track's 0.25 mm of 10 mm", x, y, d)
			}
		}
	}
	if copper != 1 {
		t.Errorf("packed %d front copper records, want the one arc", copper)
	}
}

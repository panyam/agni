package kicad

import (
	"math"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestArcPolylineSweepsThroughItsMidpoint pins which way round an arc goes. A KiCad arc states no
// direction, so the midpoint picks one of the two arcs joining its ends, and a wrong pick keeps the
// ends, the width and the net, so only the shape shows it. The sweeps cover both directions on each
// side of 180 degrees. kicad-cli's Gerber export of the same arcs agrees to 0.004 mm.
func TestArcPolylineSweepsThroughItsMidpoint(t *testing.T) {
	const steps, tolNm = 16, 100.0
	cases := []struct {
		name          string
		cx, cy, r     float64
		start, degree float64
	}{
		{"minor, increasing angle", 12e6, -10e6, 3e6, 200, 140},
		{"minor, decreasing angle", 12e6, -20e6, 3e6, -20, -140},
		{"major, increasing angle", 30e6, -10e6, 5e6, 0, 270},
		{"major, decreasing angle", 30e6, -25e6, 5e6, 0, -270},
		{"shallow", 60e6, -40e6, 30e6, 260, 20},
		{"near a full circle", 60e6, -20e6, 3e6, 10, 350},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			at := func(frac float64) (x, y float64) {
				a := (c.start + c.degree*frac) * math.Pi / 180
				return c.cx + c.r*math.Cos(a), c.cy + c.r*math.Sin(a)
			}
			point := func(frac float64) *geom.Point {
				x, y := at(frac)
				return &geom.Point{X: int64(math.Round(x)), Y: int64(math.Round(y))}
			}
			pts := arcPolyline(point(0), point(0.5), point(1)).GetPoints()
			if len(pts) != steps+1 {
				t.Fatalf("got %d points, want %d", len(pts), steps+1)
			}
			for i, p := range pts {
				wantX, wantY := at(float64(i) / steps)
				if d := math.Hypot(float64(p.X)-wantX, float64(p.Y)-wantY); d > tolNm {
					t.Errorf("point %d is %.0f nm off the arc", i, d)
				}
			}
		})
	}
}

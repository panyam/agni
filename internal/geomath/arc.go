package geomath

import (
	"math"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// ArcShape samples an arc (center cx,cy; radius r; start angle a; sweep b, all in degrees) as
// a 3-point geom arc through its start, midpoint, and end (geom.Shape_KIND_ARC's contract).
// The point constructor pt maps a native (x,y) into the reader's geom frame, so the xschem and
// gEDA arc builders (identical but for gpt vs gedaPt) share one implementation.
func ArcShape(cx, cy, r, a, b float64, pt func(x, y float64) *geom.Point) *geom.Shape {
	at := func(deg float64) *geom.Point {
		rad := deg * math.Pi / 180
		return pt(cx+r*math.Cos(rad), cy+r*math.Sin(rad))
	}
	return &geom.Shape{Kind: geom.Shape_KIND_ARC, Points: []*geom.Point{at(a), at(a + b/2), at(a + b)}}
}

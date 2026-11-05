package render

import (
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
)

// dnpCross is the two diagonals of a do-not-populate part's body, in sheet coordinates, as
// KiCad crosses one out. The body is the symbol's artwork without its pin legs, so the cross
// covers the box or the circle and stops short of the pins. ok is false for a placement that is
// fitted or draws nothing.
func dnpCross(sym *geom.SymbolDef, pl *geom.SymbolPlacement) (a, b [2]*geom.Point, ok bool) {
	if sym == nil || !pl.GetDnp() {
		return a, b, false
	}
	pinAt := map[[2]int64]bool{}
	for _, p := range sym.Pins {
		if p.Loc != nil {
			pinAt[[2]int64{p.Loc.X, p.Loc.Y}] = true
		}
	}
	var body geomath.Bounds
	for _, s := range sym.Shapes {
		if isPinLeg(s, pinAt) {
			continue
		}
		if s.Kind == geom.Shape_KIND_CIRCLE && len(s.Points) == 1 {
			c, r := s.Points[0], s.Radius
			body.Add(&geom.Point{X: c.X - r, Y: c.Y - r})
			body.Add(&geom.Point{X: c.X + r, Y: c.Y + r})
			continue
		}
		for _, p := range s.Points {
			body.Add(p)
		}
	}
	if !body.Valid() {
		return a, b, false
	}
	x0, y0 := body.Min()
	x1, y1 := body.Max()
	var w geomath.Bounds
	for _, p := range []*geom.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x0, Y: y1}, {X: x1, Y: y1}} {
		w.Add(geomath.ApplyTransform(pl.Transform, p))
	}
	wx0, wy0 := w.Min()
	wx1, wy1 := w.Max()
	a = [2]*geom.Point{{X: wx0, Y: wy0}, {X: wx1, Y: wy1}}
	b = [2]*geom.Point{{X: wx0, Y: wy1}, {X: wx1, Y: wy0}}
	return a, b, true
}

// isPinLeg reports whether a shape is a pin's own line: a two-point polyline starting or ending
// at a pin's connect point.
func isPinLeg(s *geom.Shape, pinAt map[[2]int64]bool) bool {
	if s.Kind != geom.Shape_KIND_POLYLINE || len(s.Points) != 2 {
		return false
	}
	for _, p := range s.Points {
		if pinAt[[2]int64{p.X, p.Y}] {
			return true
		}
	}
	return false
}

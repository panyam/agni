package geomath

import (
	"math"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// ArcShape samples an arc (center cx,cy; radius r; start angle a; sweep b, all in degrees) as
// a 3-point geom arc through its start, midpoint, and end (geom.Shape_KIND_ARC's contract).
// pt maps a native (x,y) into the reader's geom frame. The xschem and gEDA callers differ only in
// the pt they pass (gpt vs gedaPt).
func ArcShape(cx, cy, r, a, b float64, pt func(x, y float64) *geom.Point) *geom.Shape {
	at := func(deg float64) *geom.Point {
		rad := deg * math.Pi / 180
		return pt(cx+r*math.Cos(rad), cy+r*math.Sin(rad))
	}
	return &geom.Shape{Kind: geom.Shape_KIND_ARC, Points: []*geom.Point{at(a), at(a + b/2), at(a + b)}}
}

// ArcStep is the largest angle, in radians, that one chord of a sampled arc spans. A tenth of a
// radian keeps a chord within 0.125% of the radius of the arc it stands for.
const ArcStep = 0.1

// Arc is a circular arc in center form. Start is the angle of its first point, in radians, and
// Sweep is the signed angle it turns through, positive counter-clockwise in the Y-up frame.
type Arc struct {
	Cx, Cy, R    float64
	Start, Sweep float64
}

// ArcThrough finds the circle through a start, a mid and an end point, and the sweep from start to
// end that passes the mid point. ok is false when the three points are collinear or coincide,
// since no circle passes through them.
func ArcThrough(s, m, e *geom.Point) (Arc, bool) {
	if s == nil || m == nil || e == nil {
		return Arc{}, false
	}
	ax, ay := float64(s.X), float64(s.Y)
	bx, by := float64(m.X), float64(m.Y)
	cx, cy := float64(e.X), float64(e.Y)
	d := 2 * (ax*(by-cy) + bx*(cy-ay) + cx*(ay-by))
	if math.Abs(d) < 1e-9 {
		return Arc{}, false
	}
	a2, b2, c2 := ax*ax+ay*ay, bx*bx+by*by, cx*cx+cy*cy
	ox := (a2*(by-cy) + b2*(cy-ay) + c2*(ay-by)) / d
	oy := (a2*(cx-bx) + b2*(ax-cx) + c2*(bx-ax)) / d
	a0 := math.Atan2(ay-oy, ax-ox)
	a1 := math.Atan2(by-oy, bx-ox)
	a2a := math.Atan2(cy-oy, cx-ox)
	sweep, toMid := ccwTurn(a0, a2a), ccwTurn(a0, a1)
	if toMid > sweep {
		sweep -= 2 * math.Pi
	}
	return Arc{Cx: ox, Cy: oy, R: math.Hypot(ax-ox, ay-oy), Start: a0, Sweep: sweep}, true
}

func ccwTurn(from, to float64) float64 {
	d := math.Mod(to-from, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	return d
}

// At returns the point of the arc a fraction t of the way along its sweep.
func (a Arc) At(t float64) (float64, float64) {
	ang := a.Start + a.Sweep*t
	return a.Cx + a.R*math.Cos(ang), a.Cy + a.R*math.Sin(ang)
}

// Length is the arc's length along the curve.
func (a Arc) Length() float64 { return math.Abs(a.Sweep) * a.R }

// ArcMid returns the point halfway along the arc that runs from (sx, sy) to (ex, ey) about the
// center (cx, cy), clockwise when cw is set. A reader whose format stores an arc in center form
// uses it to fill a TrackSegment's mid. Start and end must differ, because a full circle has no
// single mid point; a reader splits one into two halves.
func ArcMid(sx, sy, ex, ey, cx, cy float64, cw bool) (float64, float64) {
	start := math.Atan2(sy-cy, sx-cx)
	sweep := ccwTurn(start, math.Atan2(ey-cy, ex-cx))
	if cw {
		sweep -= 2 * math.Pi
	}
	r := (math.Hypot(sx-cx, sy-cy) + math.Hypot(ex-cx, ey-cy)) / 2
	ang := start + sweep/2
	return cx + r*math.Cos(ang), cy + r*math.Sin(ang)
}

// ArcChords samples the arc from s through m to e as a polyline whose chords each span at most
// ArcStep radians. The first and last points are s and e themselves. Collinear points come back
// as the path s, m, e.
func ArcChords(s, m, e *geom.Point) []*geom.Point {
	a, ok := ArcThrough(s, m, e)
	if !ok {
		return []*geom.Point{s, m, e}
	}
	n := max(int(math.Ceil(math.Abs(a.Sweep)/ArcStep)), 2)
	pts := make([]*geom.Point, 0, n+1)
	pts = append(pts, s)
	for i := 1; i < n; i++ {
		x, y := a.At(float64(i) / float64(n))
		pts = append(pts, &geom.Point{X: int64(math.Round(x)), Y: int64(math.Round(y))})
	}
	return append(pts, e)
}

// TrackPath returns a track segment's path: its two ends for a straight run, or the chords of
// ArcChords for an arc, which a segment marks by setting mid. Every consumer that cannot draw a
// curve reads a segment through it, because reading an arc's a and b alone draws the chord
// across the curve.
func TrackPath(s *geom.TrackSegment) []*geom.Point {
	if s.GetMid() == nil {
		return []*geom.Point{s.GetA(), s.GetB()}
	}
	return ArcChords(s.GetA(), s.GetMid(), s.GetB())
}

// TrackLength is a track segment's length along the copper, the arc's length for an arc.
func TrackLength(s *geom.TrackSegment) float64 {
	if s.GetMid() != nil {
		if a, ok := ArcThrough(s.GetA(), s.GetMid(), s.GetB()); ok {
			return a.Length()
		}
	}
	pts := TrackPath(s)
	var l float64
	for i := 1; i < len(pts); i++ {
		l += math.Hypot(float64(pts[i].GetX()-pts[i-1].GetX()), float64(pts[i].GetY()-pts[i-1].GetY()))
	}
	return l
}

// ArcPiece is one arc as a reader emits it into a TrackSegment: start, mid and end, each an (x, y)
// pair in the reader's own units.
type ArcPiece struct{ S, M, E [2]float64 }

// CenterArc turns an arc stored in center form into the pieces a TrackSegment can hold. The arc runs
// from (sx, sy) to (ex, ey) about (cx, cy), clockwise when cw is set. An arc whose start and end
// coincide is a full circle, which has no single mid point, so it comes back as two half circles.
// The other arcs come back as one piece.
func CenterArc(sx, sy, ex, ey, cx, cy float64, cw bool) []ArcPiece {
	if sx != ex || sy != ey {
		mx, my := ArcMid(sx, sy, ex, ey, cx, cy, cw)
		return []ArcPiece{{S: [2]float64{sx, sy}, M: [2]float64{mx, my}, E: [2]float64{ex, ey}}}
	}
	r := math.Hypot(sx-cx, sy-cy)
	a := math.Atan2(sy-cy, sx-cx)
	dir := 1.0
	if cw {
		dir = -1
	}
	at := func(t float64) [2]float64 {
		return [2]float64{cx + r*math.Cos(a+dir*t), cy + r*math.Sin(a+dir*t)}
	}
	s, q1, h, q3 := [2]float64{sx, sy}, at(math.Pi/2), at(math.Pi), at(3*math.Pi/2)
	return []ArcPiece{{S: s, M: q1, E: h}, {S: h, M: q3, E: s}}
}

// TurnTo is the signed angle the arc turns through from its start to the point (x, y), measured in
// the arc's own direction. A renderer that splits an arc at its mid point reads each half's sweep
// from it.
func (a Arc) TurnTo(x, y float64) float64 {
	ang := math.Atan2(y-a.Cy, x-a.Cx)
	if a.Sweep >= 0 {
		return ccwTurn(a.Start, ang)
	}
	return -ccwTurn(ang, a.Start)
}

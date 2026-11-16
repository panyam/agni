package render

import (
	"math"
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// partBodies gives each placed part a body for the stack view to stand on its side's face (agni
// 1352). A part's footprint is the hull of its courtyard, or of its fab outline when it has no
// courtyard, or of its pads when it has neither. A part with none of the three has no body.
func partBodies(b *geom.BoardGeometry, style Style) []*geom.BoardPartBody {
	unit := unitNm(b)
	art := map[string]map[string][][2]float64{}
	for _, g := range b.GetGraphics() {
		ref := g.GetRefDes()
		kind := artKind(g.GetLayer())
		if ref == "" || (kind != "courtyard" && kind != "fab") {
			continue
		}
		if art[ref] == nil {
			art[ref] = map[string][][2]float64{}
		}
		art[ref][kind] = append(art[ref][kind], shapePoints(g.GetShape())...)
	}
	var out []*geom.BoardPartBody
	for _, pl := range b.GetPlacements() {
		pts := art[pl.GetRefDes()]["courtyard"]
		if len(pts) < 3 {
			pts = art[pl.GetRefDes()]["fab"]
		}
		if len(pts) < 3 {
			pts = padSquares(pl)
		}
		ring := convexHull(pts)
		if len(ring) < 3 {
			continue
		}
		body := &geom.BoardPartBody{RefDes: pl.GetRefDes(), Side: placementSide(pl)}
		body.PartClass, body.Color = partClassColor(style, pl.GetRefDes())
		for _, p := range ring {
			body.Outline = append(body.Outline, p[0], p[1])
		}
		if pl.Height != nil {
			mm := float64(pl.GetHeight()*unit) / 1e6
			body.HeightMm = &mm
		}
		out = append(out, body)
	}
	return out
}

// partClassColor is a part's most specific class and the colour of the first of its classes that
// has one. A part with no class, or none with a colour, keeps PartBody.
func partClassColor(style Style, ref string) (string, string) {
	if style.partClasses == nil || *style.partClasses == nil {
		return "", style.PartBody
	}
	classes := (*style.partClasses)(ref)
	if len(classes) == 0 {
		return "", style.PartBody
	}
	for _, c := range classes {
		if color := style.PartClass.of(c); color != "" {
			return classes[0], color
		}
	}
	return classes[0], style.PartBody
}

// shapePoints is the points a shape reaches. A rect gives its four corners and a circle eight
// points on a square around it, so the hull covers the circle.
func shapePoints(s *geom.Shape) [][2]float64 {
	pts := s.GetPoints()
	switch s.GetKind() {
	case geom.Shape_KIND_RECT:
		if len(pts) < 2 {
			return nil
		}
		a, c := pts[0], pts[1]
		return [][2]float64{{float64(a.X), float64(a.Y)}, {float64(c.X), float64(a.Y)}, {float64(c.X), float64(c.Y)}, {float64(a.X), float64(c.Y)}}
	case geom.Shape_KIND_CIRCLE:
		if len(pts) < 1 {
			return nil
		}
		x, y, r := float64(pts[0].X), float64(pts[0].Y), float64(s.GetRadius())
		var out [][2]float64
		for k := 0; k < 8; k++ {
			a := float64(k) * math.Pi / 4
			// The square's corners sit r√2 out, so the eight points enclose the circle.
			d := r
			if k%2 == 1 {
				d = r * math.Sqrt2
			}
			out = append(out, [2]float64{x + d*math.Cos(a), y + d*math.Sin(a)})
		}
		return out
	}
	out := make([][2]float64, 0, len(pts))
	for _, p := range pts {
		out = append(out, [2]float64{float64(p.X), float64(p.Y)})
	}
	return out
}

// padSquares is a square around each pad, half the pad's longer side out from its centre, as
// footprintBody's pick area takes it.
func padSquares(pl *geom.ComponentPlacement) [][2]float64 {
	var out [][2]float64
	for _, pad := range pl.GetPads() {
		wx, wy := padWorld(pl, pad)
		x, y := float64(wx), float64(wy)
		h := float64(max(pad.GetSize().GetX(), pad.GetSize().GetY())) / 2
		out = append(out, [2]float64{x - h, y - h}, [2]float64{x + h, y - h}, [2]float64{x + h, y + h}, [2]float64{x - h, y + h})
	}
	return out
}

// convexHull is the hull of pts, counter-clockwise, by Andrew's monotone chain. It drops points
// that sit on an edge.
func convexHull(pts [][2]float64) [][2]float64 {
	if len(pts) < 3 {
		return nil
	}
	p := append([][2]float64{}, pts...)
	sort.Slice(p, func(i, j int) bool { return p[i][0] < p[j][0] || (p[i][0] == p[j][0] && p[i][1] < p[j][1]) })
	cross := func(o, a, b [2]float64) float64 {
		return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0])
	}
	hull := make([][2]float64, 0, 2*len(p))
	for _, q := range p {
		for len(hull) >= 2 && cross(hull[len(hull)-2], hull[len(hull)-1], q) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, q)
	}
	lower := len(hull) + 1
	for i := len(p) - 2; i >= 0; i-- {
		for len(hull) >= lower && cross(hull[len(hull)-2], hull[len(hull)-1], p[i]) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, p[i])
	}
	return hull[:len(hull)-1]
}

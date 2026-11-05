package copperraster

import (
	"math"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// DrawBoardLayer draws one copper layer of a board's geometry: the tracks on it into tracks, and
// every via whose span covers it, every pad on it and every filled copper shape on it into lands.
// The two may be the same bitmap.
// order lists the board's copper layers top to bottom, so a via's span and a through pad ("*.Cu")
// resolve to the layers they cross. Zones are not drawn.
//
// A pad or via that names a padstack is drawn on each layer from that layer's land, and on no layer
// where the padstack has none (agni 1356). Otherwise a pad on the layer is drawn from its own shape.
//
// A pad is drawn from its shape word at its board position, composed as the contract defines it:
// world = at + M(R(rotation) * local), with M mirroring X on a mirrored placement. A shape the
// contract has no outline for (custom, octagon, chamfered) is drawn as its rectangle extent.
func DrawBoardLayer(g *geom.BoardGeometry, layer string, order []string, tracks, lands *Bitmap) {
	scale := float64(g.GetUnitNm())
	if scale == 0 {
		scale = 1
	}
	idx := map[string]int{}
	for i, l := range order {
		idx[l] = i
	}
	at, ok := idx[layer]
	if !ok {
		return
	}
	stacks := map[string]*geom.Padstack{}
	for _, ps := range g.GetPadstacks() {
		stacks[ps.GetId()] = ps
	}

	for _, nc := range g.GetNets() {
		for _, s := range nc.GetSegments() {
			if s.GetLayer() != layer {
				continue
			}
			for i, pts := 1, geomath.TrackPath(s); i < len(pts); i++ {
				tracks.Line(float64(pts[i-1].GetX())*scale, float64(pts[i-1].GetY())*scale,
					float64(pts[i].GetX())*scale, float64(pts[i].GetY())*scale, float64(s.GetWidth())*scale)
			}
		}
		for _, sh := range nc.GetShapes() {
			if sh.GetLayer() == layer {
				lands.Rings(shapeRings(sh, scale), true)
			}
		}
		for _, v := range nc.GetVias() {
			from, okf := idx[v.GetLayerFrom()]
			to, okt := idx[v.GetLayerTo()]
			if !okf || !okt {
				from, to = 0, len(order)-1
			}
			if ps := stacks[v.GetPadstack()]; ps != nil {
				vx, vy := float64(v.GetAt().GetX())*scale, float64(v.GetAt().GetY())*scale
				if land := LandOn(ps, layer); land != nil {
					drawShape(lands, vx, vy, land.GetShape(), float64(land.GetSize().GetX())*scale, float64(land.GetSize().GetY())*scale, 0)
				}
				continue
			}
			if at >= min(from, to) && at <= max(from, to) {
				lands.Disc(float64(v.GetAt().GetX())*scale, float64(v.GetAt().GetY())*scale, float64(v.GetSize())*scale)
			}
		}
	}
	for _, pl := range g.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if ps := stacks[pad.GetPadstack()]; ps != nil {
				if land := LandOn(ps, layer); land != nil {
					drawPad(lands, pl, &geom.Pad{At: pad.GetAt(), RotationDeg: pad.GetRotationDeg(), Shape: land.GetShape(), Size: land.GetSize()}, scale)
				}
				continue
			}
			if !padOn(pad, layer) || pad.GetSize() == nil {
				continue
			}
			drawPad(lands, pl, pad, scale)
		}
	}
}

func padOn(pad *geom.Pad, layer string) bool {
	for _, l := range pad.GetLayers() {
		if l == layer || (l == "*.Cu" && strings.HasSuffix(layer, ".Cu")) {
			return true
		}
	}
	return false
}

func drawPad(b *Bitmap, pl *geom.ComponentPlacement, pad *geom.Pad, scale float64) {
	rot := pl.GetRotationDeg() * math.Pi / 180
	s, c := math.Sincos(rot)
	lx, ly := float64(pad.GetAt().GetX()), float64(pad.GetAt().GetY())
	x, y := lx*c-ly*s, lx*s+ly*c
	angle := pl.GetRotationDeg() + pad.GetRotationDeg()
	if pl.GetMirror() {
		x = -x
		angle = -angle
	}
	cx := (float64(pl.GetAt().GetX()) + x) * scale
	cy := (float64(pl.GetAt().GetY()) + y) * scale
	drawShape(b, cx, cy, pad.GetShape(), float64(pad.GetSize().GetX())*scale, float64(pad.GetSize().GetY())*scale, angle)
}

func drawShape(b *Bitmap, cx, cy float64, shape string, w, h, angle float64) {
	switch shape {
	case "circle":
		b.Disc(cx, cy, math.Min(w, h))
	case "oval":
		b.Oval(cx, cy, w, h, angle)
	default:
		b.Rect(cx, cy, w, h, angle)
	}
}

// LandOn is a padstack's land on one copper layer, or nil where it has none.
func LandOn(ps *geom.Padstack, layer string) *geom.PadShape {
	for _, l := range ps.GetLayers() {
		if l.GetLayer() == layer {
			return l.GetPad()
		}
	}
	return nil
}

// DrawBoardOutline strokes the board outline into b at the given width in nanometres. Allegro plots
// the outline on every copper film (its films list BOARD GEOMETRY/OUTLINE beside ETCH), so a
// comparison against them draws it on agni's side too.
func DrawBoardOutline(g *geom.BoardGeometry, width float64, b *Bitmap) {
	scale := float64(g.GetUnitNm())
	if scale == 0 {
		scale = 1
	}
	for _, p := range g.GetOutline().GetPaths() {
		pts := p.GetPoints()
		for i := 1; i < len(pts); i++ {
			b.Line(float64(pts[i-1].GetX())*scale, float64(pts[i-1].GetY())*scale,
				float64(pts[i].GetX())*scale, float64(pts[i].GetY())*scale, width)
		}
	}
}

// shapeRings is a copper shape's outline and holes as rings in nanometres, for an even-odd fill.
func shapeRings(sh *geom.CopperShape, scale float64) [][][2]float64 {
	ring := func(pl *geom.Polyline) [][2]float64 {
		out := make([][2]float64, 0, len(pl.GetPoints()))
		for _, p := range pl.GetPoints() {
			out = append(out, [2]float64{float64(p.GetX()) * scale, float64(p.GetY()) * scale})
		}
		return out
	}
	rings := [][][2]float64{ring(sh.GetOutline())}
	for _, h := range sh.GetHoles() {
		rings = append(rings, ring(h))
	}
	return rings
}

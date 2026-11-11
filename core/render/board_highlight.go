package render

import (
	"math"

	"github.com/panyam/agni/core/svg"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
)

// HighlightBoardSVG resolves highlight specs against a board and returns a transparent SVG
// overlay framed like BoardSVG (both use frameBoard), which a client stacks above the base
// document. It is the board face of the WS9-016 highlight contract. A listed NET matches its
// routed copper (segments re-stroked wider, vias ringed) and every pad on it, a listed
// COMPONENT matches all its pads plus a marker ring at the placement origin, and a listed pin
// matches that (ref_des, pad number). A spec with a bounding shape frames each listed component and
// pin instead, as HighlightSVG does on a sheet (agni issue 1536). A net listed by id matches
// through the render's WithNetIDs, since the board names its nets and holds no schematic ids.
// Specs paint in order, later wins.
func HighlightBoardSVG(b *geom.BoardGeometry, specs []*geom.HighlightSpec, opts ...Option) string {
	fr := frameBoard(b)
	netID := boardNetIDs(b, resolveStyle(opts))
	c := svg.Open(fr.pxW, fr.pxH) // transparent, no background rect
	art := map[string][]*geom.Shape{}
	for _, gr := range b.GetGraphics() {
		if gr.GetRefDes() != "" && gr.GetShape() != nil {
			art[gr.GetRefDes()] = append(art[gr.GetRefDes()], gr.GetShape())
		}
	}

	for _, spec := range specs {
		m := matcherFor(spec)
		color := highlightColor(spec.GetColor())
		alpha := highlightAlpha(spec.GetAlpha())
		shape := spec.GetShape()
		framed := shape == geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT || shape == geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_CIRCLE
		frame := func(e *highlightEntity) {
			if fb, ok := framedBounds(e, fr.frameFloor()); ok {
				drawFrame(c, fb, fr, shape == geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_CIRCLE, color,
					highlightFillAlpha(spec.GetAlpha()), strokeScaleOr1(spec.GetStrokeScale()))
			}
		}

		for _, nc := range b.GetNets() {
			if !m.matchWire(nc.GetNet(), netID(nc.GetNet())) {
				continue
			}
			for _, s := range nc.GetSegments() {
				if s.GetA() == nil || s.GetB() == nil {
					continue
				}
				drawTrack(c, s, fr, svg.A("stroke", color), svg.F("stroke-opacity", alpha),
					svg.F("stroke-width", copperStrokePx(s.GetWidth(), fr.scale)+highlightStrokePx),
					svg.A("stroke-linecap", "round"))
			}
			// A net's pours light as a tinted fill under an outline, so a plane reads as part of its
			// net without hiding the copper drawn on it (agni 1357).
			for _, sh := range nc.GetShapes() {
				c.El("path", svg.A("d", shapePathD(sh, fr)), svg.A("fill", color), svg.F("fill-opacity", alpha*0.35),
					svg.A("fill-rule", "evenodd"), svg.A("stroke", color), svg.F("stroke-opacity", alpha),
					svg.F("stroke-width", highlightStrokePx/2))
			}
			for _, v := range nc.GetVias() {
				if v.GetAt() == nil {
					continue
				}
				c.El("circle", svg.F("cx", fr.tx(v.GetAt().X)), svg.F("cy", fr.ty(v.GetAt().Y)),
					svg.F("r", math.Max(float64(v.GetSize())*fr.scale/2, strokePx)+highlightStrokePx/2),
					svg.A("fill", "none"), svg.A("stroke", color),
					svg.F("stroke-opacity", alpha), svg.F("stroke-width", highlightStrokePx))
			}
		}

		for _, pl := range b.GetPlacements() {
			wholeComp := m.comps[pl.GetRefDes()]
			if framed && wholeComp {
				e := &highlightEntity{shapes: art[pl.GetRefDes()]}
				for _, pad := range pl.GetPads() {
					e.pins = append(e.pins, padCorners(pl, pad)...)
				}
				frame(e)
			}
			for _, pad := range pl.GetPads() {
				onNet := pad.GetNet() != "" && m.matchWire(pad.GetNet(), netID(pad.GetNet()))
				onPin := m.pins[[2]string{pl.GetRefDes(), pad.GetNumber()}]
				if !wholeComp && !onNet && !onPin {
					continue
				}
				if framed && (wholeComp || onPin && !onNet) {
					if !wholeComp {
						frame(&highlightEntity{pins: padCorners(pl, pad)})
					}
					continue
				}
				wx, wy := padWorld(pl, pad)
				r := math.Max(math.Max(float64(pad.GetSize().GetX()), float64(pad.GetSize().GetY()))*fr.scale/2, strokePx) + highlightStrokePx/2
				c.El("circle", svg.F("cx", fr.tx(wx)), svg.F("cy", fr.ty(wy)), svg.F("r", r),
					svg.A("fill", "none"), svg.A("stroke", color),
					svg.F("stroke-opacity", alpha), svg.F("stroke-width", highlightStrokePx))
			}
			if wholeComp && !framed && pl.GetAt() != nil {
				c.El("circle", svg.F("cx", fr.tx(pl.GetAt().X)), svg.F("cy", fr.ty(pl.GetAt().Y)),
					svg.F("r", pinRPx*2), svg.A("fill", color), svg.F("fill-opacity", alpha))
			}
		}
	}
	return c.String()
}

// padCorners returns the world points a pad's packed primitive spans, the corners of its turned
// rectangle or the rim of its disk, so a board frame bounds the pads WebGL bounds (packPad).
func padCorners(pl *geom.ComponentPlacement, pad *geom.Pad) []*geom.Point {
	wx, wy := padWorld(pl, pad)
	var pts [][2]int64
	if pad.GetShape() == "circle" {
		pts = diskPts(wx, wy, float64(pad.GetSize().GetX())/2)
	} else {
		pts = padQuadPts(wx, wy, pad.GetSize(), geomath.PadAngle(pl, pad))
	}
	if len(pts) == 0 {
		return []*geom.Point{{X: wx, Y: wy}}
	}
	out := make([]*geom.Point, len(pts))
	for i, p := range pts {
		out[i] = &geom.Point{X: p[0], Y: p[1]}
	}
	return out
}

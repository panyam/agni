package render

import (
	"math"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/svg"
)

// HighlightBoardSVG resolves highlight specs against a board and returns a transparent SVG
// overlay framed exactly like BoardSVG (both use frameBoard), so a client stacks it above
// the base document — the board face of the WS9-016 highlight contract. The join mirrors
// the board's key vocabulary: a listed NET matches its routed copper (segments re-stroked
// wider, vias ringed) and every pad connected to it; a listed COMPONENT matches all of its
// placement's pads plus a marker ring at the placement origin; a listed pin matches that
// (ref_des, pad number). Specs paint in order, later wins.
func HighlightBoardSVG(b *geom.BoardGeometry, specs []*geom.HighlightSpec) string {
	fr := frameBoard(b)
	c := svg.Open(fr.pxW, fr.pxH) // transparent: no background rect

	for _, spec := range specs {
		m := matcherFor(spec)
		color := highlightColor(spec.GetColor())
		alpha := highlightAlpha(spec.GetAlpha())

		for _, nc := range b.GetNets() {
			if !m.nets[nc.GetNet()] {
				continue
			}
			for _, s := range nc.GetSegments() {
				if s.GetA() == nil || s.GetB() == nil {
					continue
				}
				c.El("line", svg.F("x1", fr.tx(s.GetA().X)), svg.F("y1", fr.ty(s.GetA().Y)),
					svg.F("x2", fr.tx(s.GetB().X)), svg.F("y2", fr.ty(s.GetB().Y)),
					svg.A("stroke", color), svg.F("stroke-opacity", alpha),
					svg.F("stroke-width", copperStrokePx(s.GetWidth(), fr.scale)+highlightStrokePx),
					svg.A("stroke-linecap", "round"))
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
			for _, pad := range pl.GetPads() {
				onNet := pad.GetNet() != "" && m.nets[pad.GetNet()]
				onPin := m.pins[[2]string{pl.GetRefDes(), pad.GetNumber()}]
				if !wholeComp && !onNet && !onPin {
					continue
				}
				wx, wy := padWorld(pl, pad)
				r := math.Max(math.Max(float64(pad.GetSize().GetX()), float64(pad.GetSize().GetY()))*fr.scale/2, strokePx) + highlightStrokePx/2
				c.El("circle", svg.F("cx", fr.tx(wx)), svg.F("cy", fr.ty(wy)), svg.F("r", r),
					svg.A("fill", "none"), svg.A("stroke", color),
					svg.F("stroke-opacity", alpha), svg.F("stroke-width", highlightStrokePx))
			}
			if wholeComp && pl.GetAt() != nil {
				c.El("circle", svg.F("cx", fr.tx(pl.GetAt().X)), svg.F("cy", fr.ty(pl.GetAt().Y)),
					svg.F("r", pinRPx*2), svg.A("fill", color), svg.F("fill-opacity", alpha))
			}
		}
	}
	return c.String()
}

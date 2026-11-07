package render

import (
	"math"

	"github.com/panyam/agni/core/svg"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// HighlightBoardSVG resolves highlight specs against a board and returns a transparent SVG
// overlay framed like BoardSVG (both use frameBoard), which a client stacks above the base
// document. It is the board face of the WS9-016 highlight contract. A listed NET matches its
// routed copper (segments re-stroked wider, vias ringed) and every pad on it, a listed
// COMPONENT matches all its pads plus a marker ring at the placement origin, and a listed pin
// matches that (ref_des, pad number). A net listed by id matches through the render's
// WithNetIDs, since the board names its nets and holds no schematic ids. Specs paint in order,
// later wins.
func HighlightBoardSVG(b *geom.BoardGeometry, specs []*geom.HighlightSpec, opts ...Option) string {
	fr := frameBoard(b)
	netID := boardNetIDs(b, resolveStyle(opts))
	c := svg.Open(fr.pxW, fr.pxH) // transparent, no background rect

	for _, spec := range specs {
		m := matcherFor(spec)
		color := highlightColor(spec.GetColor())
		alpha := highlightAlpha(spec.GetAlpha())

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
			for _, pad := range pl.GetPads() {
				onNet := pad.GetNet() != "" && m.matchWire(pad.GetNet(), netID(pad.GetNet()))
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

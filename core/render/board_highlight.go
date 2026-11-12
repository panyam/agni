package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/panyam/agni/core/svg"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
)

// HighlightBoardSVG resolves highlight specs against a board and returns a transparent SVG
// overlay framed like BoardSVG (both use frameBoard), which a client stacks above the base
// document. It is the board face of the WS9-016 highlight contract. A listed NET repaints its
// routed copper (segments re-stroked a little wider, vias and every pad on it filled, pours tinted)
// at boardNetAlpha (agni issue 1543), a listed
// COMPONENT matches all its pads plus a marker ring at the placement origin, and a listed pin
// matches that (ref_des, pad number). A spec with a bounding shape frames each listed component and
// pin instead, as HighlightSVG does on a sheet (agni issue 1536). A net listed by id matches
// through the render's WithNetIDs, since the board names its nets and holds no schematic ids.
// Specs paint in order, later wins.
func HighlightBoardSVG(b *geom.BoardGeometry, specs []*geom.HighlightSpec, opts ...Option) string {
	fr := frameBoard(b)
	netID := boardNetIDs(b, resolveStyle(opts))
	c := svg.Open(fr.pxW, fr.pxH) // transparent, no background rect
	stacks := padstacksOf(b)
	copper := map[string]bool{}
	for _, ly := range copperLayersBackToFront(b) {
		copper[ly] = true
	}
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
		// A net repaints its own copper (agni issue 1543). A translucent stroke over a track of about
		// its width cannot be seen, so the default opacity draws the copper opaque.
		netAlpha := boardNetAlpha(spec.GetAlpha())
		shape := spec.GetShape()
		framed := shape == geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT || shape == geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_CIRCLE
		frame := func(e *highlightEntity) {
			if fb, ok := framedBounds(e, fr.entityFloor(e)); ok {
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
				drawTrack(c, s, fr, svg.A("stroke", color), svg.F("stroke-opacity", netAlpha),
					svg.F("stroke-width", copperStrokePx(s.GetWidth(), fr.scale)+highlightStrokePx),
					svg.A("stroke-linecap", "round"))
			}
			// A net's pours light as a tinted fill under an outline, so a plane reads as part of its
			// net without hiding the copper drawn on it (agni 1357).
			for _, sh := range nc.GetShapes() {
				c.El("path", svg.A("d", shapePathD(sh, fr)), svg.A("fill", color), svg.F("fill-opacity", netAlpha*0.35),
					svg.A("fill-rule", "evenodd"), svg.A("stroke", color), svg.F("stroke-opacity", netAlpha),
					svg.F("stroke-width", highlightStrokePx/2))
			}
			for _, v := range nc.GetVias() {
				if v.GetAt() == nil {
					continue
				}
				c.El("circle", svg.F("cx", fr.tx(v.GetAt().X)), svg.F("cy", fr.ty(v.GetAt().Y)),
					svg.F("r", math.Max(float64(v.GetSize())*fr.scale/2, strokePx)),
					svg.A("fill", color), svg.F("fill-opacity", netAlpha))
			}
		}

		for _, pl := range b.GetPlacements() {
			wholeComp := m.comps[pl.GetRefDes()]
			if framed && wholeComp {
				// The origin keeps a part with no pads and no art framed, as the origin dot marked it.
				e := &highlightEntity{shapes: art[pl.GetRefDes()], part: true}
				if pl.GetAt() != nil {
					e.pins = append(e.pins, pl.GetAt())
				}
				for _, pad := range pl.GetPads() {
					e.pins = append(e.pins, padCorners(pl, pad, stacks, copper)...)
				}
				frame(e)
			}
			for _, pad := range pl.GetPads() {
				onNet := pad.GetNet() != "" && m.matchWire(pad.GetNet(), netID(pad.GetNet()))
				onPin := m.pins[[2]string{pl.GetRefDes(), pad.GetNumber()}]
				if !wholeComp && !onNet && !onPin {
					continue
				}
				// In a framed spec, a part's or pin's frame stands in for the pad's ring. A pad on a
				// listed net keeps its ring too, as the net's other pads have one.
				if framed && !onNet {
					if !wholeComp {
						frame(&highlightEntity{pins: padCorners(pl, pad, stacks, copper)})
					}
					continue
				}
				// A pad on a lit net is filled over its lands, as its track and via are (agni issue 1543).
				if onNet {
					for _, land := range padLands(pl, pad, stacks, copper) {
						c.El("path", svg.A("d", ringD(land, fr)), svg.A("fill", color), svg.F("fill-opacity", netAlpha))
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

// padCorners returns the world points of a pad's lands, so a board frame bounds the same copper
// that WebGL bounds. A pad with a padstack has a land on each copper layer, which packStackLayer
// packs from its outline, its offset and its own size. A pad without one packs as packPad does,
// a turned rectangle or a disk. A pad with no size falls back to its centre.
func padCorners(pl *geom.ComponentPlacement, pad *geom.Pad, stacks stackIndex, copper map[string]bool) []*geom.Point {
	var out []*geom.Point
	for _, land := range padLands(pl, pad, stacks, copper) {
		out = append(out, land...)
	}
	if len(out) == 0 {
		wx, wy := padWorld(pl, pad)
		return []*geom.Point{{X: wx, Y: wy}}
	}
	return out
}

// padLands returns each of a pad's lands as a ring of world points, a padstack pad's one per copper
// layer, as padCorners bounds them. A pad with no size has no land.
func padLands(pl *geom.ComponentPlacement, pad *geom.Pad, stacks stackIndex, copper map[string]bool) [][]*geom.Point {
	wx, wy := padWorld(pl, pad)
	var out [][]*geom.Point
	if st := stacks.of(pad.GetPadstack()); st != nil {
		deg := geomath.PadAngle(pl, pad)
		for _, l := range st.ps.GetLayers() {
			p := l.GetPad()
			if p == nil || !copper[l.GetLayer()] {
				continue
			}
			cx, cy, ring := landAt(pl, pad, p)
			if ring == nil {
				ring = outlinePts(cx, cy, p.GetShape(), p.GetSize(), deg)
			}
			if len(ring) > 0 {
				out = append(out, ring)
			}
		}
		return out
	}
	if pad.GetSize().GetX() == 0 && pad.GetSize().GetY() == 0 {
		return nil
	}
	var pts [][2]int64
	if pad.GetShape() == "circle" {
		pts = diskPts(wx, wy, float64(pad.GetSize().GetX())/2)
	} else {
		pts = padQuadPts(wx, wy, pad.GetSize(), geomath.PadAngle(pl, pad))
	}
	ring := make([]*geom.Point, 0, len(pts))
	for _, p := range pts {
		ring = append(ring, &geom.Point{X: p[0], Y: p[1]})
	}
	return [][]*geom.Point{ring}
}

// ringD is an SVG path that closes a ring of world points in the sheet's frame.
func ringD(ring []*geom.Point, fr sheetFrame) string {
	var d strings.Builder
	for i, p := range ring {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&d, "%s%.1f %.1f ", cmd, fr.tx(p.GetX()), fr.ty(p.GetY()))
	}
	d.WriteString("Z")
	return d.String()
}

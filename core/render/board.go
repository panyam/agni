package render

import (
	"fmt"
	"math"
	"sort"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
	"github.com/panyam/agni/core/svg"
)

// boardTextMinPx is the legibility floor for silkscreen/fab text. Board text is authored at a
// true physical height and a board fits a lot of it into one view, so a floor keeps the smallest
// designators readable rather than letting them vanish. It is a board concern: schematic text
// sizes from labelFont, which floors relative to the drawing instead.
const boardTextMinPx = 6.0

// BoardSVG renders a board (the WS1-006 geometry sidecar) to a standalone SVG document —
// the board analogue of SheetSVG, and like it a verification/eyeball backend first (the
// packed/WebGL tier is WS7-035). Copper draws per layer back-to-front so the front side
// reads on top; every stratum goes into a classed <g> (edge / copper-front / copper-back /
// copper-inner / through / zones-* / labels), so layer visibility is a pure client-side
// CSS concern — no per-layer re-render, and highlight overlays (HighlightBoardSVG) share
// the exact frame.
//
// Coordinates are the sidecar's: nanometers, Y-up (the reader flips), placement/pad
// rotations verbatim from the source's Y-down frame — so composing them here negates the
// angle (see padWorld). Pad positions are footprint-local per the proto contract.
func BoardSVG(b *geom.BoardGeometry, opts ...Option) string {
	style := resolveStyle(opts)
	fr := frameBoard(b)
	tx, ty, scale := fr.tx, fr.ty, fr.scale

	c := svg.Open(fr.pxW, fr.pxH, svg.A("font-family", style.Font))
	c.El("rect", svg.F("x", 0), svg.F("y", 0), svg.F("width", fr.pxW), svg.F("height", fr.pxH), svg.A("fill", style.Page))

	// Zones first (authored outlines only — the sidecar carries no fill): a faint wash of
	// their layer's copper color, under everything.
	front, back, inner := splitByLayer(func(z *geom.Zone) string { return z.GetLayer() }, b.GetZones())
	for _, grp := range []struct {
		cls   string
		zones []*geom.Zone
	}{{"zones-back", back}, {"zones-inner", inner}, {"zones-front", front}} {
		if len(grp.zones) == 0 {
			continue
		}
		c.Group(svg.A("class", grp.cls))
		for _, z := range grp.zones {
			if z.GetOutline() == nil {
				continue
			}
			c.El("polygon", svg.A("points", points(z.GetOutline().Points, tx, ty)),
				svg.A("fill", copperColor(style, z.GetLayer())), svg.F("fill-opacity", 0.18), svg.A("stroke", "none"))
		}
		c.GroupEnd()
	}

	// Copper per layer, back-most first (higher KiCad layer number = further back).
	for _, ly := range copperLayersBackToFront(b) {
		c.Group(svg.A("class", "copper-"+layerSide(ly)), svg.A("data-layer", ly))
		color := copperColor(style, ly)
		for _, nc := range b.GetNets() {
			for _, s := range nc.GetSegments() {
				if s.GetLayer() != ly || s.GetA() == nil || s.GetB() == nil {
					continue
				}
				c.El("line", svg.F("x1", tx(s.GetA().X)), svg.F("y1", ty(s.GetA().Y)),
					svg.F("x2", tx(s.GetB().X)), svg.F("y2", ty(s.GetB().Y)),
					svg.A("stroke", color), svg.F("stroke-width", copperStrokePx(s.GetWidth(), scale)),
					svg.A("stroke-linecap", "round"))
			}
		}
		// Pads that live only on this layer (SMD); through-hole pads draw once, below.
		for _, pl := range b.GetPlacements() {
			for _, pad := range pl.GetPads() {
				if padSide(pad) == layerSide(ly) {
					drawPad(c, pl, pad, fr, color, style)
				}
			}
		}
		c.GroupEnd()
	}

	// Through copper: vias and through-hole pads, visible from both sides.
	c.Group(svg.A("class", "through"))
	for _, nc := range b.GetNets() {
		for _, v := range nc.GetVias() {
			if v.GetAt() == nil {
				continue
			}
			cx, cy := tx(v.GetAt().X), ty(v.GetAt().Y)
			c.El("circle", svg.F("cx", cx), svg.F("cy", cy), svg.F("r", math.Max(float64(v.GetSize())*scale/2, strokePx)),
				svg.A("fill", style.Via))
			c.El("circle", svg.F("cx", cx), svg.F("cy", cy), svg.F("r", math.Max(float64(v.GetDrill())*scale/2, strokePx/2)),
				svg.A("fill", style.Page))
		}
	}
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if padSide(pad) == "through" {
				drawPad(c, pl, pad, fr, style.Via, style)
			}
		}
	}
	c.GroupEnd()

	// Board outline (Edge.Cuts) above the copper, so the physical edge always reads.
	c.Group(svg.A("class", "edge"))
	for _, p := range b.GetOutline().GetPaths() {
		c.El("polyline", svg.A("fill", "none"), svg.A("stroke", style.BoardOutline),
			svg.F("stroke-width", strokePx*1.5), svg.A("points", points(p.Points, tx, ty)))
	}
	c.GroupEnd()

	// Silkscreen / fab body graphics (component outlines, courtyards, polarity marks), under
	// the text so labels stay legible. Each element carries its source layer as data-layer so
	// per-layer visibility is the same client-side CSS concern as the copper strata.
	if len(b.GetGraphics()) > 0 {
		c.Group(svg.A("class", "silk"))
		for _, gr := range b.GetGraphics() {
			if gr.GetShape() == nil {
				continue
			}
			w := math.Max(float64(gr.GetWidth())*scale, strokePx)
			c.Group(svg.A("data-layer", gr.GetLayer()))
			writeShape(c, gr.GetShape(), tx, ty, scale, w, style, style.Silk)
			c.GroupEnd()
		}
		c.GroupEnd()
	}

	// Silkscreen / legend text: the ref-des, value, and free (title-block) text the reader
	// composed to board coordinates. Rotation negates for the Y-down screen exactly like a
	// pad; anchor and font-size follow the authored justify and height.
	c.Group(svg.A("class", "labels"))
	for _, t := range b.GetTexts() {
		if t.GetAt() == nil {
			continue
		}
		x, y := tx(t.GetAt().X), ty(t.GetAt().Y)
		fontPx := math.Max(float64(boardTextHeight(t))*scale, boardTextMinPx)
		attrs := []svg.Attr{
			svg.F("x", x), svg.F("y", y), svg.A("fill", style.Field), svg.F("font-size", fontPx),
			svg.A("text-anchor", svgAnchor(t)), svg.A("dominant-baseline", "central"),
		}
		if rot := -t.GetRotationDeg(); rot != 0 {
			attrs = append(attrs, svg.A("transform", fmt.Sprintf("rotate(%.1f %.1f %.1f)", rot, x, y)))
		}
		c.Text(boardTextLine(t), attrs...)
	}
	c.GroupEnd()

	return c.String()
}

// boardTextLine flattens a board text's authored string to one line (newlines, escaped or
// literal, become spaces) for the single-line label backends.
func boardTextLine(t *geom.BoardText) string {
	return strings.NewReplacer("\n", " ", `\n`, " ").Replace(t.GetText())
}

// boardTextHeight returns a text's glyph height in source units, falling back to 1mm when
// the source carried no font size.
func boardTextHeight(t *geom.BoardText) int64 {
	if h := t.GetHeight(); h > 0 {
		return h
	}
	return 1_000_000
}

// justifyWord maps a board text's horizontal justify to the anchor word the packed overlay
// keys on ("center" when the source left it unset).
func justifyWord(t *geom.BoardText) string {
	if j := t.GetJustify(); j != "" {
		return j
	}
	return "center"
}

// svgAnchor maps a board text's horizontal justify to an SVG text-anchor ("middle" default).
func svgAnchor(t *geom.BoardText) string {
	switch t.GetJustify() {
	case "left":
		return "start"
	case "right":
		return "end"
	default:
		return "middle"
	}
}

// frameBoard maps board nanometers into the shared pixel space, bounding over the outline,
// copper, pads (world positions), and zones — the same sheetFrame contract SheetSVG uses,
// so highlight overlays composite exactly.
func frameBoard(b *geom.BoardGeometry) sheetFrame {
	var bounds geomath.Bounds
	for _, p := range b.GetOutline().GetPaths() {
		for _, pt := range p.Points {
			bounds.Add(pt)
		}
	}
	for _, nc := range b.GetNets() {
		for _, s := range nc.GetSegments() {
			bounds.Add(s.GetA())
			bounds.Add(s.GetB())
		}
		for _, v := range nc.GetVias() {
			bounds.Add(v.GetAt())
		}
	}
	for _, pl := range b.GetPlacements() {
		bounds.Add(pl.GetAt())
		for _, pad := range pl.GetPads() {
			wx, wy := padWorld(pl, pad)
			bounds.Add(&geom.Point{X: wx, Y: wy})
		}
	}
	for _, z := range b.GetZones() {
		for _, pt := range z.GetOutline().GetPoints() {
			bounds.Add(pt)
		}
	}
	for _, t := range b.GetTexts() {
		bounds.Add(t.GetAt())
	}
	for _, gr := range b.GetGraphics() {
		for _, p := range gr.GetShape().GetPoints() {
			bounds.Add(p)
		}
	}
	if !bounds.Valid() {
		bounds.Add(&geom.Point{X: 0, Y: 0})
		bounds.Add(&geom.Point{X: 1000, Y: 1000})
	}
	bMinX, bMinY := bounds.Min()
	bMaxX, bMaxY := bounds.Max()
	spanX, spanY := bMaxX-bMinX, bMaxY-bMinY
	scale := sheetMaxPx / float64(max(max(spanX, spanY), 1))
	return sheetFrame{
		tx:    func(x int64) float64 { return float64(x-bMinX)*scale + sheetMarginPx },
		ty:    func(y int64) float64 { return float64(bMaxY-y)*scale + sheetMarginPx },
		scale: scale,
		pxW:   float64(spanX)*scale + 2*sheetMarginPx,
		pxH:   float64(spanY)*scale + 2*sheetMarginPx,
	}
}

// padWorld composes a pad's footprint-local offset with its placement in the canonical Y-up
// frame (WS1-030): world = at + M(R(rotation_deg) * pad_offset), where R rotates CCW and M
// mirrors X iff the placement is on the back (mirror after rotation, so a back part is its
// front footprint rotated then reflected). Each reader delivers rotation_deg already in this
// frame (a Y-down source negates it on import), so the renderer never remaps — it is a pure
// composer shared by every board producer.
func padWorld(pl *geom.ComponentPlacement, pad *geom.Pad) (int64, int64) {
	p := geomath.ComposePlacement(pl.GetAt(), pl.GetRotationDeg(), pl.GetMirror(), pad.GetAt())
	return p.X, p.Y
}

// drawPad draws one pad at its world position: rect/roundrect/oval as a (rounded)
// rectangle, circle as a circle, rotated by the pad's own verbatim angle (KiCad stores it
// cumulative with the footprint, so it is NOT composed with the placement again — only
// negated for the Y-flip). A drilled pad gets a page-colored hole.
func drawPad(c *svg.Canvas, pl *geom.ComponentPlacement, pad *geom.Pad, fr sheetFrame, color string, style Style) {
	wx, wy := padWorld(pl, pad)
	cx, cy := fr.tx(wx), fr.ty(wy)
	w := math.Max(float64(pad.GetSize().GetX())*fr.scale, strokePx)
	h := math.Max(float64(pad.GetSize().GetY())*fr.scale, strokePx)
	rot := -pad.GetRotationDeg()
	attrs := []svg.Attr{svg.A("fill", color)}
	if rot != 0 {
		attrs = append(attrs, svg.A("transform", fmt.Sprintf("rotate(%.1f %.1f %.1f)", rot, cx, cy)))
	}
	switch pad.GetShape() {
	case "circle":
		c.El("circle", append([]svg.Attr{svg.F("cx", cx), svg.F("cy", cy), svg.F("r", w/2)}, attrs...)...)
	case "oval":
		attrs = append(attrs, svg.F("rx", math.Min(w, h)/2))
		c.El("rect", append([]svg.Attr{svg.F("x", cx-w/2), svg.F("y", cy-h/2), svg.F("width", w), svg.F("height", h)}, attrs...)...)
	case "roundrect":
		attrs = append(attrs, svg.F("rx", math.Min(w, h)*0.25))
		c.El("rect", append([]svg.Attr{svg.F("x", cx-w/2), svg.F("y", cy-h/2), svg.F("width", w), svg.F("height", h)}, attrs...)...)
	default: // "rect" and anything unknown
		c.El("rect", append([]svg.Attr{svg.F("x", cx-w/2), svg.F("y", cy-h/2), svg.F("width", w), svg.F("height", h)}, attrs...)...)
	}
	if pad.GetDrill() > 0 {
		c.El("circle", svg.F("cx", cx), svg.F("cy", cy), svg.F("r", math.Max(float64(pad.GetDrill())*fr.scale/2, strokePx/2)),
			svg.A("fill", style.Page))
	}
}

// padSide classifies a pad for layer grouping: "front"/"back" for single-side (SMD) pads,
// "through" when its layers include a wildcard or both copper sides.
func padSide(pad *geom.Pad) string {
	frontish, backish := false, false
	for _, l := range pad.GetLayers() {
		switch {
		case strings.HasPrefix(l, "*."):
			return "through"
		case strings.HasPrefix(l, "F."):
			frontish = true
		case strings.HasPrefix(l, "B."):
			backish = true
		}
	}
	if frontish && backish {
		return "through"
	}
	if backish {
		return "back"
	}
	return "front"
}

// layerSide maps a copper layer name to its visibility bucket.
func layerSide(layer string) string {
	switch {
	case layer == "F.Cu":
		return "front"
	case layer == "B.Cu":
		return "back"
	default:
		return "inner"
	}
}

// copperStrokePx is a copper trace's rendered stroke width: its true width floored to the
// physical minStrokeNm minimum, then scaled to output pixels. Unlike a fixed output-pixel
// floor, this keeps rendered thickness proportional to the actual copper, so a dense board's
// fine traces stay thin and legible instead of clamping to one width and merging. It matches
// the WebGL packer's quadPts, so both renderers draw copper at identical width.
func copperStrokePx(widthNm int64, scale float64) float64 {
	return math.Max(float64(widthNm), minStrokeNm) * scale
}

// copperColor picks the layer's copper color from the style.
func copperColor(s Style, layer string) string {
	switch layerSide(layer) {
	case "front":
		return s.CopperFront
	case "back":
		return s.CopperBack
	default:
		return s.CopperInner
	}
}

// copperLayersBackToFront lists the signal layers that actually carry drawn geometry,
// ordered so the back-most draws first (KiCad numbers the front 0 and the back highest).
func copperLayersBackToFront(b *geom.BoardGeometry) []string {
	num := map[string]int32{}
	for _, l := range b.GetLayers() {
		if l.GetKind() == "signal" {
			num[l.GetName()] = l.GetNumber()
		}
	}
	used := map[string]bool{}
	for _, nc := range b.GetNets() {
		for _, s := range nc.GetSegments() {
			used[s.GetLayer()] = true
		}
	}
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			side := padSide(pad)
			if side == "front" {
				used["F.Cu"] = true
			} else if side == "back" {
				used["B.Cu"] = true
			}
		}
	}
	out := make([]string, 0, len(used))
	for l := range used {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		ni, iOK := num[out[i]]
		nj, jOK := num[out[j]]
		if iOK != jOK {
			return !iOK // unknown layers first (drawn under the known stack)
		}
		if ni != nj {
			return ni > nj // higher number = further back = drawn first
		}
		return out[i] < out[j]
	})
	return out
}

// splitByLayer buckets items into front/back/inner by a layer accessor.
func splitByLayer[T any](layerOf func(T) string, items []T) (front, back, inner []T) {
	for _, it := range items {
		switch layerSide(layerOf(it)) {
		case "front":
			front = append(front, it)
		case "back":
			back = append(back, it)
		default:
			inner = append(inner, it)
		}
	}
	return front, back, inner
}

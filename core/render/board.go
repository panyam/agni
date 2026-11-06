package render

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/svg"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
)

// boardTextMinPx is the legibility floor for silkscreen/fab text. Board text is authored at a
// true physical height and a board fits a lot of it into one view, so without a floor the smallest
// designators vanish. Schematic text sizes from labelFont, which floors relative to the drawing.
const boardTextMinPx = 6.0

// BoardSVG renders a board to a standalone SVG document. The board is the WS1-006 geometry
// sidecar, and BoardSVG is the board analogue of SheetSVG. WS7-035 is the packed/WebGL tier.
// Copper draws per layer back-to-front so the front side reads on top. Every stratum goes into a
// classed <g> (edge, copper-front, copper-back, copper-inner, through, zones-*, silk, labels), so
// client-side CSS sets layer visibility with no re-render. HighlightBoardSVG overlays share the
// same frame.
//
// Coordinates are the sidecar's, in nanometers and Y-up, because the reader flips them. Rotations
// arrive already in the Y-up frame (see padWorld), and BoardSVG negates them only for the SVG
// transform. Pad positions are footprint-local per the proto contract.
func BoardSVG(b *geom.BoardGeometry, opts ...Option) string {
	style := resolveStyle(opts)
	fr := frameBoard(b)
	tx, ty, scale := fr.tx, fr.ty, fr.scale
	pk := boardPickKeys{on: style.PickTargets}
	stacks := padstacksOf(b)
	planes := planesOf(b)

	c := svg.Open(fr.pxW, fr.pxH, svg.A("font-family", style.Font))
	c.El("rect", svg.F("x", 0), svg.F("y", 0), svg.F("width", fr.pxW), svg.F("height", fr.pxH), svg.A("fill", style.Page))

	// Zones first, as a faint wash of their layer's copper color under everything. The sidecar
	// carries authored outlines only, no fill.
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
			// data-layer lets the zone hide with its copper layer as well as with the Zones row.
			c.El("polygon", append([]svg.Attr{svg.A("points", points(z.GetOutline().Points, tx, ty)), svg.A("data-layer", z.GetLayer()),
				svg.A("fill", copperColor(style, z.GetLayer())), svg.F("fill-opacity", 0.18), svg.A("stroke", "none")},
				pk.net(z.GetNet())...)...)
		}
		c.GroupEnd()
	}

	// Each footprint's body is an invisible pick rect over its pads, above the zones and under the
	// copper, so a pad or a track drawn over the body still wins where it is drawn.
	if style.PickTargets {
		drawFootprintPicks(c, b, fr)
	}

	// Copper per layer, back-most first (higher KiCad layer number = further back).
	for _, ly := range copperLayersBackToFront(b) {
		c.Group(svg.A("class", "copper-"+layerSide(ly)), svg.A("data-layer", ly))
		color := copperColor(style, ly)
		// Filled copper first, so the tracks and lands on it draw over it (agni 1357).
		for _, nc := range b.GetNets() {
			for _, sh := range nc.GetShapes() {
				if sh.GetLayer() == ly {
					c.El("path", append([]svg.Attr{svg.A("class", "pour"), svg.A("d", shapePathD(sh, fr)),
						svg.A("fill", color), svg.A("fill-rule", "evenodd")}, pk.net(nc.GetNet())...)...)
				}
			}
		}
		for _, nc := range b.GetNets() {
			// A net's tracks on this layer share one keyed group rather than a key per line. Tracks
			// are most of a board's elements, and a key on each grew Jetson's board by a third.
			keys, grouped := pk.net(nc.GetNet()), false
			for _, s := range nc.GetSegments() {
				if s.GetLayer() != ly || s.GetA() == nil || s.GetB() == nil {
					continue
				}
				if len(keys) > 0 && !grouped {
					c.Group(keys...)
					grouped = true
				}
				drawTrack(c, s, fr, svg.A("stroke", color), svg.F("stroke-width", copperStrokePx(s.GetWidth(), scale)),
					svg.A("stroke-linecap", "round"))
			}
			if grouped {
				c.GroupEnd()
			}
		}
		// Pads that live only on this layer (SMD); through-hole pads draw once, below. A pad or via
		// with a padstack draws its land on each layer instead, and its antipad where the layer is
		// a plane it does not join (agni 1356).
		for _, pl := range b.GetPlacements() {
			for _, pad := range pl.GetPads() {
				if st := stacks.of(pad.GetPadstack()); st != nil {
					if l := st.on(ly); l != nil {
						drawStackLayer(c, pl, pad, l, fr, color, planes.cuts(ly, pad.GetNet()), pk.pad(pl, pad)...)
					}
					continue
				}
				if padSide(pad) == layerSide(ly) {
					drawPad(c, pl, pad, fr, color, style, pk.pad(pl, pad)...)
				}
			}
		}
		for _, nc := range b.GetNets() {
			for _, v := range nc.GetVias() {
				if st := stacks.of(v.GetPadstack()); st != nil && v.GetAt() != nil {
					if l := st.on(ly); l != nil {
						at := &geom.Pad{At: v.GetAt()}
						drawStackLayer(c, &geom.ComponentPlacement{At: &geom.Point{}}, at, l, fr, color, planes.cuts(ly, nc.GetNet()), pk.net(nc.GetNet())...)
					}
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
			// The drill hole draws over the ring, so it carries the net too, or a click in it misses.
			keys := pk.net(nc.GetNet())
			if stacks.of(v.GetPadstack()) != nil {
				// The lands are on their layers, so the through copper is the plated barrel alone.
				drawBarrel(c, cx, cy, float64(v.GetDrill())*scale, style, keys...)
				continue
			}
			c.El("circle", append([]svg.Attr{svg.F("cx", cx), svg.F("cy", cy), svg.F("r", math.Max(float64(v.GetSize())*scale/2, strokePx)),
				svg.A("fill", style.Via)}, keys...)...)
			c.El("circle", append([]svg.Attr{svg.F("cx", cx), svg.F("cy", cy), svg.F("r", math.Max(float64(v.GetDrill())*scale/2, strokePx/2)),
				svg.A("fill", style.Page)}, keys...)...)
		}
	}
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if stacks.of(pad.GetPadstack()) != nil {
				if pad.GetDrill() > 0 {
					wx, wy := padWorld(pl, pad)
					drawBarrel(c, tx(wx), ty(wy), float64(pad.GetDrill())*scale, style, pk.pad(pl, pad)...)
				}
				continue
			}
			if padSide(pad) == "through" {
				drawPad(c, pl, pad, fr, style.Via, style, pk.pad(pl, pad)...)
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
	// the text so labels remain legible. Each element records its source layer in data-layer, so
	// client-side CSS sets per-layer visibility here as it does for the copper strata.
	if len(b.GetGraphics()) > 0 {
		c.Group(append([]svg.Attr{svg.A("class", "silk")}, pk.passThrough()...)...)
		for _, gr := range b.GetGraphics() {
			if gr.GetShape() == nil {
				continue
			}
			w := math.Max(float64(gr.GetWidth())*scale, strokePx)
			c.Group(svg.A("data-layer", gr.GetLayer()))
			writeShape(c, gr.GetShape(), tx, ty, scale, w, style, boardArtColor(style, gr.GetLayer()))
			c.GroupEnd()
		}
		c.GroupEnd()
	}

	// Silkscreen and legend text (ref-des, value, free title-block text) in board coordinates.
	// Rotation negates for the Y-down screen like a pad, and anchor and font-size follow the
	// authored justify and height.
	c.Group(append([]svg.Attr{svg.A("class", "labels")}, pk.passThrough()...)...)
	for _, t := range boardLabels(b) {
		if t.GetAt() == nil {
			continue
		}
		x, y := tx(t.GetAt().X), ty(t.GetAt().Y)
		fontPx := math.Max(float64(boardTextEm(t))*scale, boardTextMinPx)
		attrs := []svg.Attr{
			svg.F("x", x), svg.F("y", y), svg.A("fill", boardTextColor(style, t.GetLayer())), svg.F("font-size", fontPx),
			svg.A("text-anchor", svgAnchor(t)), svg.A("dominant-baseline", "central"),
		}
		if tf := boardTextTransform(-t.GetRotationDeg(), t.GetMirror(), x, y); tf != "" {
			attrs = append(attrs, svg.A("transform", tf))
		}
		// A text on a layer hides with it. A designator the renderer added has no layer and always draws.
		if t.GetLayer() != "" {
			attrs = append(attrs, svg.A("data-layer", t.GetLayer()))
		}
		c.Text(boardTextLine(t), attrs...)
	}
	c.GroupEnd()

	return c.String()
}

// boardTextTransform turns a board text about its anchor, and mirrors it left to right first when
// the text is mirrored, as back-side text is. KiCad draws back text mirrored, the way it sits on the
// copper seen from the front, so a view turned over to the bottom reads it the right way round
// (agni issue 1327, #142). Upright, unmirrored text needs no transform.
func boardTextTransform(rot float64, mirror bool, x, y float64) string {
	var parts []string
	if rot != 0 {
		parts = append(parts, fmt.Sprintf("rotate(%.1f %.1f %.1f)", rot, x, y))
	}
	if mirror {
		parts = append(parts, fmt.Sprintf("translate(%.1f %.1f) scale(-1 1) translate(%.1f %.1f)", x, y, -x, -y))
	}
	return strings.Join(parts, " ")
}

// boardLabels returns the text a board view draws. A board with text of its own gets that text.
// A board may also draw no designator at all, with no reference text and no silkscreen. For that
// board boardLabels makes each placement's designator, centred above its pads. An ODB++ job holds
// only what its silkscreen draws, and KiCad exports a board with no silkscreen with no text at
// all. Without the added designators those parts would go unlabelled, while the same board read
// from its .kicad_pcb is labelled (agni issue 1263). A board whose silkscreen draws its
// designators gets none added, as in the Altium case.
func boardLabels(b *geom.BoardGeometry) []*geom.BoardText {
	if len(b.GetTexts()) > 0 {
		return b.GetTexts()
	}
	for _, gr := range b.GetGraphics() {
		if strings.HasSuffix(gr.GetLayer(), ".SilkS") {
			return nil
		}
	}
	var out []*geom.BoardText
	for _, pl := range b.GetPlacements() {
		if pl.GetRefDes() == "" || pl.GetAt() == nil {
			continue
		}
		x, top := pl.GetAt().X, pl.GetAt().Y
		if len(pl.GetPads()) > 0 {
			minX, maxX := int64(math.MaxInt64), int64(math.MinInt64)
			top = math.MinInt64
			for _, pad := range pl.GetPads() {
				wx, wy := padWorld(pl, pad)
				half := max(pad.GetSize().GetX(), pad.GetSize().GetY()) / 2
				minX, maxX = min(minX, wx), max(maxX, wx)
				top = max(top, wy+half)
			}
			x = (minX + maxX) / 2
		}
		out = append(out, &geom.BoardText{
			Text: pl.GetRefDes(), At: &geom.Point{X: x, Y: top + boardDesignatorGap},
			RefDes: pl.GetRefDes(), Kind: "reference",
		})
	}
	return out
}

// boardDesignatorGap is how far above its pads a placement's own designator sits, 1 mm.
const boardDesignatorGap = 1_000_000

// boardTextLine flattens a board text's authored string to one line (newlines, escaped or
// literal, become spaces) for the single-line label backends.
func boardTextLine(t *geom.BoardText) string {
	return strings.NewReplacer("\n", " ", `\n`, " ").Replace(t.GetText())
}

// capHeightEm is a capital letter's height as a fraction of the font size in SchematicFontStack's
// faces (Arial's is 1467 of 2048 units). A board text's size is its glyph height, as KiCad draws it,
// and an SVG font-size is the em, so the renderers draw the em at height/capHeightEm. Drawn as the
// em, a 1 mm designator came out about 0.72 mm tall (agni issue 1366).
const capHeightEm = 0.716

// boardTextEm is the font size, in source units, that draws a board text's glyphs at its height.
func boardTextEm(t *geom.BoardText) int64 {
	return int64(math.Round(float64(boardTextHeight(t)) / capHeightEm))
}

// boardTextColor is the colour a board text draws in, its layer's, as KiCad colours text by layer.
// Silkscreen text draws in the silk colour and fab text in the fab colour. Text on any other layer,
// or none, such as a designator added for a board with no silkscreen, draws in the field colour.
func boardTextColor(s Style, layer string) string {
	switch {
	case strings.HasSuffix(layer, ".SilkS"):
		return s.Silk
	case strings.HasSuffix(layer, ".Fab"):
		return s.Fab
	}
	return s.Field
}

// boardArtColor is the colour a footprint graphic draws in, the fab colour on a fab layer and the
// silk colour otherwise (silkscreen and courtyard).
func boardArtColor(s Style, layer string) string {
	if strings.HasSuffix(layer, ".Fab") {
		return s.Fab
	}
	return s.Silk
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

// nearOutline reports whether a point lies within the board outline's extent, grown by a fifth of
// its larger side, or answers true for every point on a board with no outline. Designers park a
// label they have no room for well off the board (29 ref-des on the BeagleBone AI sit 300 mm away),
// and framing on it shrinks the board to a sliver while the text floor keeps the labels large.
func nearOutline(b *geom.BoardGeometry) func(*geom.Point) bool {
	var ob geomath.Bounds
	for _, p := range b.GetOutline().GetPaths() {
		for _, pt := range p.Points {
			ob.Add(pt)
		}
	}
	if !ob.Valid() {
		return func(*geom.Point) bool { return true }
	}
	x0, y0 := ob.Min()
	x1, y1 := ob.Max()
	m := max(x1-x0, y1-y0) / 5
	return func(p *geom.Point) bool {
		return p.GetX() >= x0-m && p.GetX() <= x1+m && p.GetY() >= y0-m && p.GetY() <= y1+m
	}
}

// frameBoard maps board nanometers into the shared pixel space, bounding over the outline,
// copper, pads (world positions), zones, texts and graphics. It returns a sheetFrame, as SheetSVG
// does, so highlight overlays composite exactly.
func frameBoard(b *geom.BoardGeometry) sheetFrame {
	var bounds geomath.Bounds
	for _, p := range b.GetOutline().GetPaths() {
		for _, pt := range p.Points {
			bounds.Add(pt)
		}
	}
	for _, nc := range b.GetNets() {
		for _, s := range nc.GetSegments() {
			for _, pt := range geomath.TrackPath(s) {
				bounds.Add(pt)
			}
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
	near := nearOutline(b)
	for _, t := range b.GetTexts() {
		if near(t.GetAt()) {
			bounds.Add(t.GetAt())
		}
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
// frame (WS1-030). geomath.ComposePlacement holds the transform, the mirror order and the frame
// contract, and every board producer calls it.
// drawTrack draws one track segment, a line for a straight run and an SVG arc path for an arc. The
// path draws the arc as two halves that meet at its mid point, each with its own large-arc flag.
// ty flips Y, so an arc that turns counter-clockwise in the Y-up frame turns the negative way in
// SVG's Y-down frame, which is sweep flag 0.
func drawTrack(c *svg.Canvas, s *geom.TrackSegment, fr sheetFrame, attrs ...svg.Attr) {
	if s.GetMid() != nil {
		if a, ok := geomath.ArcThrough(s.GetA(), s.GetMid(), s.GetB()); ok {
			c.El("path", append([]svg.Attr{svg.A("d", arcPathD(s, a, fr)), svg.A("fill", "none")}, attrs...)...)
			return
		}
	}
	for i, pts := 1, geomath.TrackPath(s); i < len(pts); i++ {
		c.El("line", append([]svg.Attr{svg.F("x1", fr.tx(pts[i-1].X)), svg.F("y1", fr.ty(pts[i-1].Y)),
			svg.F("x2", fr.tx(pts[i].X)), svg.F("y2", fr.ty(pts[i].Y))}, attrs...)...)
	}
}

func arcPathD(s *geom.TrackSegment, a geomath.Arc, fr sheetFrame) string {
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
	pt := func(p *geom.Point) string { return f(fr.tx(p.X)) + " " + f(fr.ty(p.Y)) }
	r := f(a.R * fr.scale)
	half := func(turn float64, to *geom.Point) string {
		large, sweep := "0", "0"
		if math.Abs(turn) > math.Pi {
			large = "1"
		}
		if turn < 0 {
			sweep = "1"
		}
		return " A " + r + " " + r + " 0 " + large + " " + sweep + " " + pt(to)
	}
	h1 := a.TurnTo(float64(s.GetMid().X), float64(s.GetMid().Y))
	return "M " + pt(s.GetA()) + half(h1, s.GetMid()) + half(a.Sweep-h1, s.GetB())
}

func padWorld(pl *geom.ComponentPlacement, pad *geom.Pad) (int64, int64) {
	p := geomath.ComposePlacement(pl.GetAt(), pl.GetRotationDeg(), pl.GetMirror(), pad.GetAt())
	return p.X, p.Y
}

// drawPad draws one pad at its world position, rect/roundrect/oval as a (rounded) rectangle and
// circle as a circle. It turns by geomath.PadAngle, the pad's footprint-frame angle composed with
// its placement (agni 1407), negated for the Y-flip. A drilled pad gets a page-colored hole.
func drawPad(c *svg.Canvas, pl *geom.ComponentPlacement, pad *geom.Pad, fr sheetFrame, color string, style Style, keys ...svg.Attr) {
	drawPadWith(c, pl, pad, fr, append([]svg.Attr{svg.A("fill", color)}, keys...)...)
	if pad.GetDrill() > 0 {
		wx, wy := padWorld(pl, pad)
		cx, cy := fr.tx(wx), fr.ty(wy)
		c.El("circle", append([]svg.Attr{svg.F("cx", cx), svg.F("cy", cy), svg.F("r", math.Max(float64(pad.GetDrill())*fr.scale/2, strokePx/2)),
			svg.A("fill", style.Page)}, keys...)...)
	}
}

// drawPadWith draws a pad's outline shape with the caller's paint, a fill for a land or a stroke
// for an antipad.
func drawPadWith(c *svg.Canvas, pl *geom.ComponentPlacement, pad *geom.Pad, fr sheetFrame, attrs ...svg.Attr) {
	wx, wy := padWorld(pl, pad)
	drawShapeAt(c, wx, wy, pad.GetShape(), pad.GetSize(), geomath.PadAngle(pl, pad), fr, attrs...)
}

// drawShapeAt draws a pad shape centred on a board point, turned deg degrees CCW on the board.
func drawShapeAt(c *svg.Canvas, wx, wy int64, shape string, size *geom.Point, deg float64, fr sheetFrame, attrs ...svg.Attr) {
	cx, cy := fr.tx(wx), fr.ty(wy)
	w := math.Max(float64(size.GetX())*fr.scale, strokePx)
	h := math.Max(float64(size.GetY())*fr.scale, strokePx)
	rot := -deg
	if deg > 180 {
		rot = 360 - deg
	}
	if rot != 0 {
		attrs = append(attrs, svg.A("transform", fmt.Sprintf("rotate(%.1f %.1f %.1f)", rot, cx, cy)))
	}
	switch shape {
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

// copperStrokePx returns a copper trace's rendered stroke width. It floors the true width to the
// physical minStrokeNm and then scales it to output pixels. A floor in board space keeps a dense
// board's fine traces proportional instead of clamping to one pixel width and merging. It matches
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
		for _, sh := range nc.GetShapes() {
			used[sh.GetLayer()] = true
		}
	}
	stacks := padstacksOf(b)
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if st := stacks.of(pad.GetPadstack()); st != nil {
				for _, ly := range st.layers() {
					used[ly] = true
				}
				continue
			}
			side := padSide(pad)
			if side == "front" {
				used["F.Cu"] = true
			} else if side == "back" {
				used["B.Cu"] = true
			}
		}
	}
	for _, nc := range b.GetNets() {
		for _, v := range nc.GetVias() {
			if st := stacks.of(v.GetPadstack()); st != nil {
				for _, ly := range st.layers() {
					used[ly] = true
				}
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

// shapePathD is a copper shape's outline and holes as one SVG path, each ring a closed subpath, for
// an even-odd fill that leaves the holes empty.
func shapePathD(sh *geom.CopperShape, fr sheetFrame) string {
	var d strings.Builder
	ring := func(pl *geom.Polyline) {
		for i, p := range pl.GetPoints() {
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&d, "%s%.1f %.1f ", cmd, fr.tx(p.GetX()), fr.ty(p.GetY()))
		}
		d.WriteString("Z ")
	}
	ring(sh.GetOutline())
	for _, h := range sh.GetHoles() {
		ring(h)
	}
	return strings.TrimSpace(d.String())
}

package render

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
	"github.com/panyam/agni/core/svg"
)

// SheetSVG renders one schematic sheet to a standalone SVG document, resolving each
// placement against the design's symbol library. It and the WebGL2 path (WS7-001) are both
// backends over the same render layer, so nothing here is WebGL- or format-bound. Coordinates
// are framed by frameSheet, which normalizes them to pixels and flips Y.
func SheetSVG(g *geom.SchematicGeometry, sheet *geom.SheetGeometry, opts ...Option) string {
	style := resolveStyle(opts)
	syms := indexSymbols(g)
	fr := frameSheet(sheet, syms)
	c := svg.Open(fr.pxW, fr.pxH, svg.A("font-family", style.Font))
	c.El("rect", svg.F("x", 0), svg.F("y", 0), svg.F("width", fr.pxW), svg.F("height", fr.pxH), svg.A("fill", style.Page))
	drawSheetContent(c, g, sheet, syms, fr, style)
	return c.String()
}

// drawSheetContent draws one sheet's schematic body (worksheet, wires, symbols, labels) onto an
// already-opened canvas. The caller owns svg.Open and the page rect, so SheetSVGHighlighted can
// composite the body and its highlight overlay onto ONE canvas with one frame.
func drawSheetContent(c *svg.Canvas, g *geom.SchematicGeometry, sheet *geom.SheetGeometry, syms map[string]*geom.SymbolDef, fr sheetFrame, style Style) {
	tx, ty, scale := fr.tx, fr.ty, fr.scale
	// The size for text that states no height, shared with the WebGL path (see labelFont).
	def := defaultTextHeight(g, sheet)

	// Worksheet frame and title block, under the schematic, when the sheet has a page.
	drawWorksheet(c, g, sheet, tx, ty, style)

	// Sheet-level raster images (logos, notes), under the schematic.
	for _, im := range sheet.Images {
		drawImage(c, im, tx, ty)
	}

	// Wires first, under symbols. A bus trunk or entry (WS7-042) draws thicker in the bus color,
	// and an unset kind is a plain wire.
	for _, wire := range sheet.Wires {
		stroke, width := style.Wire, strokePx
		switch wire.GetKind() {
		case geom.WireGeometry_KIND_BUS, geom.WireGeometry_KIND_BUS_ENTRY:
			stroke, width = style.Bus, busStrokePx
		}
		keys := wireKeys(wire)
		for _, pl := range wire.Polylines {
			pts := points(pl.Points, tx, ty)
			c.El("polyline", append([]svg.Attr{svg.A("fill", "none"), svg.A("stroke", stroke),
				svg.F("stroke-width", width), svg.A("points", pts)}, keys...)...)
			if style.PickTargets {
				// A 0.8px fill:none stroke hit-tests only ON the stroke, and in a real browser a
				// probe at a wire's own midpoint, rounded to whole pixels, hits the page rect. So
				// the viewer gets an invisible wide companion. It stays in the wire pass, under the
				// symbols, so a wire crossing beneath a part still loses to the part. See
				// docsite/content/architecture/web-picking.md.
				c.El("polyline", append([]svg.Attr{svg.A("fill", "none"), svg.A("stroke", "none"),
					svg.F("stroke-width", wirePickWidthPx), svg.A("pointer-events", "stroke"),
					svg.A("points", pts)}, keys...)...)
			}
		}
	}

	// Free sheet graphics (junction dots, no-connect markers, notes).
	for _, s := range sheet.Shapes {
		writeShape(c, s, tx, ty, scale, strokePx, style, style.Free)
	}

	// Symbol graphics.
	for _, pl := range sheet.Placements {
		keys := symbolKeys(pl)
		for _, s := range placedShapes(syms, pl) {
			writeShape(c, s, tx, ty, scale, strokePx, style, style.Symbol, keys...)
		}
	}

	// Pin number and name labels, and symbol annotations. The per-pin connect dot draws only
	// under PinDots (WS7-017).
	for _, pl := range sheet.Placements {
		sym := symbolFor(syms, pl)
		if sym == nil {
			continue
		}
		rot := placementRotation(pl.Transform)
		for _, pin := range sym.Pins {
			wp := geomath.PlacePin(pl.Transform, pin)
			if style.PinDots {
				c.El("circle", append([]svg.Attr{svg.F("cx", tx(wp.X)), svg.F("cy", ty(wp.Y)), svg.F("r", pinRPx),
					svg.A("fill", style.Pin)}, pinKeys(pl, pin.PortRef)...)...)
			} else if style.PickTargets && pl.GetNetAnchor() == "" {
				// The same circle, invisible and larger, since a pin is a POINT and a point is
				// unclickable without an area. A net anchor gets none, because its target would carry
				// an empty ref and resolve to nothing. See
				// docsite/content/architecture/geometry-and-rendering.md#symbols-that-name-a-net.
				c.El("circle", append([]svg.Attr{svg.F("cx", tx(wp.X)), svg.F("cy", ty(wp.Y)), svg.F("r", pinPickRPx),
					svg.A("fill", "none"), svg.A("pointer-events", "all")}, pinKeys(pl, pin.PortRef)...)...)
			}
			if pin.LabelOrigin != nil {
				lp := geomath.ApplyTransform(pl.Transform, pin.LabelOrigin)
				// Sized from the source like every other run, with labelFont's sheet default when
				// the format states no height.
				px := labelFont(pin.Height, def, scale)
				// The name draws at label_origin. The NUMBER draws at its own origin when the
				// source places the two separately, and otherwise stacks a line off the name.
				nx, ny, nj := tx(lp.X), ty(lp.Y)+px*1.4, pin.Justify
				if pin.NumberOrigin != nil {
					np := geomath.ApplyTransform(pl.Transform, pin.NumberOrigin)
					nx, ny, nj = tx(np.X), ty(np.Y), pin.NumberJustify
				}
				drawText(c, pin.PortRef, nx, ny, px, nj, rot, style.Ruler, 0)
				if pin.Name != "" {
					drawText(c, pin.Name, tx(lp.X), ty(lp.Y), px, pin.Justify, rot, style.PinName, 0)
				}
			}
		}
		for _, a := range sym.Annotations {
			if a.Origin == nil {
				continue
			}
			wp := geomath.ApplyTransform(pl.Transform, a.Origin)
			drawText(c, a.Text, tx(wp.X), ty(wp.Y), labelFont(a.Height, def, scale), a.Justify, rot, style.Annotation, boxWidthPx(sym, scale))
		}
	}

	// Page labels. Multi-line free-text columns (a documentation page) shrink to fit their
	// layout rather than spill into the next column or the title block (WS7-038, freetext.go).
	fit := freeTextFit(g, sheet)
	for _, l := range sheet.Labels {
		if l.Origin == nil {
			continue
		}
		fontPx := labelFont(l.Height, def, scale)
		if f, ok := fit[l]; ok {
			fontPx *= f
		}
		drawText(c, l.Text, tx(l.Origin.X), ty(l.Origin.Y), fontPx, l.Justify, l.RotationDeg, style.Label, 0)
	}
	// Placement text fields (ref-des, value, custom), each at its own position and justify.
	// KiCad rotates a field with its parent symbol, so the placement rotation adds to the field's
	// own angle.
	for _, pl := range sheet.Placements {
		rot := placementRotation(pl.Transform)
		for _, f := range pl.Fields {
			if !f.Visible || f.Origin == nil {
				continue
			}
			drawText(c, f.Value, tx(f.Origin.X), ty(f.Origin.Y), labelFont(f.Height, def, scale), f.Justify, rot+f.RotationDeg, style.Field, 0)
		}
	}
}

// SVG pixel-space constants shared by the sheet document and any overlay drawn above it
// (HighlightSVG): both must size and stroke from the same numbers to composite exactly.
const (
	sheetMaxPx    = 1600.0          // longest side of the drawing, pre-margin
	sheetMarginPx = sheetMaxPx / 40 // whitespace around the drawing
	strokePx      = 0.8             // geometry stroke width
	busStrokePx   = strokePx * 3   // bus trunk/entry stroke: a fixed-visual-width line, thicker than a wire (WS7-042)
	pinRPx        = 2.5             // pin connect-dot radius
	// pinPickRPx is the invisible pick target's radius (WithPickTargets): wider than the dot,
	// because it exists to be HIT rather than seen, and a pin at a normal zoom is a few pixels.
	pinPickRPx = 6.0
	// wirePickWidthPx is the invisible companion stroke's width: wide enough that a click near a
	// wire lands on it, narrow enough that two parallel wires a few pixels apart stay distinct.
	wirePickWidthPx = 7.0
	lineHeight    = 1.2            // multiplier on font size for stacking multi-line text
	// glyphAdvanceEm is the average horizontal advance of one glyph as a fraction of the font
	// size, the one calibration shared by naturalTextWidthPx and the free-text column fit
	// (freetext.go). It is an AVERAGE over SchematicFontStack.
	//
	// Measured in Arial across 31 realistic schematic runs, the weighted average is 0.6147
	// (0.514 for "6.3V" to 0.736 for "DGND"), so 0.6 under-predicts by 2.4%. Both callers only
	// decide WHETHER to condense, so changing it would move the free-text fit and every golden for
	// no gain. A figure taken from a few uppercase runs overstates the error threefold.
	glyphAdvanceEm = 0.6
	// zoneLabelFrac sizes a zone-ruler label as a fraction of its zone row, measured from the
	// printed frame of a tool this engine reads (a 21.9pt label in a 396pt row).
	zoneLabelFrac = 0.055
	// minStrokeNm is the minimum copper trace width in BOARD space (nanometers), not output
	// pixels, so a dense board's fine traces stay proportional. ~25um is below any real trace.
	// See docsite/content/architecture/geometry-and-rendering.md#board-geometry-sidecar.
	minStrokeNm = 25_000.0
)

// sheetFrame is the world-to-pixel mapping of one sheet's SVG document: the tx/ty coordinate
// maps (Y-flipping, margin-inset), the scale, and the document size. frameSheet computes it
// once per sheet and every SVG projection of that sheet shares it, so overlays line up.
type sheetFrame struct {
	tx, ty func(int64) float64
	scale  float64
	pxW    float64
	pxH    float64
}

// frameSheet frames one sheet: bounds over page size, placed shapes, pins, annotations,
// wires, labels, free shapes, placement fields, and images, normalized into a small pixel
// space (longest side ~sheetMaxPx) so viewBox values stay in a range rasterizers render
// precisely. Geometry is Y-up while SVG is Y-down, so ty flips Y.
func frameSheet(sheet *geom.SheetGeometry, syms map[string]*geom.SymbolDef) sheetFrame {
	var b geomath.Bounds
	if sheet.Size != nil {
		b.Add(sheet.Size.Min)
		b.Add(sheet.Size.Max)
	}
	for _, pl := range sheet.Placements {
		for _, s := range placedShapes(syms, pl) {
			for _, p := range s.Points {
				b.Add(p)
			}
		}
		if sym := symbolFor(syms, pl); sym != nil {
			for _, pin := range sym.Pins {
				b.Add(geomath.PlacePin(pl.Transform, pin))
			}
			for _, a := range sym.Annotations {
				b.Add(geomath.ApplyTransform(pl.Transform, a.Origin))
			}
		}
	}
	for _, w := range sheet.Wires {
		for _, pl := range w.Polylines {
			for _, p := range pl.Points {
				b.Add(p)
			}
		}
	}
	for _, l := range sheet.Labels {
		b.Add(l.Origin)
	}
	for _, s := range sheet.Shapes {
		for _, p := range s.Points {
			b.Add(p)
		}
	}
	for _, pl := range sheet.Placements {
		for _, f := range pl.Fields {
			b.Add(f.Origin)
		}
	}
	for _, im := range sheet.Images {
		if im.Bbox != nil {
			b.Add(im.Bbox.Min)
			b.Add(im.Bbox.Max)
		}
	}
	if !b.Valid() {
		// An empty sheet still gets a sane 1000x1000 frame.
		b.Add(&geom.Point{X: 0, Y: 0})
		b.Add(&geom.Point{X: 1000, Y: 1000})
	}

	bMinX, bMinY := b.Min()
	bMaxX, bMaxY := b.Max()
	spanX, spanY := bMaxX-bMinX, bMaxY-bMinY
	scale := sheetMaxPx / float64(max(max(spanX, spanY), 1))
	return sheetFrame{
		// tx/ty map source units to pixels, flipping Y and adding a margin.
		tx:    func(x int64) float64 { return float64(x-bMinX)*scale + sheetMarginPx },
		ty:    func(y int64) float64 { return float64(bMaxY-y)*scale + sheetMarginPx },
		scale: scale,
		pxW:   float64(spanX)*scale + 2*sheetMarginPx,
		pxH:   float64(spanY)*scale + 2*sheetMarginPx,
	}
}

// placedShapes resolves a placement to its world-space shapes (symbol graphics under the
// placement transform). Returns nil when the symbol is not in the library.
func placedShapes(syms map[string]*geom.SymbolDef, pl *geom.SymbolPlacement) []*geom.Shape {
	sym := symbolFor(syms, pl)
	if sym == nil {
		return nil
	}
	out := make([]*geom.Shape, 0, len(sym.Shapes))
	for _, s := range sym.Shapes {
		out = append(out, geomath.PlaceShape(pl.Transform, s))
	}
	return out
}

// drawImage renders a geom.Image as an SVG <image> with an inline data URI, mapped to pixels
// through tx/ty. The bbox is axis-aligned; a non-zero rotation is applied as an SVG rotate about
// the image centre (mirror is a horizontal flip). An image with no bytes is skipped.
func drawImage(c *svg.Canvas, im *geom.Image, tx, ty func(int64) float64) {
	if im == nil || im.Bbox == nil || len(im.Data) == 0 {
		return
	}
	x0, x1 := tx(im.Bbox.Min.X), tx(im.Bbox.Max.X)
	// ty flips Y, so the geom-max Y is the top edge in pixels.
	yTop, yBot := ty(im.Bbox.Max.Y), ty(im.Bbox.Min.Y)
	x, y := math.Min(x0, x1), math.Min(yTop, yBot)
	w, h := math.Abs(x1-x0), math.Abs(yBot-yTop)
	mime := im.Mime
	if mime == "" {
		mime = "image/png"
	}
	uri := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(im.Data)
	attrs := []svg.Attr{svg.F("x", x), svg.F("y", y), svg.F("width", w), svg.F("height", h), svg.A("href", uri)}
	if im.RotationDeg != 0 || im.Mirror {
		cx, cy := x+w/2, y+h/2
		t := fmt.Sprintf("rotate(%d %g %g)", -im.RotationDeg, cx, cy) // SVG rotates CW; geom is CCW
		if im.Mirror {
			t += fmt.Sprintf(" translate(%g 0) scale(-1 1)", 2*cx)
		}
		attrs = append(attrs, svg.A("transform", t))
	}
	c.El("image", attrs...)
}

// wireKeys, symbolKeys and pinKeys build the data-* attributes a rendered element carries, so
// the SVG document is its own pick index and a saved or embedded sheet keeps that identity.
//
// Values come from the design, so they go through svg.AEsc. A net named with a quote would
// otherwise close the attribute and inject markup into a document the viewer mounts with innerHTML.
func wireKeys(w *geom.WireGeometry) []svg.Attr {
	keys := []svg.Attr{svg.AEsc("data-kind", "net"), svg.AEsc("data-net", w.GetNet())}
	if id := w.GetNetId(); id != "" {
		keys = append(keys, svg.AEsc("data-net-id", id))
	}
	switch w.GetKind() {
	case geom.WireGeometry_KIND_BUS, geom.WireGeometry_KIND_BUS_ENTRY:
		// A bus carries no net identity of its own. Its NAME is the join key (WS7-042b), and the
		// kind tells a picker not to treat it as a net.
		keys[0] = svg.AEsc("data-kind", "bus")
		keys = append(keys, svg.AEsc("data-bus", w.GetNet()))
	}
	return keys
}

// symbolKeys identify a placed symbol's graphics as the NET it names when it is an anchor (a
// ground or rail glyph), and as a component otherwise. An anchor keyed by ref_des would offer a
// component nothing can join. See
// docsite/content/architecture/geometry-and-rendering.md#symbols-that-name-a-net.
func symbolKeys(pl *geom.SymbolPlacement) []svg.Attr {
	if net := pl.GetNetAnchor(); net != "" {
		return []svg.Attr{svg.AEsc("data-kind", "net"), svg.AEsc("data-net", net)}
	}
	return []svg.Attr{svg.AEsc("data-kind", "component"), svg.AEsc("data-ref", pl.GetRefDes())}
}

// pinKeys identify one pin of one placement, the finest pick target. data-ref is repeated so a
// picker that resolves to a pin also knows its component without walking the DOM.
func pinKeys(pl *geom.SymbolPlacement, portRef string) []svg.Attr {
	return []svg.Attr{svg.AEsc("data-kind", "pin"), svg.AEsc("data-ref", pl.GetRefDes()), svg.AEsc("data-pin", portRef)}
}

// writeShape draws one shape. keys carries the entity attributes the CALLER knows (a symbol's
// keys, nothing for free sheet graphics), stamped onto every element the shape emits.
func writeShape(c *svg.Canvas, s *geom.Shape, tx, ty func(int64) float64, scale, stroke float64, style Style, strokeColor string, keys ...svg.Attr) {
	el := func(tag string, attrs ...svg.Attr) { c.El(tag, append(attrs, keys...)...) }
	fill := shapeFill(s, style, strokeColor)
	switch s.Kind {
	case geom.Shape_KIND_RECT:
		if len(s.Points) < 2 {
			return
		}
		// Corners transform pointwise; renormalize since rotation may swap min/max.
		x0, y0 := tx(s.Points[0].X), ty(s.Points[0].Y)
		x1, y1 := tx(s.Points[1].X), ty(s.Points[1].Y)
		el("rect", svg.A("fill", fill), svg.A("stroke", strokeColor), svg.F("stroke-width", stroke),
			svg.F("x", math.Min(x0, x1)), svg.F("y", math.Min(y0, y1)), svg.F("width", math.Abs(x1-x0)), svg.F("height", math.Abs(y1-y0)))
	case geom.Shape_KIND_CIRCLE:
		if len(s.Points) < 1 {
			return
		}
		el("circle", svg.A("fill", fill), svg.A("stroke", strokeColor), svg.F("stroke-width", stroke),
			svg.F("cx", tx(s.Points[0].X)), svg.F("cy", ty(s.Points[0].Y)), svg.F("r", float64(s.Radius)*scale))
	case geom.Shape_KIND_DOT:
		if len(s.Points) < 1 {
			return
		}
		el("circle", svg.F("cx", tx(s.Points[0].X)), svg.F("cy", ty(s.Points[0].Y)), svg.F("r", stroke*2), svg.A("fill", strokeColor))
	case geom.Shape_KIND_POLYLINE, geom.Shape_KIND_ARC:
		// ARC draws as a polyline through its (start, mid, end) points. A filled polyline (a
		// symbol body) draws as a closed polygon so the fill has an area to cover.
		if len(s.Points) == 0 {
			return
		}
		tag := "polyline"
		if s.Fill != geom.Shape_FILL_UNSPECIFIED && s.Kind == geom.Shape_KIND_POLYLINE {
			tag = "polygon"
		}
		el(tag, svg.A("fill", fill), svg.A("stroke", strokeColor), svg.F("stroke-width", stroke), svg.A("points", points(s.Points, tx, ty)))
	}
}

// shapeFill returns the SVG fill for a shape's fill style: the stroke color for OUTLINE, a
// pale body color for BACKGROUND (matching KiCad's symbol fill), the explicit color for
// COLOR, and "none" (outline only) otherwise.
func shapeFill(s *geom.Shape, style Style, strokeColor string) string {
	switch s.Fill {
	case geom.Shape_FILL_OUTLINE:
		return strokeColor
	case geom.Shape_FILL_BACKGROUND:
		return style.ShapeFill
	case geom.Shape_FILL_COLOR:
		if s.FillColor != "" {
			return s.FillColor
		}
		return style.ShapeFill
	default:
		return "none"
	}
}

// drawWorksheet draws the drawing-sheet furniture around the schematic in pixel space: the
// inset page border, the zone ruler (numbers across, letters down), and the bottom-right title
// block. It is synthesized from sheet.Size, so a sheet with no page (an auto-layout graph) gets
// none. worksheetLines is the WebGL counterpart.
func drawWorksheet(c *svg.Canvas, g *geom.SchematicGeometry, sheet *geom.SheetGeometry, tx, ty func(int64) float64, style Style) {
	if sheet.GetSuppressWorksheet() {
		return // the source format carries its own title block/frame (xschem/gEDA); WS7-036
	}
	sz := sheet.Size
	if sz == nil || sz.Min == nil || sz.Max == nil {
		return
	}
	// Page corners in pixels (geom is Y-up, so Max.Y is the top edge).
	left, right := tx(sz.Min.X), tx(sz.Max.X)
	top, bot := ty(sz.Max.Y), ty(sz.Min.Y)
	m := 0.02 * math.Min(right-left, bot-top) // drawing margin inset
	l, r, t, b := left+m, right-m, top+m, bot-m
	const stroke = 0.8
	line := func(x0, y0, x1, y1 float64) {
		c.El("polyline", svg.A("fill", "none"), svg.A("stroke", style.Frame), svg.F("stroke-width", stroke),
			svg.A("points", fmt.Sprintf("%.1f,%.1f %.1f,%.1f", x0, y0, x1, y1)))
	}
	// Inner frame border.
	c.El("rect", svg.A("fill", "none"), svg.A("stroke", style.Frame), svg.F("stroke-width", stroke),
		svg.F("x", l), svg.F("y", t), svg.F("width", r-l), svg.F("height", b-t))

	// Zone ruler: numbers across the top and bottom, letters down the sides. zoneCols sets the
	// column count, numbered right-to-left (WS7-038).
	cols, rows := int(zoneCols(g, sheet)), 4
	// Zone labels scale with the zone row (zoneLabelFrac), floored at the text minimum.
	rulerFont := math.Max(sheetMaxPx*minFontFrac, (b-t)/float64(rows)*zoneLabelFrac)
	for i := 0; i < cols; i++ {
		x0 := l + (r-l)*float64(i)/float64(cols)
		x1 := l + (r-l)*float64(i+1)/float64(cols)
		if i > 0 {
			line(x0, t, x0, t+m*0.6)
			line(x0, b, x0, b-m*0.6)
		}
		label := fmt.Sprintf("%d", zoneNumber(int64(cols), int64(i)))
		c.Text(label, svg.F("x", (x0+x1)/2), svg.F("y", t+m*0.5), svg.F("font-size", rulerFont), svg.A("text-anchor", "middle"), svg.A("dominant-baseline", "central"), svg.A("fill", style.Ruler))
		c.Text(label, svg.F("x", (x0+x1)/2), svg.F("y", b-m*0.5), svg.F("font-size", rulerFont), svg.A("text-anchor", "middle"), svg.A("dominant-baseline", "central"), svg.A("fill", style.Ruler))
	}
	for i := 0; i < rows; i++ {
		y0 := t + (b-t)*float64(i)/float64(rows)
		y1 := t + (b-t)*float64(i+1)/float64(rows)
		if i > 0 {
			line(l, y0, l+m*0.6, y0)
			line(r, y0, r-m*0.6, y0)
		}
		label := string(rune('A' + i))
		c.Text(label, svg.F("x", l+m*0.5), svg.F("y", (y0+y1)/2), svg.F("font-size", rulerFont), svg.A("text-anchor", "middle"), svg.A("dominant-baseline", "central"), svg.A("fill", style.Ruler))
		c.Text(label, svg.F("x", r-m*0.5), svg.F("y", (y0+y1)/2), svg.F("font-size", rulerFont), svg.A("text-anchor", "middle"), svg.A("dominant-baseline", "central"), svg.A("fill", style.Ruler))
	}

	drawTitleBlock(c, g, sheet, l, r, t, b, style)
}

// drawTitleBlock draws the bottom-right title-block grid (border, dividers, field text) within
// the inner frame (l,r,t,b in pixels), mapping titleBlockGrid's cells into the pixel box. Empty
// cells draw their label with no value (WS7-021).
func drawTitleBlock(c *svg.Canvas, g *geom.SchematicGeometry, sheet *geom.SheetGeometry, l, r, t, b float64, style Style) {
	rows := titleBlockGrid(g, sheet)
	total := gridWeight(rows)
	rowH := math.Max(10, math.Min((b-t)/30, 22))
	tw := math.Min((r-l)*0.36, 360)
	th := math.Min(total*rowH, (b-t)*0.6)
	bx, byTop := r-tw, b-th // top-left of the title block; the block bottom sits on the frame
	c.El("rect", svg.A("fill", style.TitleBlockFill), svg.A("stroke", style.Frame), svg.F("stroke-width", 0.8),
		svg.F("x", bx), svg.F("y", byTop), svg.F("width", tw), svg.F("height", th))

	acc := 0.0
	for i, row := range rows {
		yTop := byTop + th*acc/total
		acc += row.weight
		yBot := byTop + th*acc/total
		if i < len(rows)-1 {
			c.El("polyline", svg.A("fill", "none"), svg.A("stroke", style.Frame), svg.F("stroke-width", 0.5),
				svg.A("points", fmt.Sprintf("%.1f,%.1f %.1f,%.1f", bx, yBot, bx+tw, yBot)))
		}
		font := math.Max(6, math.Min((yBot-yTop)*0.5, 13))
		for _, cell := range row.cells {
			cx0 := bx + tw*cell.x0
			if cell.x0 > 0.0001 { // interior column divider
				c.El("polyline", svg.A("fill", "none"), svg.A("stroke", style.Frame), svg.F("stroke-width", 0.5),
					svg.A("points", fmt.Sprintf("%.1f,%.1f %.1f,%.1f", cx0, yTop, cx0, yBot)))
			}
			if cell.text == "" {
				continue
			}
			f := font
			fill := style.Annotation
			attrs := []svg.Attr{svg.F("x", cx0+4), svg.F("y", (yTop+yBot)/2), svg.F("font-size", f), svg.A("dominant-baseline", "central"), svg.A("fill", fill)}
			if cell.bold {
				attrs = append(attrs, svg.A("font-weight", "bold"))
			}
			c.Text(cell.text, attrs...)
		}
	}
}

// points formats a point list as an SVG "x,y x,y ..." string in pixel space.
func points(pts []*geom.Point, tx, ty func(int64) float64) string {
	var b strings.Builder
	for i, p := range pts {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.FormatFloat(tx(p.X), 'f', 1, 64))
		b.WriteByte(',')
		b.WriteString(strconv.FormatFloat(ty(p.Y), 'f', 1, 64))
	}
	return b.String()
}

// Text-size bounds, as fractions of the drawing rather than absolute pixels, because a bound
// says how much of the sheet one run may occupy. A real title measured 2.2% of its sheet. See
// docsite/content/architecture/geometry-and-rendering.md#text-stays-readable-and-inside-its-box.
const (
	minFontFrac = 0.001 // 1.6px on a 1600px drawing: below this a glyph is not text
	maxFontFrac = 0.10  // 160px: still catches a height the size of the page
)

// labelFont maps a source text height to an output font size in pixels, clamped to the
// minFontFrac/maxFontFrac bounds. It honors the source height, so text a format draws tiny (KiCad's
// footprint field at 0.254mm) stays tiny (WS7-020). A source that states no height falls back to
// def, the sheet's median text height (defaultTextHeight), the same default the WebGL path uses.
func labelFont(h int64, def int64, scale float64) float64 {
	if h <= 0 {
		h = def
	}
	px := float64(h) * scale
	return math.Max(sheetMaxPx*minFontFrac, math.Min(px, sheetMaxPx*maxFontFrac))
}

// drawText emits one text run at pixel (x,y), stacking lines when content carries newlines
// (EDIF %10% escapes decode to real newlines) around the anchor blockTop picks. rotDeg is a CCW
// Y-up angle and equals the SVG rotate() angle, because the Y-flip and the handedness swap it
// induces cancel. Callers pass a placement's rotation combined with the text's own angle, and a
// zero angle emits no transform.
func drawText(c *svg.Canvas, content string, x, y, fontPx float64, justify string, rotDeg int32, fill string, maxWidthPx float64) {
	if i := strings.IndexByte(content, '\n'); i >= 0 {
		lines := strings.Split(content, "\n")
		step := fontPx * lineHeight
		// readableText may flip the justify for an upside-down run, and the block must grow the
		// way the FINAL justify says, so resolve it here exactly as drawTextLine will.
		_, j := readableText(rotDeg, justify)
		top := blockTop(y, len(lines), j, step)
		for n, line := range lines {
			drawTextLine(c, line, x, top+float64(n)*step, fontPx, justify, rotDeg, fill, maxWidthPx)
		}
		return
	}
	drawTextLine(c, content, x, y, fontPx, justify, rotDeg, fill, maxWidthPx)
}

// blockTop is the y of a multi-line block's FIRST line, given the anchor y its justify names.
// A justify anchors the whole BLOCK, so a bottom-anchored block grows UPWARD and a centered one
// grows both ways. A tool that bottom-anchors its notes places the NEXT note relative to that
// bottom, so stacking downward runs one note into the next. See
// docsite/content/architecture/geometry-and-rendering.md#text-stays-readable-and-inside-its-box.
func blockTop(y float64, lines int, justify string, step float64) float64 {
	if lines < 2 {
		return y
	}
	span := float64(lines-1) * step
	switch {
	case strings.Contains(justify, "top"):
		return y
	case strings.Contains(justify, "bottom"):
		return y - span
	default:
		return y - span/2 // centered: half above, half below
	}
}

// drawTextLine draws one line of text; drawText handles multi-line splitting.
func drawTextLine(c *svg.Canvas, content string, x, y, fontPx float64, justify string, rotDeg int32, fill string, maxWidthPx float64) {
	rotDeg, justify = readableText(rotDeg, justify)
	anchor, baseline := justifyText(justify)
	attrs := []svg.Attr{
		svg.F("x", x), svg.F("y", y), svg.F("font-size", fontPx),
		svg.A("text-anchor", anchor), svg.A("dominant-baseline", baseline), svg.A("fill", fill),
	}
	// A caption bounded by its symbol box (maxWidthPx > 0) is condensed horizontally to the box
	// width, keeping its font height, and only when it would overflow, so a short caption is
	// never stretched. librsvg ignores textLength, so verify this in a browser.
	if maxWidthPx > 0 && naturalTextWidthPx(content, fontPx) > maxWidthPx {
		attrs = append(attrs, svg.F("textLength", maxWidthPx), svg.A("lengthAdjust", "spacingAndGlyphs"))
	}
	if a := normDeg(rotDeg); a != 0 {
		attrs = append(attrs, svg.A("transform", fmt.Sprintf("rotate(%g %g %g)", a, x, y)))
	}
	c.Text(content, attrs...)
}

// boxWidthPx is a symbol's caption-width budget in pixels: its body-box width mapped through the
// sheet scale, or 0 when it has no box (meaning "do not condense"). See captionWidth and drawText.
func boxWidthPx(sym *geom.SymbolDef, scale float64) float64 {
	return float64(captionWidth(sym)) * scale
}

// naturalTextWidthPx estimates a run's rendered width as glyphAdvanceEm*fontPx per rune, to
// decide whether a box-bounded caption needs condensing. It is an average, not an exact
// advance; see glyphAdvanceEm.
func naturalTextWidthPx(content string, fontPx float64) float64 {
	return glyphAdvanceEm * fontPx * float64(len([]rune(content)))
}

// placementRotation is a placement's rotation in degrees, 0 for a nil transform. Callers add
// it to the text's own angle. Mirror and scale are not applied, because KiCad does not mirror
// glyphs.
func placementRotation(t *geom.Transform) int32 {
	if t == nil {
		return 0
	}
	return t.RotationDeg
}

// readableText keeps schematic text upright, as EDA viewers draw it. An angle whose normalized
// magnitude exceeds 90 is turned a further 180 and its justify flipped on both axes, so the run
// hangs off the same corner of its origin. Vertical text (+/-90) is left alone, which keeps
// KiCad-90 parity.
func readableText(rotDeg int32, justify string) (int32, string) {
	a := normDeg(rotDeg)
	if a <= 90 && a >= -90 {
		return rotDeg, justify
	}
	return rotDeg + 180, flipJustify(justify)
}

// flipJustify swaps a justify string across both axes (left<->right, top<->bottom), the
// alignment change that pairs with readableText's 180-degree flip. A centered axis is unaffected.
func flipJustify(justify string) string {
	switch {
	case strings.Contains(justify, "left"):
		justify = strings.Replace(justify, "left", "right", 1)
	case strings.Contains(justify, "right"):
		justify = strings.Replace(justify, "right", "left", 1)
	}
	switch {
	case strings.Contains(justify, "top"):
		justify = strings.Replace(justify, "top", "bottom", 1)
	case strings.Contains(justify, "bottom"):
		justify = strings.Replace(justify, "bottom", "top", 1)
	}
	return justify
}

// normDeg reduces a rotation to (-180, 180] so SVG rotate() takes the short way around: a
// KiCad-90 placement (geom 270) becomes -90, matching kicad-cli's rotate(-90 ...).
func normDeg(deg int32) float64 {
	a := math.Mod(float64(deg), 360)
	if a > 180 {
		a -= 360
	}
	if a <= -180 {
		a += 360
	}
	return a
}

// justifyText maps the canonical "<h> <v>" justify (h in {left,center,right}, v in
// {top,middle,bottom}, either absent meaning centered) onto SVG text-anchor and
// dominant-baseline.
func justifyText(justify string) (anchor, baseline string) {
	anchor, baseline = "middle", "central"
	switch {
	case strings.Contains(justify, "left"):
		anchor = "start"
	case strings.Contains(justify, "right"):
		anchor = "end"
	}
	switch {
	case strings.Contains(justify, "top"):
		baseline = "text-before-edge"
	case strings.Contains(justify, "bottom"):
		baseline = "text-after-edge"
	}
	return anchor, baseline
}

// indexSymbols and symbolFor delegate to geomath, which owns the join, so the reader, validate
// and the renderer all answer "does this placement draw?" the same way (agni issue 354).
func indexSymbols(g *geom.SchematicGeometry) geomath.SymbolIndex { return geomath.IndexSymbols(g) }

func symbolFor(syms geomath.SymbolIndex, pl *geom.SymbolPlacement) *geom.SymbolDef {
	return syms.SymbolFor(pl)
}

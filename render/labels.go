package render

import (
	"math"
	"sort"
	"strconv"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
)

// placedLabel is one piece of schematic text positioned in world (sheet, Y-up) coordinates,
// with the color the SVG backend would draw it in. Height is in world units; the text
// overlay scales it to pixels with the view camera.
type placedLabel struct {
	x, y        int64
	text        string
	height      int64
	rotationDeg int32
	justify     string
	color       string
	maxWidth    int64 // world-space width to condense into (0 = unbounded); see boxTextWidth
}

// collectLabels gathers all schematic text for a sheet in world (Y-up) coordinates:
// placement fields (ref-des/value), sheet labels, symbol annotations, pin numbers and names,
// and the synthesized worksheet text (zone-ruler numbers/letters and the title-block fields).
// It is the text counterpart to PackSheet's geometry; the packer rebases these into
// PackedSheet.Labels and the WebGL overlay draws them. Colors come from the Style, the same
// palette the SVG backend draws with, so the two renderers agree.
func collectLabels(g *geom.SchematicGeometry, sheet *geom.SheetGeometry, style Style) []placedLabel {
	syms := indexSymbols(g)
	// A world text height for text that carries none (many readers leave field/label height
	// unset; the SVG backend then falls back to a clamped pixel size). Using one sheet default
	// keeps such text a consistent, readable size under the page-fitting camera.
	def := defaultTextHeight(g, sheet)
	var out []placedLabel
	add := func(x, y int64, text string, h int64, rot int32, justify, color string, maxWidth int64) {
		if text == "" {
			return
		}
		if h <= 0 {
			h = def
		}
		// Keep text upright, matching the SVG backend (see readableText): the WebGL overlay
		// applies rotationDeg/justify verbatim, so it must receive already-readable values.
		rot, justify = readableText(rot, justify)
		out = append(out, placedLabel{x: x, y: y, text: text, height: h, rotationDeg: rot, justify: justify, color: color, maxWidth: maxWidth})
	}

	// Placement text fields (ref-des, value, custom) — already in sheet coordinates. KiCad
	// rotates a field with its parent symbol, so add the placement rotation to the field's own.
	for _, pl := range sheet.Placements {
		rot := placementRotation(pl.Transform)
		for _, f := range pl.Fields {
			if !f.Visible || f.Origin == nil {
				continue
			}
			add(f.Origin.X, f.Origin.Y, f.Value, f.Height, rot+f.RotationDeg, f.Justify, style.Field, 0)
		}
	}
	// Sheet labels (net labels, page text). Multi-line free-text columns are shrunk to fit their
	// layout (WS7-038, freetext.go); the same factor the SVG backend uses, applied to the world
	// text height so the two renderers agree.
	fit := freeTextFit(g, sheet)
	for _, l := range sheet.Labels {
		if l.Origin == nil {
			continue
		}
		h := l.Height
		if f, ok := fit[l]; ok {
			eff := h
			if eff <= 0 {
				eff = def
			}
			h = int64(float64(eff) * f)
		}
		add(l.Origin.X, l.Origin.Y, l.Text, h, l.RotationDeg, l.Justify, style.Label, 0)
	}
	// Symbol annotations and pin numbers/names are symbol-local; transform to sheet space.
	for _, pl := range sheet.Placements {
		sym := symbolFor(syms, pl)
		if sym == nil {
			continue
		}
		rot := placementRotation(pl.Transform)
		for _, a := range sym.Annotations {
			if a.Origin == nil {
				continue
			}
			wp := geomath.ApplyTransform(pl.Transform, a.Origin)
			add(wp.X, wp.Y, a.Text, a.Height, rot, a.Justify, style.Annotation, captionWidth(sym))
		}
		for _, pin := range sym.Pins {
			if pin.LabelOrigin == nil {
				continue
			}
			lp := geomath.ApplyTransform(pl.Transform, pin.LabelOrigin)
			add(lp.X, lp.Y, pin.PortRef, def, rot, pin.Justify, style.Ruler, 0)
			add(lp.X, lp.Y, pin.Name, def, rot, pin.Justify, style.PinName, 0)
		}
	}
	// Worksheet furniture text: zone-ruler numbers/letters and the title-block fields.
	out = append(out, worksheetLabels(g, sheet, def, style)...)
	return out
}

// worksheetLabels positions the zone-ruler numbers/letters and the title-block field text
// from the same frameLayout the furniture geometry uses (worksheet.go), so labels land in
// their cells. Empty when the sheet has no page.
func worksheetLabels(g *geom.SchematicGeometry, sheet *geom.SheetGeometry, textH int64, style Style) []placedLabel {
	if sheet.GetSuppressWorksheet() {
		return nil // no synthetic worksheet, so no ruler/title-block text either (xschem/gEDA); WS7-036
	}
	fl, ok := worksheetLayout(g, sheet)
	if !ok {
		return nil
	}
	var out []placedLabel
	rulerH := textH

	// Column numbers centered in each column, just inside the top and bottom edges. The standard
	// frame numbers zones right-to-left, so the leftmost column carries the highest number and the
	// rightmost is 1 (WS7-038).
	for i := int64(0); i < fl.cols; i++ {
		cx := fl.l + (fl.r-fl.l)*(2*i+1)/(2*fl.cols)
		label := strconv.FormatInt(zoneNumber(fl.cols, i), 10)
		out = append(out, placedLabel{cx, fl.t - fl.m/2, label, rulerH, 0, "center center", style.Ruler, 0})
		out = append(out, placedLabel{cx, fl.b + fl.m/2, label, rulerH, 0, "center center", style.Ruler, 0})
	}
	// Row letters A.. centered in each row, just inside the left and right edges. Row A is the
	// top row; geom is Y-up, so the top row has the highest y.
	for i := int64(0); i < fl.rows; i++ {
		cy := fl.b + (fl.t-fl.b)*(2*i+1)/(2*fl.rows)
		label := string(rune('A' + (fl.rows - 1 - i)))
		out = append(out, placedLabel{fl.l + fl.m/2, cy, label, rulerH, 0, "center center", style.Ruler, 0})
		out = append(out, placedLabel{fl.r - fl.m/2, cy, label, rulerH, 0, "center center", style.Ruler, 0})
	}
	// Title-block field text from the shared grid, each cell at its cell's center-left. Row 0 is
	// the top; geom is Y-up, so weight accumulates downward from the box top edge.
	rows := titleBlockGrid(g, sheet)
	bx, by, bw, bh := titleBlockBox(fl, rows)
	if total := gridWeight(rows); total > 0 {
		rowH := int64(float64(bh) / total)
		th := min(textH, max(rowH*3/5, 1)) // fit within a row
		acc := 0.0
		for _, row := range rows {
			yTop := by + bh - int64(math.Round(acc/total*float64(bh)))
			acc += row.weight
			yBot := by + bh - int64(math.Round(acc/total*float64(bh)))
			cy := (yTop + yBot) / 2
			for _, cell := range row.cells {
				if cell.text == "" {
					continue
				}
				out = append(out, placedLabel{bx + int64(cell.x0*float64(bw)) + fl.m/3, cy, cell.text, th, 0, "left center", style.Annotation, 0})
			}
		}
	}
	return out
}

// captionWidth is the world-space width a symbol's caption is allowed to occupy: the width of the
// symbol's drawn body rectangle (its widest BOX figure), which is the box the authoring tool fits
// a label like "Net Splitter" inside. A caption wider than this is condensed to fit (see drawText
// / the WebGL overlay's textLength) rather than spilling past the rectangle. It falls back to the
// full symbol bounding box when there is no BOX figure, then 0 (unbounded). The body box is
// symbol-local, the frame the caption rotates in, so this is correct for a rotated placement too.
func captionWidth(sym *geom.SymbolDef) int64 {
	if sym == nil {
		return 0
	}
	var body int64
	for _, s := range sym.Shapes {
		if s.Kind != geom.Shape_KIND_RECT || s.FigureGroup != "BOX" || len(s.Points) < 2 {
			continue
		}
		if w := absI64(s.Points[1].X - s.Points[0].X); w > body {
			body = w
		}
	}
	if body > 0 {
		return body
	}
	return bboxWidth(sym.Bbox)
}

// bboxWidth is a bounding box's absolute width, or 0 for a nil/empty box.
func bboxWidth(box *geom.BBox) int64 {
	if box == nil || box.Min == nil || box.Max == nil {
		return 0
	}
	return absI64(box.Max.X - box.Min.X)
}

func absI64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// defaultTextHeight is a world text height for labels that carry none (pin numbers/names):
// the median of the explicit field/label heights on the sheet, so pin text sits at a size
// consistent with the rest. Falls back to a fraction of the frame margin, then 1.
func defaultTextHeight(g *geom.SchematicGeometry, sheet *geom.SheetGeometry) int64 {
	var hs []int64
	for _, pl := range sheet.Placements {
		for _, f := range pl.Fields {
			if f.Visible && f.Height > 0 {
				hs = append(hs, f.Height)
			}
		}
	}
	for _, l := range sheet.Labels {
		if l.Height > 0 {
			hs = append(hs, l.Height)
		}
	}
	if len(hs) == 0 {
		if fl, ok := worksheetLayout(g, sheet); ok {
			return max(fl.m, 1) // ~2% of the page: readable under the page-fitting camera
		}
		return 1
	}
	sort.Slice(hs, func(i, j int) bool { return hs[i] < hs[j] })
	return hs[len(hs)/2]
}

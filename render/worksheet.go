package render

import (
	"fmt"
	"math"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// framePrim is one worksheet-furniture primitive: a GPU draw kind and its world-space
// vertices. All furniture belongs to groupFrame.
type framePrim struct {
	kind uint8
	pts  [][2]int64
}

// frameLayout is the world-space geometry of the drawing-sheet furniture: the inner frame
// corners, the ruler divisions, and the title-block box. Shared so the furniture lines
// (worksheetLines) and the furniture text positions (collectLabels) are computed from one
// layout and stay in step. All fields are world (Y-up) units.
type frameLayout struct {
	l, r, b, t int64 // inner frame corners (b = bottom, t = top; Y-up)
	m          int64 // inset margin
	cols, rows int64 // zone-ruler divisions
	tbx, tby   int64 // title-block box bottom-left corner
	tbw, tbh   int64 // title-block box size
}

// worksheetLayout computes the furniture layout from the sheet's page (sheet.Size). The
// second return is false when the sheet has no page, in which case there is no furniture. The
// zone-ruler column count is derived from the page size (zoneCols), so a D-size sheet gets the
// standard 8 divisions rather than a fixed count (WS7-038).
func worksheetLayout(g *geom.SchematicGeometry, sheet *geom.SheetGeometry) (frameLayout, bool) {
	sz := sheet.Size
	if sz == nil || sz.Min == nil || sz.Max == nil {
		return frameLayout{}, false
	}
	left, right := sz.Min.X, sz.Max.X
	bottom, top := sz.Min.Y, sz.Max.Y // geom is Y-up: Max.Y is the top edge
	span := right - left
	if h := top - bottom; h < span {
		span = h
	}
	m := span / 50 // 2% inset margin, in world units
	fl := frameLayout{l: left + m, r: right - m, b: bottom + m, t: top - m, m: m, cols: zoneCols(g, sheet), rows: 4}
	fl.tbw = (fl.r - fl.l) * 32 / 100
	fl.tbh = (fl.t - fl.b) * 18 / 100
	fl.tbx = fl.r - fl.tbw
	fl.tby = fl.b
	return fl, true
}

// worksheetLines synthesizes the drawing-sheet furniture in world (sheet) coordinates: the
// inner frame border, the zone-ruler tick marks (6 columns x 4 rows), and the bottom-right
// title-block box with one divider per title-block row. It is the WebGL counterpart to the
// SVG backend's drawWorksheet, but computed in world units (Y-up) so it scales with the
// schematic under the camera instead of in fixed pixels. Returns nil when the sheet has no
// page. Ruler numbers/letters and title-block field text are not geometry; the text overlay
// draws them (collectLabels / WS7-002b).
func worksheetLines(g *geom.SchematicGeometry, sheet *geom.SheetGeometry) []framePrim {
	if sheet.GetSuppressWorksheet() {
		return nil // the source format carries its own title block/frame (xschem/gEDA); WS7-036
	}
	fl, ok := worksheetLayout(g, sheet)
	if !ok {
		return nil
	}

	var out []framePrim
	rect := func(x0, y0, x1, y1 int64) {
		out = append(out, framePrim{primLineLoop, [][2]int64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}})
	}
	seg := func(x0, y0, x1, y1 int64) {
		out = append(out, framePrim{primLineStrip, [][2]int64{{x0, y0}, {x1, y1}}})
	}

	// Inner frame border.
	rect(fl.l, fl.b, fl.r, fl.t)

	// Zone ruler: tick marks at the interior column/row boundaries (numbers/letters are text).
	tick := fl.m * 3 / 5 // ~0.6 of the margin
	for i := int64(1); i < fl.cols; i++ {
		x := fl.l + (fl.r-fl.l)*i/fl.cols
		seg(x, fl.t, x, fl.t-tick) // inward from the top edge (down in Y-up)
		seg(x, fl.b, x, fl.b+tick) // inward from the bottom edge (up)
	}
	for i := int64(1); i < fl.rows; i++ {
		y := fl.b + (fl.t-fl.b)*i/fl.rows
		seg(fl.l, y, fl.l+tick, y) // inward from the left edge
		seg(fl.r, y, fl.r-tick, y) // inward from the right edge
	}

	// Title block: bottom-right box + the shared grid's row/column dividers.
	rows := titleBlockGrid(g, sheet)
	bx, by, bw, bh := titleBlockBox(fl, rows)
	rect(bx, by, bx+bw, by+bh)
	total := gridWeight(rows)
	// Row 0 is the top; geom is Y-up, so accumulate weight downward from the top edge.
	acc := 0.0
	for i, r := range rows {
		yTop := by + bh - int64(math.Round(acc/total*float64(bh)))
		acc += r.weight
		yBot := by + bh - int64(math.Round(acc/total*float64(bh)))
		if i < len(rows)-1 {
			seg(bx, yBot, bx+bw, yBot) // divider under this row
		}
		for _, c := range r.cells {
			if c.x0 > 0.0001 { // interior column divider (skip the outer left edge)
				x := bx + int64(math.Round(c.x0*float64(bw)))
				seg(x, yBot, x, yTop)
			}
		}
	}
	return out
}

// tbCell is one labeled cell of the title-block grid. text is the cell content (may be empty,
// drawn as an empty cell). x0/x1 are the cell's horizontal span as fractions of the block
// width, so 0..1 is full width. bold marks the cells KiCad emphasizes (Title, Rev), which the
// SVG backend draws heavier.
type tbCell struct {
	text   string
	x0, x1 float64
	bold   bool
}

// tbRow is one row of the title-block grid, ordered top to bottom, with a relative height
// weight (the Title row is taller) and its cells (one cell = full width; several = split).
type tbRow struct {
	cells  []tbCell
	weight float64
}

// titleBlockGrid builds KiCad's standard title-block cell layout for a sheet, filled from its
// TitleBlock and its position among g.Sheets (the Id: n/m cell). It is the single model behind
// the SVG title block (drawTitleBlock), the WebGL divider geometry (worksheetLines), and the
// WebGL text overlay (collectLabels), so all three stay in step. The grid is fixed furniture:
// every field cell is always drawn, empty where the source has no value; only the values vary.
// Order matches Eeschema, top to bottom: comments (highest index on top), company, sheet path,
// title, then the split Size|Date|Rev and the Id row.
func titleBlockGrid(g *geom.SchematicGeometry, sheet *geom.SheetGeometry) []tbRow {
	tb := sheet.TitleBlock
	title := sheet.Name
	if tb.GetTitle() != "" {
		title = tb.GetTitle()
	}
	var rows []tbRow
	full := func(text string, bold bool, weight float64) {
		rows = append(rows, tbRow{cells: []tbCell{{text: text, x0: 0, x1: 1, bold: bold}}, weight: weight})
	}

	// Comments stack above the block, highest index on top (KiCad order). Empty comment lines
	// are skipped so the block does not grow for blanks.
	comments := tb.GetComments()
	for i := len(comments) - 1; i >= 0; i-- {
		if comments[i] != "" {
			full(comments[i], false, 1)
		}
	}
	// Extra fields (Drawing, Designer, Prototype, signatures) with no typed cell render as
	// "KEY: value" rows below the comment stack, keeping their key visible.
	for _, kv := range tb.GetExtraFields() {
		full(kv.GetKey()+": "+kv.GetValue(), false, 1)
	}
	full(tb.GetCompany(), true, 1)          // Company
	full("Sheet: "+sheet.Name, false, 1)    // Sheet path
	full("Title: "+title, true, 1.7)        // Title (tall, emphasized)
	rows = append(rows, tbRow{weight: 1, cells: []tbCell{
		{text: "Size: " + paperName(g, sheet), x0: 0, x1: 0.30},
		{text: "Date: " + tb.GetDate(), x0: 0.30, x1: 0.72},
		{text: "Rev: " + tb.GetRev(), x0: 0.72, x1: 1, bold: true},
	}})
	rows = append(rows, tbRow{weight: 1, cells: []tbCell{
		{text: "", x0: 0, x1: 0.72},
		{text: "Id: " + sheetOrdinal(g, sheet), x0: 0.72, x1: 1},
	}})
	return rows
}

// gridWeight sums the row weights (the total height the block is divided into).
func gridWeight(rows []tbRow) float64 {
	var total float64
	for _, r := range rows {
		total += r.weight
	}
	return total
}

// titleBlockBox is the world-space (Y-up) box the grid fills: x,y is the bottom-left corner,
// w,h the size. The width is the frame's fixed title-block width; the height grows with the
// row count so a block with comments does not cram, but never shrinks below the base box.
func titleBlockBox(fl frameLayout, rows []tbRow) (x, y, w, h int64) {
	rowUnit := (fl.t - fl.b) / 30 // ~one row per 1/30 of the frame height
	h = int64(gridWeight(rows) * float64(rowUnit))
	if h < fl.tbh {
		h = fl.tbh
	}
	return fl.tbx, fl.tby, fl.tbw, h
}

// sheetOrdinal is the "n/m" the Id cell shows: this sheet's 1-based position among the
// design's sheets over the sheet count. Falls back to 1/1 when the sheet is not found.
func sheetOrdinal(g *geom.SchematicGeometry, sheet *geom.SheetGeometry) string {
	n, m := 1, len(g.GetSheets())
	if m == 0 {
		m = 1
	}
	for i, s := range g.GetSheets() {
		if s == sheet || (sheet.GetId() != "" && s.GetId() == sheet.GetId()) {
			n = i + 1
			break
		}
	}
	return fmt.Sprintf("%d/%d", n, m)
}

// isoPaper is the ISO/ANSI sheet-size table (landscape mm) paperName matches page dimensions
// against.
var isoPaper = []struct {
	name string
	w, h float64
}{
	{"A4", 297, 210}, {"A3", 420, 297}, {"A2", 594, 420}, {"A1", 841, 594}, {"A0", 1189, 841},
	{"A", 279.4, 215.9}, {"B", 431.8, 279.4}, {"C", 558.8, 431.8}, {"D", 863.6, 558.8}, {"E", 1117.6, 863.6},
}

// paperName infers the sheet-size name (A4, A3, ...) from the page dimensions, or "" when it
// matches no standard size or the sidecar carries no unit scale. geom.SheetGeometry has no
// paper-name field, so the Size cell is derived rather than stored (WS7-021). Dimensions are
// the page bbox scaled by g.unit_nm into millimeters, normalized to landscape.
func paperName(g *geom.SchematicGeometry, sheet *geom.SheetGeometry) string {
	sz := sheet.GetSize()
	if sz == nil || sz.GetMin() == nil || sz.GetMax() == nil || g.GetUnitNm() == 0 {
		return ""
	}
	mm := func(units int64) float64 { return float64(units) * float64(g.GetUnitNm()) / 1e6 }
	w, h := mm(sz.Max.X-sz.Min.X), mm(sz.Max.Y-sz.Min.Y)
	if h > w {
		w, h = h, w
	}
	for _, p := range isoPaper {
		if math.Abs(w-p.w) <= 3 && math.Abs(h-p.h) <= 3 {
			return p.name
		}
	}
	return ""
}

// zoneCols is the number of horizontal zone divisions in a page's standard drawing frame. An
// ASME/ISO title frame divides the sheet width into size-dependent zones (numbered right-to-left
// in the ruler); the large D/E drafting sheets use 8 divisions, verified against the native
// OrCAD D-size output (WS7-038), while smaller or unrecognized sheets keep the historical 6.
// Vertical divisions are a fixed 4 (rows A..D) across sizes.
func zoneCols(g *geom.SchematicGeometry, sheet *geom.SheetGeometry) int64 {
	switch paperName(g, sheet) {
	case "D", "E":
		return 8
	default:
		return 6
	}
}

// zoneNumber is the ruler number drawn in column i (0-based, left to right) of a cols-wide zone
// grid. The standard frame numbers zones right-to-left (leftmost = cols, rightmost = 1).
func zoneNumber(cols, i int64) int64 {
	return cols - i
}

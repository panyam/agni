package render

import (
	"math"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// freeTextFit returns the font-size shrink factor (0 < f <= 1) for the multi-line free-text
// columns of a documentation page (an OrCAD table-of-contents sheet: several fixed-origin columns
// of stacked text inside a sized worksheet). One factor is computed for the whole page — the
// minimum any column needs so its widest line fits the horizontal gap to the next column (or the
// frame margin) and its stack clears the corner title block — and applied uniformly, so the
// columns stay one size the way the source draws them rather than each shrinking independently.
// Shrinking the font also shortens the line stack, so the fix holds in every renderer including
// rsvg PNG exports (unlike textLength condensing, which only the browser honors) — WS7-038.
//
// The fit applies ONLY to a documentation page (no component placements). A schematic sheet carries
// stand-alone multi-line annotation notes that are not document columns, and squeezing one against
// its neighbor or the frame margin shrinks legible text to an unreadable blob; the placement gate
// keeps the fit off every schematic sheet (WS7-041 — verified over the 82-sheet automotive
// design: the fit engaged on 18 schematic notes, some down to 0.3x, and only the TOC benefits).
//
// The result maps each multi-line free-text label to that shared factor; it is nil when nothing
// overflows (every column fits), for single-line text, on a schematic sheet, or on a sheet with no
// page. Both backends key on the same sheet.Labels pointers, so the SVG renderer and the WebGL
// overlay apply the identical factor and stay in step.
func freeTextFit(g *geom.SchematicGeometry, sheet *geom.SheetGeometry) map[*geom.Label]float64 {
	if len(sheet.GetPlacements()) > 0 {
		return nil // a schematic sheet: its multi-line text is annotation notes, not doc columns
	}
	if sheet.GetSuppressWorksheet() {
		return nil // a source with its own frame/title block (xschem/gEDA); no synthesized layout
	}
	fl, ok := worksheetLayout(g, sheet)
	if !ok {
		return nil // no page, so no frame edge or title block to fit against
	}

	// One column's world-space extent. Text is left-anchored at (x, top) and each further line
	// sits one step lower (smaller Y, geom is Y-up); bot is the last line's baseline.
	type col struct {
		l        *geom.Label
		x        int64   // origin X (left edge for the common left-anchored free text)
		top, bot int64   // vertical span in world Y (top >= bot)
		width    float64 // widest line's natural width in world units
		lines    int
		effH     int64 // effective world text height (falls back to the sheet default)
	}
	def := defaultTextHeight(g, sheet)
	var cols []col
	for _, l := range sheet.Labels {
		if l.Origin == nil {
			continue
		}
		effH := l.Height
		if effH <= 0 {
			effH = def
		}
		text := l.Text
		lines := strings.Count(text, "\n") + 1
		step := int64(float64(effH) * lineHeight)
		widest := 0
		for _, line := range strings.Split(text, "\n") {
			if n := len([]rune(line)); n > widest {
				widest = n
			}
		}
		cols = append(cols, col{
			l:     l,
			x:     l.Origin.X,
			top:   l.Origin.Y,
			bot:   l.Origin.Y - int64(lines-1)*step,
			width: monoAdvanceEm * float64(effH) * float64(widest),
			lines: lines,
			effH:  effH,
		})
	}

	// Title-block clearance references: its top edge and left edge in world coordinates. A column
	// whose text reaches into the title-block x-range and drops below its top edge must be shortened.
	tbx, tby, _, tbh := titleBlockBox(fl, titleBlockGrid(g, sheet))
	tbTop := tby + tbh

	// The source draws every column in one font, so the fix is one shrink factor applied
	// uniformly to all multi-line free text: the minimum any column needs, so the widest/tallest
	// column fits and the columns stay the same size (as the native output draws them), rather
	// than each column shrinking independently to a different size.
	global := 1.0
	var multiline []*geom.Label
	for i, a := range cols {
		if a.lines <= 1 || a.width <= 0 {
			continue // single-line free text (net-stub labels) is left alone
		}
		multiline = append(multiline, a.l)

		// Horizontal: budget to the nearest column origin on the right whose vertical span
		// overlaps this one, else the inner frame's right edge. Leave one glyph of gutter so the
		// text does not butt against the neighbor.
		budget := float64(fl.r - a.x)
		for j, b := range cols {
			if j == i || b.x <= a.x {
				continue
			}
			if a.bot > b.top || b.bot > a.top { // no vertical overlap
				continue
			}
			if gap := float64(b.x - a.x); gap < budget {
				budget = gap
			}
		}
		gutter := monoAdvanceEm * float64(a.effH)
		if avail := budget - gutter; avail > 0 && a.width > avail {
			global = math.Min(global, avail/a.width)
		}

		// Vertical: if the column sits over the title block and its stack drops below the
		// title-block top, shrink so it clears. availV is the room from the origin down to that top.
		reachesTB := float64(a.x)+a.width > float64(tbx)
		if reachesTB && a.bot < tbTop {
			availV := float64(a.top - tbTop)
			fullV := float64(a.top - a.bot)
			if availV > 0 && fullV > availV {
				global = math.Min(global, availV/fullV)
			}
		}
	}
	if global >= 1 {
		return nil
	}
	out := map[*geom.Label]float64{}
	for _, l := range multiline {
		out[l] = global
	}
	return out
}

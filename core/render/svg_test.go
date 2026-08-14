package render

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestSheetSVG_DrawsTextLayers renders a placement whose symbol carries a static
// annotation and a labeled pin, plus a ref-des sheet label, and asserts the annotation
// text, pin number, and ref-des all reach the output as text. Ref-des is a sheet label now
// (emitted by the readers at the field position), not drawn from pl.RefDes at the origin.
func TestSheetSVG_DrawsTextLayers(t *testing.T) {
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{
			CellRef: "TB", LibraryRef: "L", ViewRef: "sym",
			Bbox:   &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 400, Y: 200}},
			Shapes: []*geom.Shape{{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{}, {X: 400, Y: 200}}}},
			Pins: []*geom.PinPoint{{
				PortRef: "A7", Loc: &geom.Point{X: 0, Y: 100},
				LabelOrigin: &geom.Point{X: 30, Y: 100}, Justify: "left",
			}},
			Annotations: []*geom.Label{{Text: "SHEET-42", Origin: &geom.Point{X: 20, Y: 150}, Justify: "left"}},
		}},
		Sheets: []*geom.SheetGeometry{{
			Placements: []*geom.SymbolPlacement{{
				RefDes: "TB1", CellRef: "TB", LibraryRef: "L", ViewRef: "sym",
				Transform: &geom.Transform{Origin: &geom.Point{X: 100, Y: 100}},
			}},
			Labels: []*geom.Label{{Text: "TB1", Origin: &geom.Point{X: 120, Y: 210}}},
		}},
	}
	out := SheetSVG(g, g.Sheets[0])
	for _, want := range []string{">SHEET-42<", ">A7<", ">TB1<"} {
		if !strings.Contains(out, want) {
			t.Errorf("SVG missing text %q", want)
		}
	}
}

// textElem returns the <text ...>content</text> element whose content is exactly want, or "".
func textElem(svgOut, want string) string {
	re := regexp.MustCompile(`<text[^>]*>` + regexp.QuoteMeta(want) + `</text>`)
	return re.FindString(svgOut)
}

// textY reads the y attribute of a <text ...> element, or NaN.
func textY(elem string) float64 {
	m := regexp.MustCompile(`\by="([-\d.]+)"`).FindStringSubmatch(elem)
	if m == nil {
		return math.NaN()
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return v
}

// TestSheetSVG_MultiLineLabel: a label carrying newlines (EDIF %10% decodes to '\n', e.g. a
// table-of-contents sheet list) is drawn as one stacked <text> per line, not a single overlapping
// run. Guards the multi-line path in drawText.
func TestSheetSVG_MultiLineLabel(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Labels: []*geom.Label{{Text: "LINE1\nLINE2\nLINE3", Origin: &geom.Point{X: 100, Y: 100}, Height: 10}},
		}},
	}
	out := SheetSVG(g, g.Sheets[0])
	e1, e2, e3 := textElem(out, "LINE1"), textElem(out, "LINE2"), textElem(out, "LINE3")
	if e1 == "" || e2 == "" || e3 == "" {
		t.Fatalf("each line must be its own <text>; got %q / %q / %q", e1, e2, e3)
	}
	y1, y2, y3 := textY(e1), textY(e2), textY(e3)
	if !(y1 < y2 && y2 < y3) {
		t.Errorf("lines must stack downward: y1=%g y2=%g y3=%g", y1, y2, y3)
	}
	if strings.Contains(out, "LINE1\nLINE2") {
		t.Error("a single <text> still carries the raw newline; it was not split")
	}
}

// TestReadableText pins the readable-text rule: text whose angle would read upside down
// (normalized magnitude > 90) is turned a further 180 and its justify flipped on both axes,
// while upright and vertical (+/-90) text is left untouched so KiCad-90 parity is preserved.
func TestReadableText(t *testing.T) {
	cases := []struct {
		name       string
		inDeg      int32
		inJustify  string
		wantDeg    int32
		wantJustif string
	}{
		{"upright", 0, "left bottom", 0, "left bottom"},
		{"cw90 kept", 270, "left", 270, "left"}, // normalizes to -90, vertical, kept
		{"ccw90 kept", 90, "right top", 90, "right top"},
		{"r180 flips", 180, "left bottom", 360, "right top"},
		{"neg180 flips", -180, "right top", 0, "left bottom"},
		{"r135 flips", 135, "left", 315, "right"},
		{"center axis survives flip", 180, "left", 360, "right"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotDeg, gotJustify := readableText(c.inDeg, c.inJustify)
			if normDeg(gotDeg) != normDeg(c.wantDeg) || gotJustify != c.wantJustif {
				t.Errorf("readableText(%d, %q) = (%d, %q), want (%d, %q)",
					c.inDeg, c.inJustify, gotDeg, gotJustify, c.wantDeg, c.wantJustif)
			}
		})
	}
}

// TestSheetSVG_UprightText is the regression guard for the upside-down text bug seen on a
// real-corpus headers sheet: a symbol placed at R180 (or text with its own R180 orientation)
// was drawn genuinely upside down instead of flipped upright the way every EDA viewer draws
// it. The rule the render layer enforces: no text run ever carries a rotate(180 ...).
func TestSheetSVG_UprightText(t *testing.T) {
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{
			CellRef: "CONN", LibraryRef: "L", ViewRef: "sym",
			Bbox:   &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 100, Y: 100}},
			Shapes: []*geom.Shape{{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{}, {X: 100, Y: 100}}}},
			Pins: []*geom.PinPoint{{
				PortRef: "P1", Loc: &geom.Point{X: 0, Y: 70},
				LabelOrigin: &geom.Point{X: 10, Y: 70}, Justify: "left",
			}},
		}},
		Sheets: []*geom.SheetGeometry{{
			Size: &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 2000, Y: 1000}},
			Placements: []*geom.SymbolPlacement{{
				RefDes: "X4", CellRef: "CONN", LibraryRef: "L", ViewRef: "sym",
				Transform: &geom.Transform{Origin: &geom.Point{X: 400, Y: 500}, RotationDeg: 180},
				Fields:    []*geom.Field{{Name: "Reference", Value: "X4", Origin: &geom.Point{X: 400, Y: 500}, Visible: true, Height: 20, Justify: "left"}},
			}},
			// A net label carrying its own R180 text orientation (an off-page "pin" stub).
			Labels: []*geom.Label{{Text: "NET_FLIP", Origin: &geom.Point{X: 400, Y: 850}, Height: 20, RotationDeg: 180, Justify: "left"}},
		}},
	}
	out := SheetSVG(g, g.Sheets[0])

	if strings.Contains(out, "rotate(180 ") {
		t.Errorf("SVG has upside-down text (rotate(180 ...)):\n%s", out)
	}
	// The R180 field's justify must flip so the run still hangs off the same corner: a left
	// justify becomes a right (text-anchor=end) rather than staying left.
	if el := textElem(out, "X4"); el == "" {
		t.Fatal("X4 ref-des text element not found")
	} else if !strings.Contains(el, `text-anchor="end"`) {
		t.Errorf("flipped ref-des should re-anchor to end, got %q", el)
	}
}

// TestSheetSVG_AnnotationCondensed guards the caption-overflow fix (a "Net Splitter" label on
// a real-corpus headers sheet): a symbol annotation wider than its drawn BOX is condensed to
// the box width with textLength + lengthAdjust rather than spilling past it, while a short caption
// (narrower than the box) is left at natural width so it is not stretched to fill the box.
func TestSheetSVG_AnnotationCondensed(t *testing.T) {
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{
			CellRef: "NS", LibraryRef: "L", ViewRef: "sym",
			Bbox: &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 1778, Y: 480}},
			Shapes: []*geom.Shape{
				{Kind: geom.Shape_KIND_RECT, FigureGroup: "BOX", Points: []*geom.Point{{X: 254}, {X: 1524, Y: 480}}},
			},
			Annotations: []*geom.Label{
				{Text: "Net Splitter", Origin: &geom.Point{X: 889, Y: 240}, Justify: "center center"}, // long: overflows
				{Text: "Ok", Origin: &geom.Point{X: 889, Y: 100}, Justify: "center center"},           // short: fits
			},
		}},
		// A large sheet so the ~1270-wide symbol box is a small fraction of it (as on a real
		// sheet), giving the box a small pixel width the long caption overflows.
		Sheets: []*geom.SheetGeometry{{
			Size: &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 84000, Y: 50000}},
			Placements: []*geom.SymbolPlacement{{
				RefDes: "U1", CellRef: "NS", LibraryRef: "L", ViewRef: "sym",
				Transform: &geom.Transform{Origin: &geom.Point{}},
			}},
		}},
	}
	out := SheetSVG(g, g.Sheets[0])

	if el := textElem(out, "Net Splitter"); el == "" {
		t.Fatal("Net Splitter annotation not found")
	} else if !strings.Contains(el, "textLength=") || !strings.Contains(el, `lengthAdjust="spacingAndGlyphs"`) {
		t.Errorf("overflowing caption should be condensed with textLength, got %q", el)
	}
	if el := textElem(out, "Ok"); el == "" {
		t.Fatal("Ok annotation not found")
	} else if strings.Contains(el, "textLength=") {
		t.Errorf("short caption should not be condensed, got %q", el)
	}
}

// TestSheetSVG_PinNameAndRotation covers two KiCad-faithfulness fixes: the pin NAME renders
// (distinct from the number, WS7-023), and a placement's rotation propagates to its field and
// pin text (WS7-024). A symbol placed at geom rotation 270 (KiCad 90) must draw its text with
// rotate(-90 ...), matching kicad-cli; a placement at rotation 0 must draw text with no
// transform.
func TestSheetSVG_PinNameAndRotation(t *testing.T) {
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{
			CellRef: "DUAL", LibraryRef: "L", ViewRef: "2",
			Bbox:   &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 400, Y: 200}},
			Shapes: []*geom.Shape{{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{{}, {X: 400}}}},
			Pins: []*geom.PinPoint{{
				PortRef: "2", Name: "Y", Loc: &geom.Point{X: 0, Y: 100},
				LabelOrigin: &geom.Point{X: 0, Y: 100}, Justify: "left",
			}},
		}},
		Sheets: []*geom.SheetGeometry{{
			Placements: []*geom.SymbolPlacement{
				{
					RefDes: "U1", CellRef: "DUAL", LibraryRef: "L", ViewRef: "2",
					Transform: &geom.Transform{Origin: &geom.Point{X: 100, Y: 100}, RotationDeg: 270},
					Fields:    []*geom.Field{{Name: "Value", Value: "DUAL", Origin: &geom.Point{X: 120, Y: 90}, Visible: true, Height: 1_270_000}},
				},
				{
					RefDes: "R1", CellRef: "DUAL", LibraryRef: "L", ViewRef: "2",
					Transform: &geom.Transform{Origin: &geom.Point{X: 300, Y: 100}, RotationDeg: 0},
					Fields:    []*geom.Field{{Name: "Value", Value: "FLAT", Origin: &geom.Point{X: 320, Y: 90}, Visible: true, Height: 1_270_000}},
				},
			},
		}},
	}
	out := SheetSVG(g, g.Sheets[0])

	// B: the pin name "Y" reaches the output as its own text run.
	if !strings.Contains(out, ">Y<") {
		t.Error("pin name \"Y\" missing from SVG")
	}

	// C: the rotated placement's field text carries rotate(-90 ...).
	dual := textElem(out, "DUAL")
	if dual == "" {
		t.Fatal("DUAL field text element not found")
	}
	if !strings.Contains(dual, "rotate(-90 ") {
		t.Errorf("DUAL field should rotate -90 with its symbol, got %q", dual)
	}

	// C: the unrotated placement's field text carries no transform.
	flat := textElem(out, "FLAT")
	if flat == "" {
		t.Fatal("FLAT field text element not found")
	}
	if strings.Contains(flat, "transform=") {
		t.Errorf("unrotated field should have no transform, got %q", flat)
	}
}

// TestSheetSVG_Worksheet asserts a sheet with a page size and title block draws the worksheet
// frame + title-block fields, and that a sheet without a page (an auto-layout graph) does not.
func TestSheetSVG_Worksheet(t *testing.T) {
	withPage := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{
		Name:       "S1",
		Size:       &geom.BBox{Min: &geom.Point{X: 0, Y: -2100}, Max: &geom.Point{X: 2970, Y: 0}},
		TitleBlock: &geom.TitleBlock{Title: "My Board", Rev: "3"},
	}}}
	out := SheetSVG(withPage, withPage.Sheets[0])
	for _, want := range []string{"Title: My Board", "Rev: 3", ">1<", ">A<"} {
		if !strings.Contains(out, want) {
			t.Errorf("worksheet SVG missing %q", want)
		}
	}

	noPage := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Name: "graph"}}}
	if strings.Contains(SheetSVG(noPage, noPage.Sheets[0]), "Title:") {
		t.Error("a sheet with no page size should not draw a title block")
	}
}

// TestSheetSVG_PinDotsOptional covers WS7-017: the per-pin verification dot (the Pin color)
// is omitted by default so a faithful render looks like Eeschema, is drawn under WithPinDots
// for the eyeball check, and a real junction dot (a sheet Shape) draws in both modes.
func TestSheetSVG_PinDotsOptional(t *testing.T) {
	pinFill := `fill="` + DefaultStyle.Pin + `"`
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{
			CellRef: "R", LibraryRef: "L", ViewRef: "sym",
			Bbox:   &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 400, Y: 200}},
			Shapes: []*geom.Shape{{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{}, {X: 400, Y: 200}}}},
			Pins:   []*geom.PinPoint{{PortRef: "1", Loc: &geom.Point{X: 0, Y: 100}}},
		}},
		Sheets: []*geom.SheetGeometry{{
			Placements: []*geom.SymbolPlacement{{
				RefDes: "R1", CellRef: "R", LibraryRef: "L", ViewRef: "sym",
				Transform: &geom.Transform{Origin: &geom.Point{X: 100, Y: 100}},
			}},
			// A real junction dot on the sheet (drawn with the Free color), not a pin dot.
			Shapes: []*geom.Shape{{Kind: geom.Shape_KIND_DOT, Points: []*geom.Point{{X: 100, Y: 100}}}},
		}},
	}

	def := SheetSVG(g, g.Sheets[0])
	if strings.Contains(def, pinFill) {
		t.Errorf("default render should draw no pin dots (%s), got one", pinFill)
	}
	// The junction dot uses the Free color and must survive the default render.
	if !strings.Contains(def, `fill="`+DefaultStyle.Free+`"`) {
		t.Error("default render dropped the real junction dot")
	}

	on := SheetSVG(g, g.Sheets[0], WithPinDots())
	if !strings.Contains(on, pinFill) {
		t.Errorf("WithPinDots render should draw the pin dot (%s)", pinFill)
	}
}

// TestSheetSVG_TitleBlockGrid covers WS7-021: the title block renders KiCad's fixed cell grid
// (Title / Size / Date / Rev / Company / Sheet / comments / Id), values filled from TitleBlock,
// the Size cell derived from the A4 page, Id showing this sheet's n-of-m, empty cells still
// drawing their label, and comments stacking highest-index first.
func TestSheetSVG_TitleBlockGrid(t *testing.T) {
	a4 := &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 297, Y: 210}}
	g := &geom.SchematicGeometry{
		UnitNm: 1_000_000, // 1 unit = 1 mm, so 297x210 units resolves to A4
		Sheets: []*geom.SheetGeometry{
			{
				Name: "Root", Size: a4,
				TitleBlock: &geom.TitleBlock{
					Title: "My Board", Rev: "C3", Date: "2026-07-09", Company: "Acme",
					Comments:    []string{"first", "second"},
					ExtraFields: []*geom.KeyValue{{Key: "DRAWING", Value: "DEMO-100-SCH"}},
				},
			},
			{Name: "Sub", Size: a4, TitleBlock: &geom.TitleBlock{Title: "Sub"}},
		},
	}

	root := SheetSVG(g, g.Sheets[0])
	for _, want := range []string{
		">Title: My Board<", ">Rev: C3<", ">Date: 2026-07-09<", ">Acme<",
		">first<", ">second<", ">Size: A4<", ">Id: 1/2<",
		">DRAWING: DEMO-100-SCH<",
	} {
		if !strings.Contains(root, want) {
			t.Errorf("root title block missing %q", want)
		}
	}
	// Comments stack highest-index on top: "second" sits above "first".
	if strings.Index(root, ">second<") > strings.Index(root, ">first<") {
		t.Error("comment order wrong: comment 2 should render above comment 1")
	}

	// The sub-sheet has only a title: the other cells still draw their labels (empty values),
	// and Id reflects its position.
	sub := SheetSVG(g, g.Sheets[1])
	for _, want := range []string{">Title: Sub<", ">Rev: <", ">Date: <", ">Id: 2/2<", ">Size: A4<"} {
		if !strings.Contains(sub, want) {
			t.Errorf("sub-sheet title block missing empty-cell label %q", want)
		}
	}
}

func TestLabelFontHonorsSourceHeight(t *testing.T) {
	// Intentionally-tiny source text (KiCad's 0.254mm footprint field) must NOT be clamped up
	// to a cluttering size; an unspecified height falls back to the SHEET's own default rather
	// than a fixed pixel size; an absurd height is capped relative to the drawing.
	const def = 100 // the sheet's median text height, in source units
	if got := labelFont(13, def, 0.1); got > 2 {
		t.Errorf("tiny text (13 units at scale 0.1) -> %v, want it kept small (not inflated)", got)
	}
	// Unspecified height uses def, so it tracks the sheet instead of meaning a different
	// physical size on every drawing (the old flat 7px).
	if got := labelFont(0, def, 0.1); got != 10 {
		t.Errorf("unspecified height -> %v, want 10 (def=100 at scale 0.1)", got)
	}
	if got := labelFont(100, def, 0.1); got != 10 {
		t.Errorf("normal height -> %v, want 10", got)
	}
	// The ceiling is a fraction of the drawing, so it catches a bogus height without
	// truncating a legitimately large title (which measured 2.2% of its sheet).
	if got := labelFont(1_000_000, def, 0.1); got != sheetMaxPx*maxFontFrac {
		t.Errorf("absurd height -> %v, want capped at %v", got, sheetMaxPx*maxFontFrac)
	}
	if got := labelFont(1, def, 0.0001); got != sheetMaxPx*minFontFrac {
		t.Errorf("sub-visible height -> %v, want floored at %v", got, sheetMaxPx*minFontFrac)
	}
}

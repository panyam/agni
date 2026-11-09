package render

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/edif"
)

// textFontSize reads the font-size attribute of a <text ...> element, or NaN.
func textFontSize(elem string) float64 {
	m := regexp.MustCompile(`font-size="([-\d.]+)"`).FindStringSubmatch(elem)
	if m == nil {
		return math.NaN()
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return v
}

// TestSheetSVG_FreeTextColumnsFit covers WS7-038's width dimension on a documentation page.
// Imagine one multi-line free-text column overflows the gap to the frame edge. Every column then
// shrinks by the same factor, below its natural font size, so the widest fits and the columns keep
// one uniform size. The native output draws them that way. The fix is a font shrink, not
// textLength, so it also holds in rsvg PNG exports, and the test asserts no textLength attribute
// is emitted.
func TestSheetSVG_FreeTextColumnsFit(t *testing.T) {
	wideLine := "WIDE" + strings.Repeat("X", 76) // 80 runes: 0.6*120*80 = 5760 world units
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Size: &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 10000, Y: 8000}},
			Labels: []*geom.Label{
				// A narrow left column. It would fit on its own, but shrinks with the rest.
				{Text: "NARROW\nN2", Origin: &geom.Point{X: 500, Y: 7000}, Height: 120, Justify: "left"},
				// The rightmost column, whose widest line exceeds the gap to the right frame edge.
				{Text: wideLine + "\nW2", Origin: &geom.Point{X: 5000, Y: 7000}, Height: 120, Justify: "left"},
			},
		}},
	}
	out := SheetSVG(g, g.Sheets[0])

	if strings.Contains(out, "textLength") {
		t.Error("free-text fit must shrink the font (universal), not condense with textLength")
	}

	// The natural font size both columns would draw at without the fit.
	fr := frameSheet(g.Sheets[0], indexSymbols(g))
	natural := labelFont(120, defaultTextHeight(g, g.Sheets[0]), fr.scale)

	narrow := textElem(out, "NARROW")
	wide := textElem(out, wideLine)
	if narrow == "" || wide == "" {
		t.Fatalf("column first lines not found: narrow=%q wide=%q", narrow, wide)
	}
	nf, wf := textFontSize(narrow), textFontSize(wide)
	if !(wf < natural) {
		t.Errorf("overflowing column should shrink below natural font %.2f, got %.2f", natural, wf)
	}
	if math.Abs(nf-wf) > 0.05 {
		t.Errorf("all columns should shrink to one uniform size, got narrow=%.2f wide=%.2f", nf, wf)
	}
}

// TestFreeTextFitLeavesFittingColumnsAlone checks that a page whose multi-line columns all fit
// produces no shrink factor, so the free text renders at its natural size.
func TestFreeTextFitLeavesFittingColumnsAlone(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Size: &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 20000, Y: 8000}},
			Labels: []*geom.Label{
				{Text: "A\nB", Origin: &geom.Point{X: 500, Y: 7000}, Height: 100, Justify: "left"},
				{Text: "C\nD", Origin: &geom.Point{X: 9000, Y: 7000}, Height: 100, Justify: "left"},
			},
		}},
	}
	if fit := freeTextFit(g, g.Sheets[0]); len(fit) != 0 {
		t.Errorf("columns that already fit must not be shrunk; got %d entries", len(fit))
	}
}

// TestFreeTextFitClearsTitleBlock covers WS7-038's height dimension. A tall free-text column
// stands over the corner title block, and the fit shrinks it so its stacked lines clear the
// title-block top instead of dropping into it. The test computes the clearance from the renderer's
// own worksheet layout. So the assertion is on the geometric goal, that the shrunk stack is above
// the title block, and not on the mechanism.
func TestFreeTextFitClearsTitleBlock(t *testing.T) {
	const top, x, h, lines = int64(7500), int64(7000), int64(200), 30
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Size:       &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 10000, Y: 8000}},
			TitleBlock: &geom.TitleBlock{Title: "T"},
			Labels: []*geom.Label{{
				Text:   strings.TrimRight(strings.Repeat("ABCDE\n", lines), "\n"),
				Origin: &geom.Point{X: x, Y: top}, Height: h, Justify: "left",
			}},
		}},
	}
	l := g.Sheets[0].Labels[0]

	fit := freeTextFit(g, g.Sheets[0])
	f, ok := fit[l]
	if !ok || !(f < 1) {
		t.Fatalf("tall column over the title block should be shrunk; factor present=%v value=%v", ok, f)
	}

	fl, _ := worksheetLayout(g, g.Sheets[0])
	_, tby, _, tbh := titleBlockBox(fl)
	tbTop := tby + tbh

	step := int64(float64(h) * lineHeight)
	fullV := float64(int64(lines-1) * step)
	postBot := top - int64(fullV*f)
	if postBot < tbTop-1 { // -1 for int rounding
		t.Errorf("shrunk column bottom %d should clear the title-block top %d", postBot, tbTop)
	}
}

// TestFreeTextFitSingleLineUntouched guards the net-stub safety gate. Off-page connectors carry a
// single-line net name, and single-line page labels are never in the fit map. So the free-text
// column fit touches only multi-line documentation text.
func TestFreeTextFitSingleLineUntouched(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Size: &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 2000, Y: 1000}},
			Labels: []*geom.Label{
				{Text: "A_VERY_LONG_SINGLE_LINE_NET_NAME", Origin: &geom.Point{X: 100, Y: 800}, Height: 20},
			},
		}},
	}
	if fit := freeTextFit(g, g.Sheets[0]); len(fit) != 0 {
		t.Errorf("single-line labels must not be fitted; got %d entries", len(fit))
	}
}

// TestFreeTextFitSkipsSchematicSheet guards WS7-041. The free-text column fit is a
// documentation-page feature, for an OrCAD table-of-contents style sheet with no component
// placements. A schematic sheet's multi-line text is stand-alone annotation notes, not document
// columns. Squeezing those notes against a neighbor or the frame margin shrinks legible text to an
// unreadable blob. Over a real industrial design the fit engaged on 18 schematic notes, some to
// 0.3x. The fit must skip any sheet that has component placements.
func TestFreeTextFitSkipsSchematicSheet(t *testing.T) {
	wideLine := "WIDE" + strings.Repeat("X", 76) // overflows the gap to the frame edge
	labels := func() []*geom.Label {
		return []*geom.Label{
			{Text: "NARROW\nN2", Origin: &geom.Point{X: 500, Y: 7000}, Height: 120, Justify: "left"},
			{Text: wideLine + "\nW2", Origin: &geom.Point{X: 5000, Y: 7000}, Height: 120, Justify: "left"},
		}
	}
	size := func() *geom.BBox { return &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 10000, Y: 8000}} }

	// Documentation page: no placements -> the overflowing columns are fitted.
	doc := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Size: size(), Labels: labels()}}}
	if fit := freeTextFit(doc, doc.Sheets[0]); len(fit) == 0 {
		t.Fatal("a documentation page (no placements) with overflowing columns should be fitted")
	}

	// Schematic page: identical overflowing text, but the sheet has a component placement, so the
	// text is an annotation note and must keep its natural size.
	sch := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{
		Size:       size(),
		Labels:     labels(),
		Placements: []*geom.SymbolPlacement{{}},
	}}}
	if fit := freeTextFit(sch, sch.Sheets[0]); len(fit) != 0 {
		t.Errorf("a schematic sheet (has placements) must skip the free-text fit; got %d entries", len(fit))
	}
}

// TestFreeTextFitDemoFixture drives the same scope rule end to end through the real EDIF reader on
// the redistributable freetext-fit-demo.eds fixture. Its "NOTES ON A SCHEMATIC" page carries two
// component placements plus an overflowing multi-line note, and its "CONTENTS" page is a
// documentation page (no placements) with an overflowing column. The schematic page must skip the
// fit, so its note keeps its natural, legible size. The documentation page must still get fitted.
func TestFreeTextFitDemoFixture(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "readers", "edif", "testdata", "freetext-fit-demo.eds"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := edif.ReadSchematic(f, "freetext-fit-demo.eds")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*geom.SheetGeometry{}
	for _, s := range g.GetSheets() {
		byName[s.GetName()] = s
	}
	sch, ok := byName["NOTES ON A SCHEMATIC"]
	if !ok {
		t.Fatal("fixture missing the schematic sheet")
	}
	if len(sch.GetPlacements()) == 0 {
		t.Fatal("schematic sheet should have component placements")
	}
	if fit := freeTextFit(g, sch); len(fit) != 0 {
		t.Errorf("schematic sheet must skip the fit; got %d fitted labels", len(fit))
	}
	doc, ok := byName["CONTENTS"]
	if !ok {
		t.Fatal("fixture missing the contents sheet")
	}
	if len(doc.GetPlacements()) != 0 {
		t.Fatal("contents sheet should have no placements")
	}
	if fit := freeTextFit(g, doc); len(fit) == 0 {
		t.Error("documentation page with an overflowing column should still be fitted")
	}
}

// TestWorksheetZoneGridBySize covers WS7-038's frame fidelity. A D-size sheet's zone ruler has 8
// divisions numbered right-to-left, 8 on the left and 1 on the right. An unrecognized page keeps
// the historical 6, also numbered right-to-left.
func TestWorksheetZoneGridBySize(t *testing.T) {
	dSize := &geom.SchematicGeometry{
		UnitNm: 1_000_000, // 1 unit = 1 mm
		Sheets: []*geom.SheetGeometry{{
			Size:       &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 864, Y: 559}}, // ~D landscape
			TitleBlock: &geom.TitleBlock{Title: "D"},
		}},
	}
	out := SheetSVG(dSize, dSize.Sheets[0])
	for _, want := range []string{">8<", ">7<", ">1<"} {
		if !strings.Contains(out, want) {
			t.Errorf("D-size zone ruler missing %q", want)
		}
	}
	if strings.Contains(out, ">9<") {
		t.Error("D-size zone ruler should stop at 8")
	}
	// Right-to-left: 8 is drawn to the left of 1.
	if strings.Index(out, ">8<") > strings.Index(out, ">1<") {
		t.Error("zones must number right-to-left: 8 left of 1")
	}

	other := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Size:       &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 5000, Y: 4000}}, // no standard match
			TitleBlock: &geom.TitleBlock{Title: "X"},
		}},
	}
	o := SheetSVG(other, other.Sheets[0])
	if !strings.Contains(o, ">6<") || strings.Contains(o, ">8<") {
		t.Error("an unrecognized page should keep the 6-zone ruler")
	}
}

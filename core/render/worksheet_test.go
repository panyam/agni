package render

import (
	"math"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// sheetWithPage builds a minimal one-label sheet with a page size, so the synthetic worksheet
// (frame + ruler + title block) has somewhere to draw.
func sheetWithPage(suppress bool) *geom.SchematicGeometry {
	return &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Name:              "S1",
			Size:              &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: 10000, Y: 8000}},
			TitleBlock:        &geom.TitleBlock{Title: "MyTitle"},
			SuppressWorksheet: suppress,
			Labels:            []*geom.Label{{Text: "N1", Origin: &geom.Point{X: 5000, Y: 4000}}},
		}},
	}
}

// TestWorksheetSuppressed checks a sheet that has its own title block, which xschem/gEDA mark
// with SuppressWorksheet. That sheet draws no synthetic worksheet furniture, while an
// unsuppressed sheet does (WS7-036). The title-block field text is the tell, and the schematic
// content renders either way.
func TestWorksheetSuppressed(t *testing.T) {
	gOn := sheetWithPage(false)
	on := SheetSVG(gOn, gOn.Sheets[0])
	if !strings.Contains(on, "MyTitle") || !strings.Contains(on, "Title:") {
		t.Errorf("unsuppressed sheet should draw the synthetic title block; got no title text")
	}

	gOff := sheetWithPage(true)
	off := SheetSVG(gOff, gOff.Sheets[0])
	if strings.Contains(off, "MyTitle") || strings.Contains(off, "Title:") || strings.Contains(off, "Id: ") {
		t.Errorf("suppressed sheet should draw no synthetic worksheet; found title-block text")
	}
	if !strings.Contains(off, ">N1<") {
		t.Errorf("suppressed sheet dropped the schematic label N1")
	}
}

// Both renderers draw one title block (agni issue 765). SheetSVG draws it in pixels, and the packed
// sheet in world units, which the sheet's scale turns back into pixels. Before, WebGL's block took
// 32% of the frame's width where SVG's took 36%, and its text sized by another rule.
func TestTitleBlockMatchesAcrossRenderers(t *testing.T) {
	for _, page := range []struct {
		name string
		w, h int64
	}{{"A4", 297_000_000, 210_000_000}, {"A0", 1_189_000_000, 841_000_000}} {
		g := sheetWithPage(false)
		sh := g.Sheets[0]
		sh.Size = &geom.BBox{Min: &geom.Point{}, Max: &geom.Point{X: page.w, Y: page.h}}
		sh.Labels[0].Origin = &geom.Point{X: page.w / 2, Y: page.h / 2}

		doc := SheetSVG(g, sh)
		var svgW, svgH, svgFont float64
		for _, line := range strings.Split(doc, "\n") {
			if strings.Contains(line, `fill="`+DefaultStyle.TitleBlockFill+`"`) && strings.HasPrefix(strings.TrimSpace(line), "<rect") {
				svgW, svgH = attrF(t, line, "width"), attrF(t, line, "height")
			}
			if strings.Contains(line, ">Title: MyTitle<") {
				svgFont = attrF(t, line, "font-size")
			}
		}
		if svgW == 0 || svgFont == 0 {
			t.Fatalf("%s: SheetSVG drew no title block", page.name)
		}

		fl, _ := worksheetLayout(g, sh)
		if got := float64(fl.tbw) * fl.scale; math.Abs(got-svgW) > 0.5 {
			t.Errorf("%s: packed title block is %.1f px wide, SVG's %.1f", page.name, got, svgW)
		}
		if got := float64(fl.tbh) * fl.scale; math.Abs(got-svgH) > 0.5 {
			t.Errorf("%s: packed title block is %.1f px tall, SVG's %.1f", page.name, got, svgH)
		}
		var font float64
		for _, l := range PackSheet(g, sh).GetLabels() {
			if l.GetText() == "Title: MyTitle" {
				font = float64(l.GetHeight()) * fl.scale
			}
		}
		if math.Abs(font-svgFont) > 0.5 {
			t.Errorf("%s: packed title text is %.1f px, SVG's %.1f", page.name, font, svgFont)
		}
	}
}

// attrF reads a numeric attribute off one SVG element line.
func attrF(t *testing.T, line, name string) float64 {
	t.Helper()
	i := strings.Index(line, " "+name+`="`)
	if i < 0 {
		t.Fatalf("no %s in %s", name, line)
	}
	rest := line[i+len(name)+3:]
	v, err := strconv.ParseFloat(rest[:strings.IndexByte(rest, '"')], 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// WithStyle keeps a switch an earlier option turned on, so options combine in any order (agni issue
// 765). Before, WithStyle after WithPinDots replaced the whole Style and the dots went away.
func TestWithStyleKeepsEarlierSwitches(t *testing.T) {
	s := resolveStyle([]Option{WithPinDots(), WithPickTargets(), WithStyle(DarkStyle)})
	if !s.PinDots || !s.PickTargets {
		t.Errorf("after WithStyle, PinDots=%v PickTargets=%v, want both kept", s.PinDots, s.PickTargets)
	}
	if s.Page != DarkStyle.Page {
		t.Errorf("WithStyle did not apply its palette: page %q, want %q", s.Page, DarkStyle.Page)
	}
}

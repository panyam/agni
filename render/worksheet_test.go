package render

import (
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

// TestWorksheetSuppressed: a sheet that carries its own title block (xschem/gEDA set
// SuppressWorksheet) draws no synthetic worksheet furniture, while an unsuppressed sheet does
// (WS7-036). The title-block field text is the tell; the schematic content renders either way.
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

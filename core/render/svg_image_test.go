package render

import (
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// A sheet-level geom.Image renders as an SVG <image> with an inline base64 data URI, so embedded
// bitmaps (KiCad/gEDA logos) draw rather than being dropped.
func TestSheetSVGImage(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Size: &geom.BBox{Min: &geom.Point{X: 0, Y: 0}, Max: &geom.Point{X: 1000, Y: 1000}},
			Images: []*geom.Image{{
				Bbox: &geom.BBox{Min: &geom.Point{X: 100, Y: 100}, Max: &geom.Point{X: 400, Y: 300}},
				Mime: "image/png",
				Data: []byte{0x89, 0x50, 0x4e, 0x47}, // arbitrary non-empty bytes
				Asset: &geom.Asset{Kind: geom.Asset_KIND_IMAGE},
			}},
		}},
	}
	out := SheetSVG(g, g.Sheets[0])
	if !strings.Contains(out, "<image") {
		t.Error("SVG has no <image> element for the sheet image")
	}
	if !strings.Contains(out, "data:image/png;base64,iVBORw==") {
		t.Errorf("SVG missing the inline data URI; got:\n%s", out)
	}
}

// An image with no bytes is skipped (nothing to draw), not emitted as a broken element.
func TestSheetSVGImageEmptySkipped(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{
			Size:   &geom.BBox{Min: &geom.Point{X: 0, Y: 0}, Max: &geom.Point{X: 1000, Y: 1000}},
			Images: []*geom.Image{{Bbox: &geom.BBox{Min: &geom.Point{X: 100, Y: 100}, Max: &geom.Point{X: 400, Y: 300}}}},
		}},
	}
	if strings.Contains(SheetSVG(g, g.Sheets[0]), "<image") {
		t.Error("empty image should not render an <image> element")
	}
}

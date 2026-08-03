package kicad

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// A KiCad (image ...) node decodes into a geom.Image marked as a loaded image asset, with a bbox
// sized from the PNG's pixel dimensions and the (scale). This is the extraction half of KiCad
// embedded-bitmap rendering (logos etc.), previously dropped by the geometry reader.
func TestKicadImageExtraction(t *testing.T) {
	raw := tinyPNG(t, 8, 8)
	b64 := base64.StdEncoding.EncodeToString(raw)
	sch := "(kicad_sch (version 20230121)(image (at 100 80 0)(scale 2)(uuid \"img-1\")(data \"" + b64 + "\")))"

	g, err := ReadSchematicGeometry(bytes.NewReader([]byte(sch)), "logo.kicad_sch")
	if err != nil {
		t.Fatalf("ReadSchematicGeometry: %v", err)
	}
	if len(g.Sheets) != 1 || len(g.Sheets[0].Images) != 1 {
		t.Fatalf("images = %v, want 1", g.Sheets)
	}
	im := g.Sheets[0].Images[0]
	if im.Mime != "image/png" {
		t.Errorf("mime = %q, want image/png", im.Mime)
	}
	if !bytes.Equal(im.Data, raw) {
		t.Errorf("image data not round-tripped (%d vs %d bytes)", len(im.Data), len(raw))
	}
	if im.Asset == nil || im.Asset.Kind != geom.Asset_KIND_IMAGE {
		t.Errorf("asset = %v, want KIND_IMAGE", im.Asset)
	}
	// 8 px * scale 2 * (25.4mm/300px) = 1.354 mm across; centre at x=100mm.
	nmPerPx := 25_400_000.0 / 300.0 * 2.0
	wantHalf := int64(float64(8) * nmPerPx / 2)
	if got := im.Bbox.Max.X - mmToNm("100"); got != wantHalf {
		t.Errorf("image half-width = %d nm, want %d", got, wantHalf)
	}
}

func tinyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{200, 20, 20, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return b.Bytes()
}

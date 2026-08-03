package geda

import (
	"encoding/base64"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// readPicture parses a gEDA picture object into a geom.Image and returns the index past it:
//
//	G x y width height angle mirror embedded
//	<filename>
//	<base64 data lines...>        (only when embedded == 1)
//
// The schematic stores no data length, so the embedded bytes are read as the run of pure-base64
// lines (no spaces) that follows the filename, stopping at the next object line (which has
// spaces). An external (non-embedded) picture has no bytes to load here, so it yields a
// placeholder Image with just its bbox. The mime is guessed from the filename extension.
func readPicture(lines []string, start int, pt func(x, y float64) *geom.Point) (*geom.Image, int) {
	f := strings.Fields(lines[start])
	x, _ := atof(field(f, 1))
	y, _ := atof(field(f, 2))
	w, _ := atof(field(f, 3))
	h, _ := atof(field(f, 4))
	angle := atoiInt(field(f, 5))
	mirror := atoiInt(field(f, 6))
	embedded := atoiInt(field(f, 7)) == 1

	i := start + 1
	filename := ""
	if i < len(lines) {
		filename = strings.TrimSpace(lines[i])
		i++
	}

	img := &geom.Image{
		Bbox:        &geom.BBox{Min: pt(x, y), Max: pt(x+w, y+h)},
		Mime:        mimeFromName(filename),
		RotationDeg: int32(angle % 360),
		Mirror:      mirror == 1,
		Asset:       &geom.Asset{Kind: geom.Asset_KIND_IMAGE, Id: filename, Prov: &geom.Provenance{SourceId: filename}},
	}
	if !embedded {
		return img, i
	}

	var b strings.Builder
	for i < len(lines) && isBase64Line(lines[i]) {
		b.WriteString(strings.TrimSpace(lines[i]))
		i++
	}
	if data, err := base64.StdEncoding.DecodeString(b.String()); err == nil {
		img.Data = data
	}
	return img, i
}

// isBase64Line reports whether a line is pure base64 (no spaces), i.e. embedded picture data
// rather than the next object (object lines carry space-separated fields).
func isBase64Line(s string) bool {
	s = strings.TrimRight(s, "\r")
	if s == "" || strings.ContainsAny(s, " \t") {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' || c == '=') {
			return false
		}
	}
	return true
}

func mimeFromName(name string) string {
	switch {
	case strings.HasSuffix(strings.ToLower(name), ".png"):
		return "image/png"
	case strings.HasSuffix(strings.ToLower(name), ".jpg"), strings.HasSuffix(strings.ToLower(name), ".jpeg"):
		return "image/jpeg"
	default:
		return "image/png"
	}
}

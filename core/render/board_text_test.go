package render

import (
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// Back-side text draws mirrored about its anchor, as KiCad draws it, and front text draws as it was
// (agni issue 1327, #142's BoardText.mirror).
func TestBoardTextTransform(t *testing.T) {
	for _, c := range []struct {
		name   string
		rot    float64
		mirror bool
		want   string
	}{
		{"front, upright", 0, false, ""},
		{"front, turned", -90, false, "rotate(-90.0 10.0 20.0)"},
		{"back, upright", 0, true, "translate(10.0 20.0) scale(-1 1) translate(-10.0 -20.0)"},
		{"back, turned", -90, true, "rotate(-90.0 10.0 20.0) translate(10.0 20.0) scale(-1 1) translate(-10.0 -20.0)"},
	} {
		if got := boardTextTransform(c.rot, c.mirror, 10, 20); got != c.want {
			t.Errorf("%s: transform %q, want %q", c.name, got, c.want)
		}
	}
}

func TestBoardSVGMirrorsBackText(t *testing.T) {
	b := &geom.BoardGeometry{Texts: []*geom.BoardText{
		{Text: "TOP", At: &geom.Point{X: 0, Y: 0}, Layer: "F.SilkS"},
		{Text: "BOTTOM", At: &geom.Point{X: 10_000_000, Y: 0}, Layer: "B.SilkS", Mirror: true},
	}}
	doc := BoardSVG(b)
	line := func(text string) string {
		for _, l := range strings.Split(doc, "\n") {
			if strings.Contains(l, ">"+text+"</text>") {
				return l
			}
		}
		t.Fatalf("no %s text in the board", text)
		return ""
	}
	if strings.Contains(line("TOP"), "scale(-1 1)") {
		t.Error("front text must not be mirrored")
	}
	if !strings.Contains(line("BOTTOM"), "scale(-1 1)") {
		t.Error("back text must be mirrored")
	}
}

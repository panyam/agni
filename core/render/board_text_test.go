package render

import (
	"fmt"
	"math"
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

// The packed board marks back-side text mirrored, so the WebGL overlay flips it as BoardSVG does
// (agni issue 1368).
func TestPackBoardMirrorsBackText(t *testing.T) {
	b := &geom.BoardGeometry{Texts: []*geom.BoardText{
		{Text: "TOP", At: &geom.Point{X: 0, Y: 0}, Layer: "F.SilkS"},
		{Text: "BOTTOM", At: &geom.Point{X: 10_000_000, Y: 0}, Layer: "B.SilkS", Mirror: true},
	}}
	got := map[string]bool{}
	for _, l := range PackBoard(b).GetLabels() {
		got[l.GetText()] = l.GetMirror()
	}
	if len(got) != 2 || got["TOP"] || !got["BOTTOM"] {
		t.Errorf("packed labels mirror %v, want BOTTOM alone", got)
	}
}

// A board text draws its glyphs at its height, as KiCad draws them, in its layer's colour (agni issue
// 1366). Both renderers size and colour it the same way.
func TestBoardTextSizeAndColour(t *testing.T) {
	b := &geom.BoardGeometry{
		Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 100_000_000, Y: 0}, {X: 100_000_000, Y: 100_000_000}}}}},
		Texts: []*geom.BoardText{
			{Text: "U1", At: &geom.Point{X: 10_000_000, Y: 10_000_000}, Height: 1_000_000, Layer: "F.SilkS"},
			{Text: "10K", At: &geom.Point{X: 30_000_000, Y: 10_000_000}, Height: 1_000_000, Layer: "B.Fab"},
			{Text: "NOTE", At: &geom.Point{X: 50_000_000, Y: 10_000_000}, Height: 1_000_000, Layer: "Cmts.User"},
		},
		Graphics: []*geom.BoardGraphic{
			{Layer: "F.Fab", Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{{X: 0, Y: 0}, {X: 1_000_000, Y: 0}}}},
		},
	}
	scale := frameBoard(b).scale
	// Arial's capital is 0.716 of its em, so a 1 mm glyph needs a 1/0.716 mm em. The test writes the
	// number out rather than reading capHeightEm, which would agree with itself whatever it held.
	wantPx := 1_000_000 / 0.716 * scale
	doc := BoardSVG(b)
	for text, colour := range map[string]string{"U1": DefaultStyle.Silk, "10K": DefaultStyle.Fab, "NOTE": DefaultStyle.Field} {
		var line string
		for _, l := range strings.Split(doc, "\n") {
			if strings.Contains(l, ">"+text+"</text>") {
				line = l
			}
		}
		if line == "" {
			t.Fatalf("no %s text in the board", text)
		}
		if !strings.Contains(line, `fill="`+colour+`"`) {
			t.Errorf("%s draws as %s, want fill %s", text, line, colour)
		}
		if want := fmt.Sprintf(`font-size="%.1f"`, wantPx); !strings.Contains(line, want) {
			t.Errorf("%s draws as %s, want %s, the em that draws a 1 mm glyph", text, line, want)
		}
	}
	if !strings.Contains(doc, `stroke="`+DefaultStyle.Fab+`"`) {
		t.Error("a fab outline must draw in the fab colour")
	}

	p := PackBoard(b)
	var labels []string
	for _, l := range p.GetLabels() {
		labels = append(labels, fmt.Sprintf("%s %d %s", l.GetText(), l.GetHeight(), l.GetColor()))
	}
	em := int64(math.Round(1_000_000 / 0.716))
	for _, want := range []string{fmt.Sprintf("U1 %d %s", em, DefaultStyle.Silk), fmt.Sprintf("10K %d %s", em, DefaultStyle.Fab)} {
		found := false
		for _, l := range labels {
			found = found || l == want
		}
		if !found {
			t.Errorf("packed labels %v, want %q", labels, want)
		}
	}
	if got := p.GetGroupColors()[boardRow(t, BoardLayers(b), "F.Fab").GetGroups()[0]]; got != DefaultStyle.Fab {
		t.Errorf("the packed fab group draws in %q, want %q", got, DefaultStyle.Fab)
	}
}

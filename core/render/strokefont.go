package render

import (
	"math"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// KiCad draws board text in its own stroke font, whose glyphs sit on a grid 21 units to the glyph
// height. strokeGlyphs holds each glyph's advance, its left bearing and its ink width in those
// units, measured from kicad-cli 10.0.6's own drawing. Each character was drawn once and ten times
// with a pen of 0.001 mm at a 21 mm height, so the advance is the difference over nine and the
// bearings cancel. A rune missing here takes strokeGlyphDefault.
var strokeGlyphs = map[rune]strokeGlyph{
	'!':  {10, 4, 2},
	'"':  {16, 4, 8},
	'#':  {21, 2, 17},
	'$':  {20, 4, 12},
	'%':  {24, 4, 16},
	'&':  {26, 5, 17},
	'\'': {10, 4, 2},
	'(':  {14, 5, 6},
	')':  {14, 3, 6},
	'*':  {16, 3, 10},
	'+':  {26, 5, 16},
	',':  {10, 4, 2},
	'-':  {26, 5, 16},
	'.':  {10, 4, 2},
	'/':  {22, 2, 18},
	'0':  {20, 4, 12},
	'1':  {20, 4, 12},
	'2':  {20, 3, 13},
	'3':  {20, 3, 13},
	'4':  {20, 4, 13},
	'5':  {20, 4, 12},
	'6':  {20, 4, 12},
	'7':  {20, 3, 14},
	'8':  {20, 4, 12},
	'9':  {20, 4, 12},
	':':  {10, 4, 2},
	';':  {10, 4, 2},
	'<':  {26, 5, 16},
	'=':  {26, 5, 16},
	'>':  {26, 5, 16},
	'?':  {18, 4, 10},
	'@':  {27, 4, 19},
	'A':  {18, 2, 14},
	'B':  {21, 5, 12},
	'C':  {21, 4, 13},
	'D':  {21, 5, 12},
	'E':  {19, 5, 10},
	'F':  {18, 5, 10},
	'G':  {21, 4, 13},
	'H':  {22, 5, 12},
	'I':  {10, 5, 0},
	'J':  {16, 3, 8},
	'K':  {21, 5, 12},
	'L':  {17, 5, 10},
	'M':  {24, 5, 14},
	'N':  {22, 5, 12},
	'O':  {22, 4, 14},
	'P':  {21, 5, 12},
	'Q':  {22, 4, 15},
	'R':  {21, 5, 12},
	'S':  {20, 4, 12},
	'T':  {16, 2, 12},
	'U':  {22, 5, 12},
	'V':  {18, 2, 14},
	'W':  {24, 3, 18},
	'X':  {20, 3, 14},
	'Y':  {18, 2, 14},
	'Z':  {20, 3, 14},
	'[':  {14, 6, 5},
	'\\': {14, -2, 18},
	']':  {14, 3, 5},
	'^':  {12, 2, 8},
	'_':  {16, 0, 16},
	'`':  {8, 2, 3},
	'a':  {19, 4, 10},
	'b':  {19, 5, 10},
	'c':  {18, 4, 10},
	'd':  {19, 4, 10},
	'e':  {18, 4, 10},
	'f':  {12, 2, 8},
	'g':  {19, 4, 10},
	'h':  {19, 5, 9},
	'i':  {10, 4, 2},
	'j':  {10, 1, 5},
	'k':  {17, 5, 8},
	'l':  {11, 5, 3},
	'm':  {28, 5, 18},
	'n':  {19, 5, 9},
	'o':  {19, 4, 11},
	'p':  {19, 5, 10},
	'q':  {19, 4, 10},
	'r':  {13, 5, 6},
	's':  {17, 4, 9},
	't':  {12, 2, 8},
	'u':  {19, 5, 9},
	'v':  {16, 3, 10},
	'w':  {22, 3, 16},
	'x':  {17, 3, 11},
	'y':  {16, 3, 10},
	'z':  {17, 3, 11},
	'{':  {14, 4, 7},
	'|':  {20, 10, 0},
	'}':  {14, 3, 7},
	'~':  {15, 2, 10},
	'±':  {26, 5, 16},
	'∅':  {29, 4, 21},
	'Ω':  {24, 4, 16},
	'ø':  {19, 3, 13},
	'µ':  {22, 5, 13},
	'°':  {16, 5, 6},
}

// strokeGlyph is one stroke-font glyph's advance, left bearing and ink width, in 21sts of the glyph
// height.
type strokeGlyph struct{ adv, left, ink int }

const (
	strokeGrid  = 21.0 // units to the glyph height
	strokeSpace = 16   // a space's advance
	// strokePenPad is how far KiCad pads a run's box on each side, as a fraction of the pen width.
	// kicad-cli's box grows by 1.316 pen widths whatever the string.
	strokePenPad = 0.658
	// strokeDefaultPen is KiCad's usual pen, as a fraction of the glyph height, for a text whose
	// source gives no thickness.
	strokeDefaultPen = 0.15
)

// strokeGlyphDefault is the glyph a rune outside the table takes, near the table's mean.
var strokeGlyphDefault = strokeGlyph{adv: 18, left: 4, ink: 10}

func strokeGlyphOf(r rune) strokeGlyph {
	if r == ' ' {
		return strokeGlyph{adv: strokeSpace}
	}
	if g, ok := strokeGlyphs[r]; ok {
		return g
	}
	return strokeGlyphDefault
}

// strokeInk is where a line of KiCad stroke text puts its ink, along the text's own direction from
// its anchor, and how long the ink runs, both in source units. A left-justified run's box starts at
// the anchor, a centred one is centred on it, and a right-justified one ends at it. The box is the
// glyphs' advances plus the pen padding on each side, and the ink starts at the first glyph's left
// bearing inside the padding. ok is false for a line with no ink.
func strokeInk(text, justify string, height, pen float64) (start, length float64, ok bool) {
	runes := []rune(strings.TrimRight(text, " "))
	lead := len(runes)
	for i, r := range runes {
		if r != ' ' {
			lead = i
			break
		}
	}
	if lead == len(runes) {
		return 0, 0, false
	}
	u := height / strokeGrid
	pad := strokePenPad * pen
	adv := 0
	for _, r := range runes[:len(runes)-1] {
		adv += strokeGlyphOf(r).adv
	}
	first, last := strokeGlyphOf(runes[lead]), strokeGlyphOf(runes[len(runes)-1])
	leadAdv := 0
	for _, r := range runes[:lead] {
		leadAdv += strokeGlyphOf(r).adv
	}
	box := float64(adv+last.adv)*u + 2*pad
	inkStart := pad + float64(leadAdv+first.left)*u
	inkEnd := pad + float64(adv+last.left+last.ink)*u
	switch {
	case strings.Contains(justify, "right"):
		inkStart, inkEnd = inkStart-box, inkEnd-box
	case strings.Contains(justify, "left"):
	default:
		inkStart, inkEnd = inkStart-box/2, inkEnd-box/2
	}
	return inkStart, inkEnd - inkStart, true
}

// boardTextRun is where a board text draws and how wide: its anchor, the justify the renderer
// applies, and an exact run length (0 for the font's own). A text in KiCad's stroke font is moved to
// the start of its ink along its own direction, left-justified, at the ink's length, so the
// renderer's font covers what KiCad's covers. The move goes backwards along the text for mirrored
// text, which the renderers mirror about the anchor they are given.
func boardTextRun(t *geom.BoardText) (*geom.Point, string, int64) {
	if t.GetFont() != "kicad_stroke" || t.GetAt() == nil {
		return t.GetAt(), t.GetJustify(), 0
	}
	h := float64(boardTextHeight(t))
	pen := float64(t.GetThickness())
	if pen <= 0 {
		pen = strokeDefaultPen * h
	}
	start, length, ok := strokeInk(t.GetText(), t.GetJustify(), h, pen)
	if !ok {
		return t.GetAt(), t.GetJustify(), 0
	}
	r := t.GetRotationDeg() * math.Pi / 180
	if t.GetMirror() {
		start = -start
	}
	at := &geom.Point{X: t.GetAt().GetX() + int64(math.Round(start*math.Cos(r))), Y: t.GetAt().GetY() + int64(math.Round(start*math.Sin(r)))}
	return at, "left", int64(math.Round(length))
}

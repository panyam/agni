package render

import (
	"math"
	"unicode/utf8"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// flagOutline is the closed outline drawn around a flagged label, in sheet coordinates, or nil
// for a label without one. h is the label's text height after the sheet default applies.
//
// The shape is KiCad's global label (SCH_GLOBALLABEL::CreateGraphicShape), measured against
// kicad-cli on the gateway sheet. Its half-height is half the text height plus a margin of
// 0.375 heights plus the pen, and an arrow end adds a point half that far out. Its length comes
// from this renderer's own text width, glyphAdvanceEm per rune, rather than from KiCad's stroke
// font, so the box fits the text a reader sees.
func flagOutline(l *geom.Label, h int64) []*geom.Point {
	if l.GetFlag() == geom.Label_FLAG_NONE || l.Anchor == nil || l.Origin == nil {
		return nil
	}
	a := l.Anchor
	// The flag runs from its connect point toward the text.
	tx, ty := float64(l.Origin.X-a.X), float64(l.Origin.Y-a.Y)
	n := math.Hypot(tx, ty)
	if n == 0 {
		return nil
	}
	tx, ty = tx/n, ty/n
	ux, uy := -ty, tx

	fh := float64(h)
	margin := 0.375 * fh
	pen := 0.12 * fh
	half := fh/2 + margin
	y := half + pen
	x := glyphAdvanceEm*fh*float64(utf8.RuneCountInString(l.Text)) + 2*margin + pen

	// KiCad's points, with the flag running toward -x from the connect point at 0.
	pts := [][2]float64{{0, 0}, {0, -y}, {-x, -y}, {-x, 0}, {-x, y}, {0, y}}
	off := 0.0
	switch l.Flag {
	case geom.Label_FLAG_INPUT:
		off = -half
		pts[0][0] += half
	case geom.Label_FLAG_OUTPUT:
		pts[3][0] -= half
	case geom.Label_FLAG_BIDIRECTIONAL, geom.Label_FLAG_TRI_STATE:
		off = -half
		pts[0][0] += half
		pts[3][0] -= half
	}
	out := make([]*geom.Point, 0, len(pts)+1)
	for _, p := range pts {
		along, across := -(p[0] + off), p[1]
		out = append(out, &geom.Point{
			X: a.X + int64(math.Round(tx*along+ux*across)),
			Y: a.Y + int64(math.Round(ty*along+uy*across)),
		})
	}
	return append(out, out[0])
}

// labelHeight is a label's text height, or the sheet default when the label states none.
func labelHeight(l *geom.Label, def int64) int64 {
	if l.Height > 0 {
		return l.Height
	}
	return def
}

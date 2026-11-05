package render

import (
	"math"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
)

// pinTextAt places one run of a pin's text (its name or its number) on a placed symbol. It
// returns the world anchor, the rotation drawText takes (clockwise on screen, as a field's is)
// and the justify, already turned to read left to right or bottom to top.
//
// Both backends call it, so the SVG and the WebGL overlay put pin text in one place. It works
// from directions rather than adding angles, because a mirror reverses text and a placement's
// rotation is counter-clockwise where drawText's is clockwise. Adding the two turned a rotated
// part's text half a turn, which nothing showed while pin text was centered (agni 1344).
//
// A run beside its pin line stays on the side of the line that the symbol drew it on, as seen
// by a reader. So a pin number above its line is still above it on a part turned 180 or
// mirrored, as KiCad draws it. A run on the line's own extension, such as a name inside the
// body, has no side and only turns.
func pinTextAt(t *geom.Transform, pin *geom.PinPoint, origin *geom.Point, justify string) (x, y int64, rotCW int32, just string) {
	at := geomath.ApplyTransform(t, origin)
	loc := pin.GetLoc()
	if loc == nil {
		loc = origin
	}
	// The run's direction and its "up" in the symbol's frame, and the same two placed.
	th := float64(pin.GetTextRotationDeg()) * math.Pi / 180
	dx, dy := math.Cos(th), math.Sin(th)
	ux, uy := -dy, dx
	const k = 1e6
	l := geomath.ApplyTransform(t, loc)
	placed := func(x, y float64) (float64, float64) {
		p := geomath.ApplyTransform(t, &geom.Point{X: loc.X + int64(math.Round(x*k)), Y: loc.Y + int64(math.Round(y*k))})
		return float64(p.X-l.X) / k, float64(p.Y-l.Y) / k
	}
	wdx, wdy := placed(dx, dy)
	wux, wuy := placed(ux, uy)

	h, v := splitJustify(justify)
	// A mirrored placement reverses the run's up, and the text a reader sees has its up a
	// quarter turn counter-clockwise from its direction, so the vertical justify flips.
	if wdx*wuy-wdy*wux < 0 {
		v = flipV(v)
	}
	// Turn the run to read left to right, or bottom to top when it is vertical.
	phi := math.Atan2(wdy, wdx) * 180 / math.Pi
	if phi <= -90+1e-6 || phi > 90+1e-6 {
		phi += 180
		h, v = flipH(h), flipV(v)
	}
	if phi > 180 {
		phi -= 360
	}
	// Keep the run on the side of its line that the symbol drew it on.
	side := func(px, py, nx, ny float64) float64 { return px*nx + py*ny }
	local := side(float64(origin.X-loc.X), float64(origin.Y-loc.Y), ux, uy)
	r := phi * math.Pi / 180
	fx, fy := math.Cos(r), math.Sin(r)
	fux, fuy := -fy, fx
	ax, ay := float64(at.X-l.X), float64(at.Y-l.Y)
	if now := side(ax, ay, fux, fuy); math.Abs(local) > 0.5 && math.Signbit(now) != math.Signbit(local) {
		// Reflect across the line through the placed pin, along the run.
		along := ax*fx + ay*fy
		ax, ay = 2*along*fx-ax, 2*along*fy-ay
		v = flipV(v)
	}
	return l.X + int64(math.Round(ax)), l.Y + int64(math.Round(ay)), int32(math.Round(-phi)), joinJustify(h, v)
}

// splitJustify splits a canonical "<h> <v>" justify into its two words, either one empty when
// the justify leaves that axis centered.
func splitJustify(j string) (h, v string) {
	for _, w := range strings.Fields(j) {
		switch w {
		case "left", "right", "center":
			h = w
		case "top", "bottom", "middle":
			v = w
		}
	}
	return h, v
}

func joinJustify(h, v string) string { return strings.TrimSpace(h + " " + v) }

func flipH(h string) string {
	switch h {
	case "left":
		return "right"
	case "right":
		return "left"
	}
	return h
}

func flipV(v string) string {
	switch v {
	case "top":
		return "bottom"
	case "bottom":
		return "top"
	}
	return v
}

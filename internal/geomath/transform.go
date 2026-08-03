// Package geomath is the shared geometry math for the geom sidecar: mapping symbol-local
// coordinates into sheet (world) coordinates under a placement Transform. World space is
// Y-up, int64 source units; the application order is scale, then mirror, then rotate
// (CCW), then translate, matching the EDIF orientation semantics in
// docs/edif-schematic-primer.md §7 and the format-neutral Transform contract in docs/16.
//
// Both producers and consumers of geometry depend on it: readers that compute pin
// world positions for connectivity (kicad) and the renderers that draw placements
// (render). Sharing one implementation is what guarantees pins land where symbols are
// drawn — readers must never reach into the presentation tier for it (CONSTRAINTS C15).
package geomath

import (
	"math"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// ApplyTransform maps a symbol-local point into sheet (world) coordinates (see the
// package doc for the application order and coordinate conventions).
//
// A pin's absolute connect location is ApplyTransform(placement.Transform, pin.Loc); this
// is what makes pins land on wire endpoints (primer §7).
func ApplyTransform(t *geom.Transform, p *geom.Point) *geom.Point {
	if p == nil {
		return nil
	}
	x, y := float64(p.X), float64(p.Y)
	if t != nil {
		// Scale (0 means unset, treat as 1.0). Rare in practice.
		sx, sy := t.ScaleX, t.ScaleY
		if sx == 0 {
			sx = 1
		}
		if sy == 0 {
			sy = 1
		}
		x, y = x*sx, y*sy

		// Mirror, applied before rotation. mirror_x = mirror across the X axis (flip Y);
		// mirror_y = mirror across the Y axis (flip X).
		if t.MirrorX {
			y = -y
		}
		if t.MirrorY {
			x = -x
		}

		// Rotate counter-clockwise. EDIF uses only 0/90/180/270.
		switch ((t.RotationDeg % 360) + 360) % 360 {
		case 90:
			x, y = -y, x
		case 180:
			x, y = -x, -y
		case 270:
			x, y = y, -x
		}

		// Translate by placement origin.
		if t.Origin != nil {
			x += float64(t.Origin.X)
			y += float64(t.Origin.Y)
		}
	}
	return &geom.Point{X: roundInt64(x), Y: roundInt64(y)}
}

// PlacePin returns the world-space connect location of a pin under a placement transform.
func PlacePin(t *geom.Transform, pin *geom.PinPoint) *geom.Point {
	if pin == nil {
		return nil
	}
	return ApplyTransform(t, pin.Loc)
}

// PlaceShape returns a copy of a shape with every point mapped into world coordinates.
// Rotation and mirror leave a circle's radius unchanged; a scale factor (rare) scales it.
func PlaceShape(t *geom.Transform, s *geom.Shape) *geom.Shape {
	if s == nil {
		return nil
	}
	out := &geom.Shape{Kind: s.Kind, Radius: s.Radius, FigureGroup: s.FigureGroup, Fill: s.Fill, FillColor: s.FillColor}
	for _, p := range s.Points {
		out.Points = append(out.Points, ApplyTransform(t, p))
	}
	if t != nil && s.Kind == geom.Shape_KIND_CIRCLE {
		sx := t.ScaleX
		if sx == 0 {
			sx = 1
		}
		if sx != 1 {
			out.Radius = roundInt64(float64(s.Radius) * math.Abs(sx))
		}
	}
	return out
}

func roundInt64(v float64) int64 {
	return int64(math.Round(v))
}

// ComposePlacement maps a footprint-local offset into board (world) coordinates under a
// board placement: world = at + M(R(rotationDeg) * offset), where R rotates CCW and M
// mirrors X iff the placement is on the back side (mirror is applied AFTER the rotation, so
// a back part is its front footprint rotated then reflected). rotationDeg is already in the
// canonical Y-up frame (a Y-down source negates it on import), so no source remapping
// happens here — this is the pure composer every board producer and the board renderer
// share, and sharing it is what pins a footprint's pads and its silkscreen text to the same
// spot. offset is in the reader's Y-flipped source-unit frame (pcbPoint). A nil at yields
// the origin; a nil offset yields at unchanged. Truncates to int64 (not rounds) to match the
// board renderer's long-standing pad placement byte-for-byte.
func ComposePlacement(at *geom.Point, rotationDeg float64, mirror bool, offset *geom.Point) *geom.Point {
	if at == nil {
		return &geom.Point{}
	}
	if offset == nil {
		return &geom.Point{X: at.X, Y: at.Y}
	}
	rad := rotationDeg * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	px, py := float64(offset.X), float64(offset.Y)
	rx, ry := cos*px-sin*py, sin*px+cos*py
	if mirror {
		rx = -rx
	}
	return &geom.Point{X: at.X + int64(rx), Y: at.Y + int64(ry)}
}

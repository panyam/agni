// Package geomath is the shared geometry math for the geom sidecar, mapping symbol-local
// coordinates into sheet (world) coordinates under a placement Transform. World space is Y-up in
// int64 source units. The application order is scale, then mirror, then rotate (CCW), then
// translate, matching section 7 of docsite/content/reference/edif-schematic-primer.md and the
// Transform message in docsite/content/architecture/geometry-and-rendering.md#proto-contract-sketch.
//
// Two kinds of caller use it. Readers compute pin world positions for connectivity (kicad), and
// core/render draws placements. Both use the same math, so pins sit where symbols are drawn.
// Readers must not import the presentation tier for it (CONSTRAINTS C17).
package geomath

import (
	"math"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// ApplyTransform maps a symbol-local point into sheet (world) coordinates, in the order the
// package doc gives. A pin's absolute connect location is ApplyTransform(placement.Transform,
// pin.Loc). A nil point returns nil and a nil transform is the identity.
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

		// Mirror before rotation. mirror_x flips Y (across the X axis), mirror_y flips X.
		if t.MirrorX {
			y = -y
		}
		if t.MirrorY {
			x = -x
		}

		// Rotate counter-clockwise. A quarter turn is exact. Any other angle turns by cos and sin,
		// since a format may write one and dropping it to 0 drew the part unturned (agni issue 1344).
		switch a := ((t.RotationDeg % 360) + 360) % 360; a {
		case 0:
		case 90:
			x, y = -y, x
		case 180:
			x, y = -x, -y
		case 270:
			x, y = y, -x
		default:
			rad := float64(a) * math.Pi / 180
			cos, sin := math.Cos(rad), math.Sin(rad)
			x, y = cos*x-sin*y, sin*x+cos*y
		}

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

// ComposePlacement maps a footprint-local offset into board (world) coordinates as
// world = at + M(R(rotationDeg) * offset). R rotates CCW. M mirrors X iff the part is on the
// back side, and it applies AFTER the rotation, so a back part is its front footprint rotated then
// reflected. rotationDeg is already in the canonical Y-up frame (a clockwise source such as ODB++
// negates it on import, while KiCad's angle carries over), and offset is in the reader's Y-flipped source-unit frame (pcbPoint). Every board
// producer and the board renderer share it, so a footprint's pads and silkscreen text are placed
// together. A nil at yields the origin and a nil offset yields at unchanged. It truncates to int64
// rather than rounding, to match the board renderer's pad placement byte-for-byte.
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

// NormDeg folds an angle in degrees into [0, 360).
func NormDeg(d float64) float64 {
	d = math.Mod(d, 360)
	if d < 0 {
		d += 360
	}
	return d
}

// PlaceLand puts a point of a pad's own frame on the board, the frame a padstack land's offset and
// outline are written in (agni 1407). The point turns with the pad's angle into the footprint's
// frame, and then the footprint is placed as the contract defines it:
// world = at + M(R(rotation) * local), with M mirroring X on a mirrored placement.
func PlaceLand(pl *geom.ComponentPlacement, pad *geom.Pad, px, py float64) (float64, float64) {
	s, c := math.Sincos(pad.GetRotationDeg() * math.Pi / 180)
	lx := float64(pad.GetAt().GetX()) + px*c - py*s
	ly := float64(pad.GetAt().GetY()) + px*s + py*c
	s, c = math.Sincos(pl.GetRotationDeg() * math.Pi / 180)
	x, y := lx*c-ly*s, lx*s+ly*c
	if pl.GetMirror() {
		x = -x
	}
	return float64(pl.GetAt().GetX()) + x, float64(pl.GetAt().GetY()) + y
}

// PadAngle is the angle a pad's shape is drawn at on the board, CCW in the Y-up frame. A pad's own
// angle is in its footprint's frame (agni 1407), so it adds to the placement's, and a mirrored
// placement reflects the sum: M R(a) = R(-a) M, and the shapes a pad names are symmetric in X.
func PadAngle(pl *geom.ComponentPlacement, pad *geom.Pad) float64 {
	a := pl.GetRotationDeg() + pad.GetRotationDeg()
	if pl.GetMirror() {
		a = -a
	}
	return NormDeg(a)
}

// PadLocalAngle is PadAngle's inverse: the footprint-frame angle of a pad drawn at boardDeg on the
// board. A reader whose source stores a pad's board angle (KiCad, ODB++) writes this.
func PadLocalAngle(pl *geom.ComponentPlacement, boardDeg float64) float64 {
	if pl.GetMirror() {
		boardDeg = -boardDeg
	}
	return NormDeg(boardDeg - pl.GetRotationDeg())
}

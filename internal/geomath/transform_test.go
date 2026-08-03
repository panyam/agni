package geomath

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestApplyTransform_Orientations pins the 8 EDIF orientation codes to hand-computed
// results (primer §7 matrices). The probe point (10, 5) is asymmetric so every code
// yields a distinct output; this is the ground-truth oracle for the transform math that
// the SVG dumper and the connectivity oracle both depend on.
func TestApplyTransform_Orientations(t *testing.T) {
	p := &geom.Point{X: 10, Y: 5}
	cases := []struct {
		name  string
		t     *geom.Transform
		wantX int64
		wantY int64
	}{
		{"R0", &geom.Transform{}, 10, 5},
		{"R90", &geom.Transform{RotationDeg: 90}, -5, 10},
		{"R180", &geom.Transform{RotationDeg: 180}, -10, -5},
		{"R270", &geom.Transform{RotationDeg: 270}, 5, -10},
		{"MX", &geom.Transform{MirrorX: true}, 10, -5},                      // flip Y
		{"MY", &geom.Transform{MirrorY: true}, -10, 5},                      // flip X
		{"MXR90", &geom.Transform{MirrorX: true, RotationDeg: 90}, 5, 10},   // MX then R90
		{"MYR90", &geom.Transform{MirrorY: true, RotationDeg: 90}, -5, -10}, // MY then R90
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ApplyTransform(c.t, p)
			if got.X != c.wantX || got.Y != c.wantY {
				t.Fatalf("%s: got (%d,%d) want (%d,%d)", c.name, got.X, got.Y, c.wantX, c.wantY)
			}
		})
	}
}

// TestApplyTransform_OriginAndScale checks translation and the rare scale factor compose
// after the orientation (scale, then mirror/rotate, then translate).
func TestApplyTransform_OriginAndScale(t *testing.T) {
	// R90 of (10,5) = (-5,10); origin (100,200) -> (95,210).
	got := ApplyTransform(&geom.Transform{RotationDeg: 90, Origin: &geom.Point{X: 100, Y: 200}}, &geom.Point{X: 10, Y: 5})
	if got.X != 95 || got.Y != 210 {
		t.Fatalf("origin compose: got (%d,%d) want (95,210)", got.X, got.Y)
	}
	// Scale x2 then translate: (10,5)*2 = (20,10), +origin(1,1) = (21,11).
	got = ApplyTransform(&geom.Transform{ScaleX: 2, ScaleY: 2, Origin: &geom.Point{X: 1, Y: 1}}, &geom.Point{X: 10, Y: 5})
	if got.X != 21 || got.Y != 11 {
		t.Fatalf("scale compose: got (%d,%d) want (21,11)", got.X, got.Y)
	}
}

// TestPlaceShape_RectStaysAxisAligned confirms a rect's corners transform pointwise; at
// 90 degrees width and height swap but the shape stays axis-aligned (EDIF only rotates by
// multiples of 90).
func TestPlaceShape_RectStaysAxisAligned(t *testing.T) {
	rect := &geom.Shape{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{X: 0, Y: 0}, {X: 40, Y: 20}}}
	out := PlaceShape(&geom.Transform{RotationDeg: 90}, rect)
	// (0,0)->(0,0); (40,20)->(-20,40).
	if out.Points[0].X != 0 || out.Points[0].Y != 0 {
		t.Fatalf("corner0: got (%d,%d) want (0,0)", out.Points[0].X, out.Points[0].Y)
	}
	if out.Points[1].X != -20 || out.Points[1].Y != 40 {
		t.Fatalf("corner1: got (%d,%d) want (-20,40)", out.Points[1].X, out.Points[1].Y)
	}
}

package geomath

import (
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// Bounds accumulates a bounding box over geom points. The zero value is an empty box:
// Valid reports whether any point was added, and BBox on an empty box is the origin. It
// replaces three per-package copies (xschem, geda, render), so producers and consumers
// compute extents with the same arithmetic.
type Bounds struct {
	set                    bool
	minX, minY, maxX, maxY int64
}

// Add widens the box to include p; a nil point is ignored.
func (b *Bounds) Add(p *geom.Point) {
	if p == nil {
		return
	}
	if !b.set {
		b.minX, b.minY, b.maxX, b.maxY = p.X, p.Y, p.X, p.Y
		b.set = true
		return
	}
	b.minX = min(b.minX, p.X)
	b.minY = min(b.minY, p.Y)
	b.maxX = max(b.maxX, p.X)
	b.maxY = max(b.maxY, p.Y)
}

// Valid reports whether at least one point was added.
func (b *Bounds) Valid() bool { return b.set }

// Min returns the lower corner (0,0 for an empty box).
func (b *Bounds) Min() (int64, int64) { return b.minX, b.minY }

// Max returns the upper corner (0,0 for an empty box).
func (b *Bounds) Max() (int64, int64) { return b.maxX, b.maxY }

// BBox returns the accumulated box as the proto form (the origin box when empty).
func (b *Bounds) BBox() *geom.BBox {
	return &geom.BBox{Min: &geom.Point{X: b.minX, Y: b.minY}, Max: &geom.Point{X: b.maxX, Y: b.maxY}}
}

// SymbolBBox is the extent of a symbol definition's artwork and pins, the bbox recorded on
// SymbolDef for renderer fit and auto-layout sizing.
func SymbolBBox(sd *geom.SymbolDef) *geom.BBox {
	var b Bounds
	for _, s := range sd.Shapes {
		for _, p := range s.Points {
			b.Add(p)
		}
	}
	for _, pn := range sd.Pins {
		b.Add(pn.Loc)
	}
	return b.BBox()
}

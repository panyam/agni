package graph

import (
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// gutter is the minimum gap between adjacent node bounding boxes. Sized so a full grid of synthetic
// glyphs (widest is 2*terminalX = 80) still fits one node per pitch: 80 + gutter <= pitch, so the
// glyph/box layout stays exactly on the base grid (compactBySize leaves it unchanged).
const gutter = pitch / 5

// compactBySize re-spaces a grid-aligned placement so each node gets a cell sized to its own
// symbol, rather than every node being spaced for the largest one (the old uniform scale, which
// left mixed-size layouts sparse). Nodes that share an X form a column and nodes that share a Y
// form a row (both strategies place on integer multiples of pitch); each column is widened to its
// widest node and each row heightened to its tallest, floored at pitch so an all-glyph design is
// unchanged. Nodes sit at their cell centres, and the whole layout is translated so the
// sorted-first ref anchors the origin (stable for diffing). refs must be the placement's ref-des
// in sorted order.
//
// It assumes grid-aligned positions (distinct X = columns, distinct Y = rows), which grid and
// layered satisfy; a future continuous-coordinate strategy would need its own overlap removal.
func compactBySize(pos map[string]*geom.Point, sizes map[string]nodeSize, refs []string) map[string]*geom.Point {
	if len(pos) == 0 {
		return pos
	}
	// Per-column width and per-row height: max node extent in that column/row, floored at pitch.
	colW := map[int64]int64{}
	rowH := map[int64]int64{}
	for ref, p := range pos {
		s := sizes[ref]
		if w := s.w + gutter; w > colW[p.X] {
			colW[p.X] = w
		}
		if h := s.h + gutter; h > rowH[p.Y] {
			rowH[p.Y] = h
		}
	}
	floor := func(m map[int64]int64) {
		for k, v := range m {
			if v < pitch {
				m[k] = pitch
			}
		}
	}
	floor(colW)
	floor(rowH)

	// Lay columns left-to-right (X ascending) and rows top-to-bottom (Y descending, since geom is
	// Y-up) edge to edge; a cell's centre is its running offset plus half its size.
	centerX := runningCenters(colW, false)
	centerY := runningCenters(rowH, true)

	out := make(map[string]*geom.Point, len(pos))
	for ref, p := range pos {
		out[ref] = &geom.Point{X: centerX[p.X], Y: centerY[p.Y]}
	}
	// Anchor the sorted-first ref at the origin so the layout stays stable for diffing.
	if len(refs) > 0 {
		if a := out[refs[0]]; a != nil {
			ax, ay := a.X, a.Y
			for _, p := range out {
				p.X -= ax
				p.Y -= ay
			}
		}
	}
	return out
}

// runningCenters maps each distinct coordinate to the centre of its cell, laying the cells edge to
// edge in coordinate order: ascending when descend is false (columns, left to right), descending
// when true (rows, top to bottom in Y-up).
func runningCenters(size map[int64]int64, descend bool) map[int64]int64 {
	keys := make([]int64, 0, len(size))
	for k := range size {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if descend {
			return keys[i] > keys[j]
		}
		return keys[i] < keys[j]
	})
	center := make(map[int64]int64, len(keys))
	var edge int64
	for _, k := range keys {
		w := size[k]
		if descend {
			center[k] = edge - w/2 // going down in Y-up: edges decrease
			edge -= w
		} else {
			center[k] = edge + w/2
			edge += w
		}
	}
	return center
}

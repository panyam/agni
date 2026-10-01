package graph

import (
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// gutter is the minimum gap between adjacent node bounding boxes. It is sized so the widest
// synthetic glyph (2*terminalX = 80) plus gutter still fits one pitch, which keeps an all-glyph
// layout exactly on the base grid.
const gutter = pitch / 5

// compactBySize re-spaces a grid-aligned placement so each node gets a cell sized to its own symbol
// rather than to the largest one. Nodes sharing an X form a column and nodes sharing a Y form a row.
// Each column is widened to its widest node and each row heightened to its tallest, floored at pitch
// so an all-glyph design is unchanged. Nodes sit at their cell centres, and the layout is translated
// so the sorted-first ref anchors the origin. refs must be the placement's ref-des in sorted order.
//
// It assumes grid-aligned positions. Every current strategy places on integer multiples of pitch
// (stress and force snap to it), and a continuous-coordinate strategy would need its own overlap
// removal.
func compactBySize(pos map[string]*geom.Point, sizes map[string]nodeSize, refs []string) map[string]*geom.Point {
	if len(pos) == 0 {
		return pos
	}
	// Per-column width and per-row height, as the max node extent floored at pitch.
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

	// Columns run left to right (X ascending) and rows top to bottom (Y descending, since geom is
	// Y-up).
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
// edge in coordinate order, ascending unless descend is set.
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
			center[k] = edge - w/2 // edges decrease going down in Y-up
			edge -= w
		} else {
			center[k] = edge + w/2
			edge += w
		}
	}
	return center
}

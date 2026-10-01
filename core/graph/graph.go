// Package graph builds a netlist-graph view of a design from the core IR alone, for formats that
// carry no schematic page (IPC-2581, a bare netlist, a board-only export). It is the fallback behind
// `agni render --layout=grid` and the other auto-layouts. See
// docsite/content/architecture/geometry-and-rendering.md#auto-layout-node-drawing.
//
// The output is an ordinary geom.SchematicGeometry, so it feeds the same render backends as a real
// schematic sheet (SheetSVG, PackSheet) and nothing downstream is graph-specific. Layout stays in the
// Go core rather than a backend, so every surface reuses it (CONSTRAINTS C1).
//
// A Strategy places nodes (see layout.go) and a shared assembler turns those positions into drawable
// geometry, so only placement differs between algorithms. This file holds the default grid placer and
// the helpers assembly shares.
package graph

import (
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// nodeCell is the plain box drawn for a component node that reaches no glyph. It is not a real
// part type, so it uses a reserved cell_ref that no reader emits.
const nodeCell = "__node__"

// Layout coordinates. Absolute values are arbitrary since SheetSVG normalizes to pixels, so only
// their ratios (box size relative to grid pitch) affect the look.
const (
	pitch    = 100 // grid spacing between node centers
	halfNode = 30  // half the node box side
)

// gridPlace positions components on a deterministic square grid, row-major over components
// sorted by ref_des (cols = ceil(sqrt(n))). It ignores edges and is the baseline the other layouts
// improve on. The ref_des order keeps a rendered graph stable across runs for diffing.
func gridPlace(d *ir.Design) Placement {
	comps := make([]*ir.Component, len(d.Components))
	copy(comps, d.Components)
	sort.Slice(comps, func(i, j int) bool { return comps[i].RefDes < comps[j].RefDes })

	cols := 1
	for cols*cols < len(comps) {
		cols++
	}
	pos := make(map[string]*geom.Point, len(comps))
	for i, c := range comps {
		// Row-major grid; Y grows downward (rows increase), which SheetSVG flips to Y-up.
		pos[c.RefDes] = &geom.Point{X: int64(i%cols) * pitch, Y: -int64(i/cols) * pitch}
	}
	return Placement{Positions: pos}
}

// netPinPoints returns the world-space attach point for each connection of a net, in first-seen
// order. That is the node origin plus the pin's symbol-local location, or the node centre when the
// symbol has no matching pin (see findPin). Points are deduped so two connections at the same spot
// draw one spoke, and connections to unplaced components (dangling refs) are skipped.
func netPinPoints(net *ir.Net, positions map[string]*geom.Point, syms map[string]*geom.SymbolDef) []*geom.Point {
	seen := make(map[[2]int64]bool)
	out := make([]*geom.Point, 0, len(net.Connections))
	for _, conn := range net.Connections {
		origin, ok := positions[conn.ComponentRef]
		if !ok {
			continue
		}
		pt := origin
		if pin := findPin(syms[conn.ComponentRef], conn.PinRef); pin.GetLoc() != nil {
			pt = &geom.Point{X: origin.X + pin.Loc.X, Y: origin.Y + pin.Loc.Y}
		}
		key := [2]int64{pt.X, pt.Y}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, pt)
	}
	return out
}

// findPin returns the symbol's pin whose designator matches pinRef, or nil when the symbol has no
// such pin (a generic box, or a source pin name the glyph does not carry). Callers then fall back to
// the node centre so the edge still draws.
func findPin(sym *geom.SymbolDef, pinRef string) *geom.PinPoint {
	for _, p := range sym.GetPins() {
		if p.GetPortRef() == pinRef {
			return p
		}
	}
	return nil
}

// nodeSymbol is the shared box a glyph-less component node is drawn as, centered on the origin so
// a placement's transform origin is the node center (where ref_des labels and net edges meet).
func nodeSymbol() *geom.SymbolDef {
	return &geom.SymbolDef{
		CellRef: nodeCell,
		Shapes: []*geom.Shape{{
			Kind: geom.Shape_KIND_RECT,
			Points: []*geom.Point{
				{X: -halfNode, Y: -halfNode},
				{X: halfNode, Y: halfNode},
			},
		}},
	}
}

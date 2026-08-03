package graph

import (
	"math"
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// SymbolSource supplies the drawable symbol for each component node in an auto-layout. It is the
// seam that lets the layout stay fixed while what gets drawn at a node varies: the Registry
// (classified synthetic glyphs, WS7-030) is one implementation, FaithfulSource (the design's own
// symbol artwork, WS7-031) is another. Symbol is called once per placed component; the returned
// SymbolDef's CellRef keys the placement to the shipped symbol library, so a source must return
// a stable CellRef per distinct symbol. c may be nil when a placed ref has no matching component.
type SymbolSource interface {
	Symbol(ref string, c *ir.Component, parts map[string]*ir.PartType) *geom.SymbolDef
}

// FaithfulSource draws each component with its real symbol from a design's geometry sidecar,
// keyed by ref_des, so an auto-layout can re-place the design's own artwork instead of synthetic
// glyphs (partial-faithful mode). Components with no faithful symbol fall back to a base source
// (normally the Registry, then the box), so a design that mixes provided and unknown parts still
// renders every node.
type FaithfulSource struct {
	byRef map[string]*geom.SymbolDef
	base  SymbolSource
}

// NewFaithfulSource indexes a geometry sidecar by ref_des (via each placement's cell_ref) so a
// component resolves to the symbol the design actually drew it with. base supplies the fallback
// for refs the sidecar does not cover; a nil base uses DefaultRegistry.
func NewFaithfulSource(faithful *geom.SchematicGeometry, base SymbolSource) *FaithfulSource {
	if base == nil {
		base = DefaultRegistry()
	}
	byCell := make(map[string]*geom.SymbolDef, len(faithful.GetSymbols()))
	for _, s := range faithful.GetSymbols() {
		byCell[s.GetCellRef()] = s
	}
	byRef := make(map[string]*geom.SymbolDef)
	for _, sh := range faithful.GetSheets() {
		for _, pl := range sh.GetPlacements() {
			if pl.GetRefDes() == "" {
				continue
			}
			if s := byCell[pl.GetCellRef()]; s != nil {
				byRef[pl.GetRefDes()] = s
			}
		}
	}
	return &FaithfulSource{byRef: byRef, base: base}
}

// Symbol returns the component's faithful symbol, or the base source's symbol when the sidecar
// has none for that ref_des.
func (f *FaithfulSource) Symbol(ref string, c *ir.Component, parts map[string]*ir.PartType) *geom.SymbolDef {
	if s := f.byRef[ref]; s != nil && !s.GetAsset().GetPlaceholder() {
		return s
	}
	// A missing ref, or only a placeholder box (the .sym did not resolve), falls back to the base
	// source's synthetic glyph rather than drawing a blank box.
	return f.base.Symbol(ref, c, parts)
}

// Covers reports how many placed ref_des the source has a real (non-placeholder) symbol for, so a
// caller can tell an empty/unresolved join from a real one.
func (f *FaithfulSource) Covers() int {
	n := 0
	for _, s := range f.byRef {
		if !s.GetAsset().GetPlaceholder() {
			n++
		}
	}
	return n
}

// nodeSize is a node's drawn extent (its symbol's width and height) in layout units.
type nodeSize struct{ w, h int64 }

// symbolSize is a symbol's width and height in its own units, from its bounding box when set, else
// from its shape and pin points. The zero size means the symbol has no geometry. It is what the
// size-aware layout (compactBySize) spaces nodes by, so a small part gets a small cell and only a
// large symbol expands its column/row.
func symbolSize(s *geom.SymbolDef) nodeSize {
	if b := s.GetBbox(); b.GetMin() != nil && b.GetMax() != nil {
		return nodeSize{b.Max.X - b.Min.X, b.Max.Y - b.Min.Y}
	}
	minX, minY, maxX, maxY := int64(math.MaxInt64), int64(math.MaxInt64), int64(math.MinInt64), int64(math.MinInt64)
	acc := func(p *geom.Point) {
		if p == nil {
			return
		}
		minX, maxX = min(minX, p.X), max(maxX, p.X)
		minY, maxY = min(minY, p.Y), max(maxY, p.Y)
	}
	for _, sh := range s.GetShapes() {
		for _, p := range sh.GetPoints() {
			acc(p)
		}
	}
	for _, pn := range s.GetPins() {
		acc(pn.GetLoc())
	}
	if maxX < minX {
		return nodeSize{}
	}
	return nodeSize{maxX - minX, maxY - minY}
}

// sortedByCell returns the symbols keyed by cell_ref in a deterministic (cell_ref-sorted) order.
func sortedByCell(byCell map[string]*geom.SymbolDef) []*geom.SymbolDef {
	cells := make([]string, 0, len(byCell))
	for c := range byCell {
		cells = append(cells, c)
	}
	sort.Strings(cells)
	out := make([]*geom.SymbolDef, 0, len(cells))
	for _, c := range cells {
		out = append(out, byCell[c])
	}
	return out
}

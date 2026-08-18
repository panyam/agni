package geomath

import geom "github.com/panyam/agni/gen/go/agni/v1/geom"

// SymbolIndex is a geometry's symbol table keyed for placement lookup, with the fallbacks a
// placement may need when its refs do not match a definition exactly.
//
// It lives here rather than in core/render because more than one tier has to answer "does this
// placement draw?" and they must not answer it differently. The renderer asks in order to draw; the
// reader asks in order to report what it could not draw (agni issue 354); validate asks in order to
// judge a read's health. Those three had two different joins between them, so a placement resolvable
// by the renderer's cell-only fallback counted as UNRESOLVED to validate, and any shortfall report
// built on that would have named placements that draw perfectly well.
//
// It is also the only home readers can use. C17 forbids a reader importing core/render, and the
// shortfall has to be computed where geometry is produced.
type SymbolIndex map[string]*geom.SymbolDef

// symKey is the composite lookup key. NUL separates the parts because it cannot occur in a cell,
// library or view ref, so no concatenation of one triple can collide with another.
func symKey(cell, lib, view string) string { return cell + "\x00" + lib + "\x00" + view }

// IndexSymbols builds the lookup table for a geometry's symbol definitions.
//
// Each definition is registered under its exact (cell, library, view) triple and then under two
// progressively looser keys, FIRST DEFINITION WINS, so a placement whose view or library ref does not
// match exactly still resolves. The looser keys are what make a multi-section cell and a
// single-view cell both work off one table.
func IndexSymbols(g *geom.SchematicGeometry) SymbolIndex {
	m := make(SymbolIndex, len(g.GetSymbols()))
	for _, s := range g.GetSymbols() {
		m[symKey(s.GetCellRef(), s.GetLibraryRef(), s.GetViewRef())] = s
		if _, ok := m[symKey(s.GetCellRef(), s.GetLibraryRef(), "")]; !ok {
			m[symKey(s.GetCellRef(), s.GetLibraryRef(), "")] = s
		}
		if _, ok := m[symKey(s.GetCellRef(), "", "")]; !ok {
			m[symKey(s.GetCellRef(), "", "")] = s
		}
	}
	return m
}

// SymbolFor resolves a placement to the definition that will be drawn for it, or nil when nothing
// will be. An exact (cell, library, view) match selects the right bank of a multi-section cell; the
// view- and library-agnostic fallbacks keep single-view cells and any ref mismatch resolving.
//
// A nil answer is what "this placement contributes no shapes" means, so every consumer that wants to
// know whether a placement draws should ask THIS rather than re-deriving the join.
func (m SymbolIndex) SymbolFor(pl *geom.SymbolPlacement) *geom.SymbolDef {
	if s := m[symKey(pl.GetCellRef(), pl.GetLibraryRef(), pl.GetViewRef())]; s != nil {
		return s
	}
	if s := m[symKey(pl.GetCellRef(), pl.GetLibraryRef(), "")]; s != nil {
		return s
	}
	return m[symKey(pl.GetCellRef(), "", "")]
}

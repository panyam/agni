package geomath

import geom "github.com/panyam/agni/gen/go/agni/v1/geom"

// SymbolIndex is a geometry's symbol table keyed for placement lookup, with the fallbacks a
// placement may need when its refs do not match a definition exactly.
//
// The renderer, the readers (to report what they could not draw, agni issue 354) and validate all
// ask "does this placement draw?", and they must get one answer. With separate joins, a placement
// the renderer resolved by its cell-only fallback counted as UNRESOLVED to validate. It lives here
// rather than core/render because C17 forbids a reader importing core/render.
type SymbolIndex map[string]*geom.SymbolDef

// symKey is the composite lookup key. NUL separates the parts because it cannot occur in a cell,
// library or view ref, so no concatenation of one triple can collide with another.
func symKey(cell, lib, view string) string { return cell + "\x00" + lib + "\x00" + view }

// IndexSymbols builds the lookup table for a geometry's symbol definitions.
//
// Each definition is registered under its exact (cell, library, view) triple and then under two
// progressively looser keys, FIRST DEFINITION WINS, so a placement whose view or library ref does not
// match exactly still resolves. That lets a multi-section cell and a single-view cell share one table.
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
// nil means the placement contributes no shapes. A consumer asking whether a placement draws should
// call THIS rather than re-deriving the join.
func (m SymbolIndex) SymbolFor(pl *geom.SymbolPlacement) *geom.SymbolDef {
	if s := m[symKey(pl.GetCellRef(), pl.GetLibraryRef(), pl.GetViewRef())]; s != nil {
		return s
	}
	if s := m[symKey(pl.GetCellRef(), pl.GetLibraryRef(), "")]; s != nil {
		return s
	}
	return m[symKey(pl.GetCellRef(), "", "")]
}

// MarkUndrawn fills a geometry's `undrawn` list with every placement no symbol resolves for, in sheet
// then placement order so the answer is stable across runs.
//
// Call it where geometry is PRODUCED, so every consumer reads one list computed with the renderer's
// own resolution. It overwrites rather than appends, so re-running it is a no-op. A geometry with no
// placements yields an empty list.
func MarkUndrawn(g *geom.SchematicGeometry) {
	if g == nil {
		return
	}
	ix := IndexSymbols(g)
	var out []*geom.UndrawnPlacement
	for _, sh := range g.GetSheets() {
		for _, pl := range sh.GetPlacements() {
			if ix.SymbolFor(pl) != nil {
				continue
			}
			out = append(out, &geom.UndrawnPlacement{
				RefDes:     pl.GetRefDes(),
				CellRef:    pl.GetCellRef(),
				LibraryRef: pl.GetLibraryRef(),
				SheetId:    sh.GetId(),
			})
		}
	}
	g.Undrawn = out
}

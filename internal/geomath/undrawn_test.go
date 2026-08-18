package geomath

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// MarkUndrawn is the whole of agni issue 354 on the engine side: a render that loses a symbol still
// produces a complete-LOOKING sheet, so the only way a reader learns the drawing is short is if the
// geometry says so.
func TestMarkUndrawnRecordsOnlyWhatWillNotDraw(t *testing.T) {
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{CellRef: "R", LibraryRef: "Device", ViewRef: "v1"}},
		Sheets: []*geom.SheetGeometry{{Id: "s1", Placements: []*geom.SymbolPlacement{
			{RefDes: "R1", CellRef: "R", LibraryRef: "Device", ViewRef: "v1"},
			{RefDes: "R2", CellRef: "R"}, // resolves through the cell-only fallback
			{RefDes: "U1", CellRef: "MCU", LibraryRef: "Acme"},
		}}},
	}
	MarkUndrawn(g)
	if len(g.GetUndrawn()) != 1 {
		t.Fatalf("undrawn = %d, want 1 (only U1): %+v", len(g.GetUndrawn()), g.GetUndrawn())
	}
	u := g.GetUndrawn()[0]
	if u.GetRefDes() != "U1" {
		t.Errorf("undrawn ref_des = %q, want U1", u.GetRefDes())
	}
	// What the placement ASKED FOR, which is what a reader pastes into a --symbol-path search.
	if u.GetCellRef() != "MCU" || u.GetLibraryRef() != "Acme" {
		t.Errorf("undrawn = %+v, want the refs the placement asked for", u)
	}
	if u.GetSheetId() != "s1" {
		t.Errorf("undrawn sheet_id = %q, want s1", u.GetSheetId())
	}
}

// R2 above is the reason the join is shared. A stricter join would report it undrawn while the
// renderer draws it, and a banner naming a part that is plainly on screen is worse than no banner.
func TestMarkUndrawnDoesNotReportWhatTheRendererDraws(t *testing.T) {
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{CellRef: "R", LibraryRef: "Device", ViewRef: "v1"}},
		Sheets:  []*geom.SheetGeometry{{Id: "s1", Placements: []*geom.SymbolPlacement{{RefDes: "R2", CellRef: "R"}}}},
	}
	MarkUndrawn(g)
	if len(g.GetUndrawn()) != 0 {
		t.Errorf("a placement resolving through a fallback must not be reported undrawn: %+v", g.GetUndrawn())
	}
}

// A complete drawing must report nothing, or the banner appears on every render and readers learn to
// ignore it. The positive control for the test above.
func TestMarkUndrawnIsEmptyWhenEverythingResolves(t *testing.T) {
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{CellRef: "R", LibraryRef: "Device"}},
		Sheets:  []*geom.SheetGeometry{{Id: "s1", Placements: []*geom.SymbolPlacement{{RefDes: "R1", CellRef: "R", LibraryRef: "Device"}}}},
	}
	MarkUndrawn(g)
	if len(g.GetUndrawn()) != 0 {
		t.Errorf("a complete drawing must report nothing undrawn, got %+v", g.GetUndrawn())
	}
}

// A sheet that draws nothing has not FAILED to draw anything. Without this, an empty geometry would
// look like a total failure and a caller could not tell the two apart.
func TestMarkUndrawnDistinguishesEmptyFromShort(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "s1"}}}
	MarkUndrawn(g)
	if len(g.GetUndrawn()) != 0 {
		t.Errorf("no placements means nothing was undrawn, got %+v", g.GetUndrawn())
	}
}

// Re-running must not double the list. The fill happens where geometry is produced, and more than one
// path can reach a geometry, so idempotence is the property that keeps a second pass harmless.
func TestMarkUndrawnIsIdempotent(t *testing.T) {
	g := &geom.SchematicGeometry{
		Sheets: []*geom.SheetGeometry{{Id: "s1", Placements: []*geom.SymbolPlacement{{RefDes: "U1", CellRef: "MCU"}}}},
	}
	MarkUndrawn(g)
	MarkUndrawn(g)
	if len(g.GetUndrawn()) != 1 {
		t.Errorf("undrawn = %d after two passes, want 1", len(g.GetUndrawn()))
	}
}

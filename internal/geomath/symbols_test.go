package geomath

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// oneSymbol is a geometry whose single definition is registered under a library, and one placement
// per resolution path the index is supposed to support.
func oneSymbol() *geom.SchematicGeometry {
	return &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{CellRef: "R", LibraryRef: "Device", ViewRef: "v1"}},
	}
}

func TestSymbolForResolvesThroughEachFallback(t *testing.T) {
	ix := IndexSymbols(oneSymbol())
	for _, tc := range []struct {
		name string
		pl   *geom.SymbolPlacement
		want bool
	}{
		{"exact triple", &geom.SymbolPlacement{CellRef: "R", LibraryRef: "Device", ViewRef: "v1"}, true},
		{"view mismatch falls back to cell+library", &geom.SymbolPlacement{CellRef: "R", LibraryRef: "Device", ViewRef: "other"}, true},
		// The one the two joins disagreed about. A placement naming no library still draws, because
		// the index registers a cell-only key; a join on (cell, library) alone calls it unresolved.
		{"library mismatch falls back to cell alone", &geom.SymbolPlacement{CellRef: "R", LibraryRef: "", ViewRef: ""}, true},
		{"unknown cell resolves to nothing", &geom.SymbolPlacement{CellRef: "NOSUCH", LibraryRef: "Device"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ix.SymbolFor(tc.pl) != nil
			if got != tc.want {
				t.Errorf("SymbolFor(%+v) resolved = %v, want %v", tc.pl, got, tc.want)
			}
		})
	}
}

// The property the whole shortfall report rests on: "does it draw" has ONE answer.
//
// A consumer computing its own join is free to be stricter than the renderer, and a stricter join
// reports a shortfall for placements that draw perfectly well. That report is worse than none: it
// teaches a reader to distrust a banner that is usually wrong.
func TestSymbolForIsTheOnlyJoin(t *testing.T) {
	ix := IndexSymbols(oneSymbol())
	// A cell-only match is exactly the case a naive (cell, library) join gets wrong.
	pl := &geom.SymbolPlacement{CellRef: "R"}
	if ix.SymbolFor(pl) == nil {
		t.Fatal("a cell-only placement must resolve; this is the case the old validate join missed")
	}
	naive := map[string]bool{}
	for _, s := range oneSymbol().GetSymbols() {
		naive[s.GetCellRef()+"|"+s.GetLibraryRef()] = true
	}
	if naive[pl.GetCellRef()+"|"+pl.GetLibraryRef()] {
		t.Fatal("the naive join was supposed to MISS this placement; the fixture no longer demonstrates the bug")
	}
}

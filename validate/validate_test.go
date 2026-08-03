package validate

import (
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestDesign(t *testing.T) {
	if got := Design(nil); len(got) != 1 || got[0] != "no design produced" {
		t.Errorf("nil design = %v", got)
	}
	empty := &ir.Design{}
	if got := Design(empty); len(got) != 2 {
		t.Errorf("empty design = %v, want no-components + no-nets", got)
	}
	ok := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1"}},
		Nets:       []*ir.Net{{Name: "A"}},
	}
	if got := Design(ok); len(got) != 0 {
		t.Errorf("healthy design flagged: %v", got)
	}
}

// twoPlacementGeom builds a geometry with n placements of which resolved join to a symbol.
func geomWith(placements, resolved int) *geom.SchematicGeometry {
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{CellRef: "R", LibraryRef: "L"}},
		Sheets: []*geom.SheetGeometry{{
			Id:    "P1",
			Wires: []*geom.WireGeometry{{Net: "N", Polylines: []*geom.Polyline{{Points: []*geom.Point{{}, {X: 1}}}}}},
		}},
	}
	for i := 0; i < placements; i++ {
		pl := &geom.SymbolPlacement{RefDes: "X", CellRef: "MISSING", LibraryRef: "L"}
		if i < resolved {
			pl.CellRef = "R"
		}
		g.Sheets[0].Placements = append(g.Sheets[0].Placements, pl)
	}
	return g
}

func TestGeometryResolutionRate(t *testing.T) {
	if got := Geometry(geomWith(100, 100)); len(got) != 0 {
		t.Errorf("fully resolved flagged: %v", got)
	}
	if got := Geometry(geomWith(100, 99)); len(got) != 0 {
		t.Errorf("99%% resolution must pass (boundary), got %v", got)
	}
	got := Geometry(geomWith(100, 98))
	if len(got) != 1 || !strings.Contains(got[0], "symbol resolution 98.0%") {
		t.Errorf("98%% resolution must fail with the rate named, got %v", got)
	}
}

func TestGeometryStructure(t *testing.T) {
	if got := Geometry(nil); len(got) != 1 {
		t.Errorf("nil geometry = %v", got)
	}
	empty := &geom.SchematicGeometry{}
	got := Geometry(empty)
	for _, want := range []string{"no symbols", "no sheets", "no placements", "no wires"} {
		found := false
		for _, p := range got {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Errorf("empty geometry missing problem %q in %v", want, got)
		}
	}
}

package formats

import (
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestRelocateSourcesReachesEveryLocator is the point of walking by reflection rather than by hand.
// Each case is a SHAPE the walk has to descend through, not a message that matters in itself: a
// locator directly on the root, one under a repeated field two levels down, one under a repeated
// field of Provenance itself, and one in the geometry sidecar, whose Provenance is a different proto
// type carrying the same field.
//
// A hand-written visitor covers whichever of these its author remembered. What makes that expensive
// is the failure direction: a message added later still walks, still writes, and quietly keeps the
// host path, so the output looks exactly like a correct one.
func TestRelocateSourcesReachesEveryLocator(t *testing.T) {
	const abs = "/home/someone/work/board.edn"
	rel := func(string) string { return "designs/d/board.edn" }

	t.Run("netlist IR", func(t *testing.T) {
		d := &ir.Design{
			Prov: &ir.Provenance{SourceFile: abs},
			Components: []*ir.Component{{
				RefDes:   "R1",
				Sections: []*ir.ComponentSection{{Prov: &ir.Provenance{SourceFile: abs}}},
			}},
			InputDiagnostics: &ir.InputDiagnostics{
				RefDesCollisions: []*ir.RefDesCollision{{
					RefDes:    "R1",
					Instances: []*ir.Provenance{{SourceFile: abs}, {SourceFile: abs}},
				}},
			},
		}
		relocateSources(d, rel)
		assertNoAbsolute(t, d.String())
	})

	t.Run("geometry sidecar", func(t *testing.T) {
		g := &geom.SchematicGeometry{
			Sheets: []*geom.SheetGeometry{{
				Placements: []*geom.SymbolPlacement{{Prov: &geom.Provenance{SourceFile: abs}}},
			}},
		}
		relocateSources(g, rel)
		assertNoAbsolute(t, g.String())
	})
}

// TestRelocateSourcesWithoutAName pins the identity case, since a loader reading through an fs.FS
// already holds unrooted paths and sets no name function. Passing nil has to leave the tree alone
// rather than blank a locator, which is the shape a nil-tolerant hook usually gets wrong.
func TestRelocateSourcesWithoutAName(t *testing.T) {
	d := &ir.Design{Prov: &ir.Provenance{SourceFile: "designs/d/board.edn"}}
	relocateSources(d, nil)
	if got := d.GetProv().GetSourceFile(); got != "designs/d/board.edn" {
		t.Errorf("source_file = %q, want it untouched", got)
	}
}

func assertNoAbsolute(t *testing.T, dump string) {
	t.Helper()
	if strings.Contains(dump, "/home/someone") {
		t.Errorf("a locator kept its host path:\n%s", dump)
	}
	if !strings.Contains(dump, "designs/d/board.edn") {
		t.Errorf("no locator was rewritten at all, so the walk proved nothing:\n%s", dump)
	}
}

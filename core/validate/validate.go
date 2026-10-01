// Package validate holds the reader-health invariants behind `agni validate` (WS6-007), which
// are structural sanity checks over what a reader produced. They catch "parsed but empty" and
// "placements that resolve to nothing" regressions that per-fixture unit tests miss on real
// files. Design-rule checking is core/check. These are pure functions over parsed structures,
// and file I/O and format dispatch stay in the CLI (CONSTRAINTS C1).
package validate

import (
	"fmt"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// MinResolutionRate is the fraction of placements that must resolve to a symbol
// definition for a geometry to pass. Real exports resolve essentially everything, so a
// lower rate signals a reader regression (it caught the EDIF id-normalization bug). It is
// not 1.0 because real libraries occasionally carry a definition-less decorative instance.
const MinResolutionRate = 0.99

// Design returns the netlist-tier problems with a parsed design, empty when it passes. A
// design that parsed but carries no components or no nets is the usual silent reader
// regression.
func Design(d *ir.Design) []string {
	if d == nil {
		return []string{"no design produced"}
	}
	var problems []string
	if len(d.Components) == 0 {
		problems = append(problems, "no components")
	}
	if len(d.Nets) == 0 {
		problems = append(problems, "no nets")
	}
	return problems
}

// Geometry returns the drawing-tier problems with a parsed schematic geometry, empty when
// it passes. Beyond non-empty structure, placements must join to symbol definitions at
// MinResolutionRate.
func Geometry(g *geom.SchematicGeometry) []string {
	if g == nil {
		return []string{"no geometry produced"}
	}
	var problems []string
	if len(g.Symbols) == 0 {
		problems = append(problems, "no symbols")
	}
	if len(g.Sheets) == 0 {
		problems = append(problems, "no sheets")
	}
	placements, wires := 0, 0
	for _, s := range g.Sheets {
		placements += len(s.Placements)
		wires += len(s.Wires)
	}
	if placements == 0 {
		problems = append(problems, "no placements")
	}
	if wires == 0 {
		problems = append(problems, "no wires")
	}
	if placements > 0 {
		if rate := float64(Resolved(g)) / float64(placements); rate < MinResolutionRate {
			problems = append(problems, fmt.Sprintf("symbol resolution %.1f%% (%d/%d), want >= %.0f%%",
				rate*100, Resolved(g), placements, MinResolutionRate*100))
		}
	}
	return problems
}

// Resolved counts the placements that will actually DRAW, through the renderers' own join
// (geomath.IndexSymbols, including its cell-only fallback), so "does it draw" has one answer
// (agni issue 354).
func Resolved(g *geom.SchematicGeometry) int {
	ix := geomath.IndexSymbols(g)
	n := 0
	for _, sh := range g.GetSheets() {
		for _, pl := range sh.GetPlacements() {
			if ix.SymbolFor(pl) != nil {
				n++
			}
		}
	}
	return n
}

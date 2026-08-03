// Package validate holds the reader-health invariants behind `agni validate` (WS6-007):
// structural sanity checks over what a reader produced, catching "parsed but empty" and
// "placements that resolve to nothing" regressions that per-fixture unit tests miss on
// real files. It is not design-rule checking (that is check/, over the design's meaning);
// these invariants are about the reader. Pure functions over parsed structures — file I/O
// and format dispatch stay at the edge (CONSTRAINTS C1); the CLI walks files through
// formats and calls these.
package validate

import (
	"fmt"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// MinResolutionRate is the fraction of placements that must resolve to a symbol
// definition for a geometry to pass. Real exports resolve essentially everything once
// cell/library ids normalize correctly, so a lower rate signals a reader regression (the
// invariant that caught the EDIF id-normalization bug); it is not 1.0 because real
// libraries occasionally carry a genuinely definition-less decorative instance.
const MinResolutionRate = 0.99

// Design returns the netlist-tier problems with a parsed design: empty problems means it
// passes. A design that parsed but carries no components or no nets is the classic
// silent-reader-regression shape.
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

// Geometry returns the drawing-tier problems with a parsed schematic geometry: empty
// problems means it passes. Beyond non-empty structure, every placement should join to a
// symbol definition (see MinResolutionRate).
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

// Resolved counts the placements whose (cell_ref, library_ref) joins to a symbol
// definition in the geometry's library — the join the renderers draw by.
func Resolved(g *geom.SchematicGeometry) int {
	byKey := map[string]bool{}
	for _, s := range g.Symbols {
		byKey[s.CellRef+"|"+s.LibraryRef] = true
	}
	n := 0
	for _, sh := range g.Sheets {
		for _, pl := range sh.Placements {
			if byKey[pl.CellRef+"|"+pl.LibraryRef] {
				n++
			}
		}
	}
	return n
}

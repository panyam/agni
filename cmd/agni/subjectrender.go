package main

import (
	"fmt"
	"io"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"

	"github.com/panyam/agni/core/render"
)

// renderSubjects draws one design with a set of entities baked in as highlight overlays, and is the
// shared half of every command that can point at something on a drawing.
//
// The split is deliberate. Turning a command's own answer into subjects is TYPED and lives with that
// command (traceSpecs here, findingSpecs in reviewrender.go), while loading the geometry, picking a
// sheet and emitting the SVG is the same work every time and lives here. A single untyped
// "highlight these strings" entry point would be shorter and would put the interesting decision, what
// counts as a subject of this answer, somewhere nobody reviews it.
//
// It draws the design's OWN sheets when it has them and falls back to an auto-layout when it does
// not, saying which on stderr. Most designs a reader points at are netlists with no drawn schematic,
// and refusing those would make the flag useless exactly where a picture helps most: an auto-layout
// is a true picture of the connectivity, and the note is what stops it being mistaken for the
// schematic somebody drew.
func renderSubjects(errOut io.Writer, named, out string, specs []*geom.HighlightSpec) error {
	// The resolver's note is DROPPED here, and errOut carries only this function's own. Every caller
	// had to read the design to compute its subjects, so it has already resolved and reported the
	// same design, and renderSource produces the identical sentence. What errOut must still carry is
	// the auto-layout fallback below, which is a statement about the DRAWING rather than about which
	// file was read, and which nothing else is in a position to make.
	file, _ := renderSource(named)
	reg, err := buildRegistry(nil, "")
	if err != nil {
		return err
	}
	rl, err := renderLoader(file)
	if err != nil {
		return err
	}
	g, err := rl.ResolveGeometry(file, faithfulLayout, reg, symbolsGlyph)
	if err != nil || len(g.GetSheets()) == 0 {
		g, err = rl.ResolveGeometry(file, "force", reg, symbolsGlyph)
		if err != nil {
			return err
		}
		if len(g.GetSheets()) == 0 {
			return fmt.Errorf("no sheets to draw for %s", file)
		}
		fmt.Fprintf(errOut, "note: %s carries no drawn schematic, so this is an auto-layout of its netlist.\n", file)
	}
	sheet, err := render.PickSheet(g, "0")
	if err != nil {
		return err
	}
	return writeRender(out, g, sheet, "svg", specs)
}

package main

import (
	"fmt"
	"io"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"

	"github.com/panyam/agni/core/render"
)

// renderSubjects draws one design with a set of entities baked in as highlight overlays. It is the
// shared half of every command that can point at something on a drawing. Turning a command's answer
// into subjects is TYPED and stays with that command (traceSpecs in trace.go, findingSpecs in
// reviewrender.go).
//
// It draws the design's OWN sheets when it has them and otherwise falls back to an auto-layout,
// saying so on stderr. Most designs are netlists with no drawn schematic, and the note stops the
// auto-layout being mistaken for one somebody drew.
func renderSubjects(errOut io.Writer, named, out, sheetID string, specs []*geom.HighlightSpec) error {
	// The resolver's note is DROPPED. Every caller already read the design to compute its subjects
	// and printed the same sentence. errOut carries only the auto-layout note below.
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
	// The sheet the ANSWER is on, when the caller knows it, rather than always sheet "0" (on one
	// 82-sheet export that was a table of contents with no wires, agni issue 657). A sheet id the loaded
	// geometry does not hold falls back to "0" rather than failing, because the auto-layout has its
	// own sheet names and a sheet id from the faithful drawing means nothing there.
	sheet, err := render.PickSheet(g, sheetID)
	if err != nil {
		if sheet, err = render.PickSheet(g, "0"); err != nil {
			return err
		}
	}
	return writeRender(out, g, sheet, "svg", specs)
}

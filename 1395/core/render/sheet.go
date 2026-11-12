package render

import (
	"fmt"
	"strconv"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// PickSheet resolves a sheet selector (a numeric index, a sheet id or a sheet name) against a
// schematic's sheets. An id or name match wins over an index, so a single-sheet format that ids
// its page "1" is still selectable by that id. It does no I/O, and both the CLI render command
// and the web design service call it.
func PickSheet(g *geom.SchematicGeometry, sel string) (*geom.SheetGeometry, error) {
	for _, s := range g.Sheets {
		if s.Id == sel || s.Name == sel {
			return s, nil
		}
	}
	if i, err := strconv.Atoi(sel); err == nil {
		if i < 0 || i >= len(g.Sheets) {
			return nil, fmt.Errorf("sheet index %d out of range (0..%d)", i, len(g.Sheets)-1)
		}
		return g.Sheets[i], nil
	}
	return nil, fmt.Errorf("no sheet with id/name %q", sel)
}

// SheetIndex returns the 0-based position of sheet in the geometry's sheet list, or 0 if not
// found. It maps a selected sheet to a tool's 1-based page order (the native renderer).
func SheetIndex(g *geom.SchematicGeometry, sheet *geom.SheetGeometry) int {
	for i, sh := range g.GetSheets() {
		if sh == sheet {
			return i
		}
	}
	return 0
}

package render

import (
	"fmt"
	"strconv"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// PickSheet resolves a sheet selector against a schematic's sheets: a numeric index, or a match
// on sheet id or name. An explicit id/name match wins over a positional index, so a sheet whose
// id happens to be numeric (e.g. a flat single-sheet format that ids its page "1") is still
// selectable by that id rather than being misread as an out-of-range index. This is a pure
// geometry operation (no I/O), shared by the CLI render command and the web design service.
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

package common

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/readers/edif"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// LoadSchematic reads a schematic geometry sidecar from arg, which an example takes as user
// input. arg may be a filesystem path (relative to the working directory, e.g.
// "../common/designs/demo-schematic.eds") or the bare name of a bundled fixture. It reads the
// path from disk first, so a user can point the example at their own schematic, then falls
// back to the embedded fixture whose base name matches, mirroring Load. Only EDIF .eds carries
// this geometry today, so that is the reader used. A file that exists but fails to parse is
// reported, not masked by the fallback.
func LoadSchematic(arg string) (*geom.SchematicGeometry, error) {
	f, err := os.Open(arg)
	if err == nil {
		defer f.Close()
		return edif.ReadSchematic(f, arg)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if g, ferr := ReadSchematicFixture(filepath.Base(arg)); ferr == nil {
		return g, nil
	}
	return nil, fmt.Errorf("no schematic at path %q, and no bundled fixture named %q", arg, filepath.Base(arg))
}

// ReadSchematicFixture decodes a bundled EDIF schematic (.eds) fixture into the geometry
// sidecar, as edif.ReadSchematic does for on-disk files. Geometry is a separate artifact
// from the netlist IR (ReadFixture): it feeds the SVG and WebGL renderers, keyed to the IR,
// and is never the IR itself. File paths stay at the edge; the core reader sees an io.Reader
// (CONSTRAINTS C1).
func ReadSchematicFixture(name string) (*geom.SchematicGeometry, error) {
	f, err := designsFS.Open("designs/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return edif.ReadSchematic(f, name)
}

// GeomStatsLines summarizes a schematic geometry sidecar for narration: the symbol library
// size and the per-sheet placement, wire, and label counts.
func GeomStatsLines(g *geom.SchematicGeometry) string {
	placements, wires, labels := 0, 0, 0
	for _, s := range g.Sheets {
		placements += len(s.Placements)
		labels += len(s.Labels)
		for _, w := range s.Wires {
			wires += len(w.Polylines)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "design:      %s\n", g.DesignRef)
	fmt.Fprintf(&b, "unit_nm:     %d\n", g.UnitNm)
	fmt.Fprintf(&b, "symbols:     %d (library)\n", len(g.Symbols))
	fmt.Fprintf(&b, "sheets:      %d\n", len(g.Sheets))
	fmt.Fprintf(&b, "placements:  %d\n", placements)
	fmt.Fprintf(&b, "wires:       %d (polylines)\n", wires)
	fmt.Fprintf(&b, "labels:      %d", labels)
	return b.String()
}

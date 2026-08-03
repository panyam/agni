package render

import (
	"flag"

	"os"
	"path/filepath"
	"strings"
	"testing"

	"bytes"

	"github.com/panyam/agni/readers/edif"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/graph"
	"github.com/panyam/agni/readers/kicad"
)

var update = flag.Bool("update", false, "rewrite the golden SVGs under testdata/golden from the current renderer output")

// goldenCompare asserts got matches the committed golden byte-for-byte (or rewrites it under
// -update). The substring assertions elsewhere in this package cannot catch a transform
// regression that moves every element while keeping the markup; an exact golden can.
func goldenCompare(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (regenerate with `go test ./render/ -run Golden -update`): %v", err)
	}
	if got == string(want) {
		return
	}
	// Report the first diverging line so the failure is readable without a diff tool.
	gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gl) && i < len(wl); i++ {
		if gl[i] != wl[i] {
			t.Fatalf("%s: line %d diverges from golden\n got: %s\nwant: %s\n(regenerate with -update if intentional)", name, i+1, gl[i], wl[i])
		}
	}
	t.Fatalf("%s: length differs from golden (%d vs %d lines); regenerate with -update if intentional", name, len(gl), len(wl))
}

// faithfulFixtureGeometry loads the multi-purpose .eds fixture through the real reader, so the
// golden covers reader output shapes, not just hand-built geometry.
func faithfulFixtureGeometry(t testing.TB) *geom.SchematicGeometry {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "readers", "edif", "testdata", "sample.eds"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := edif.ReadSchematic(f, "sample.eds")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestGoldenSVGFaithful(t *testing.T) {
	g := faithfulFixtureGeometry(t)
	sheet, err := PickSheet(g, "0")
	if err != nil {
		t.Fatal(err)
	}
	goldenCompare(t, "faithful-sample.svg", SheetSVG(g, sheet))
}

func TestGoldenSVGAutoLayout(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "readers", "edif", "testdata", "basic.edn"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := edif.Read(f, "basic.edn")
	if err != nil {
		t.Fatal(err)
	}
	g, err := graph.LayoutWith(d, "grid")
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := PickSheet(g, "0")
	if err != nil {
		t.Fatal(err)
	}
	svg := SheetSVG(g, sheet)
	// Guard the golden's premise before comparing: the layout must be deterministic, or the
	// golden would flake rather than catch regressions.
	if g2, _ := graph.LayoutWith(d, "grid"); SheetSVG(g2, mustPick(t, g2)) != svg {
		t.Fatal("grid layout is not deterministic; a golden cannot guard it")
	}
	goldenCompare(t, "grid-basic.svg", svg)
}

// TestGoldenSVGBus renders a faithful KiCad schematic carrying a bus trunk, a bus entry, and a
// plain wire (WS7-042), so the golden captures the bus styling (thick blue trunk vs thin green
// wire) and would catch a regression that stops drawing or restyles buses.
func TestGoldenSVGBus(t *testing.T) {
	src := "bus-render.kicad_sch"
	raw, err := os.ReadFile(filepath.Join("..", "readers", "kicad", "testdata", src))
	if err != nil {
		t.Fatal(err)
	}
	g, err := kicad.ReadSchematicGeometry(bytes.NewReader(raw), src)
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := PickSheet(g, "root")
	if err != nil {
		t.Fatal(err)
	}
	goldenCompare(t, "bus-render.svg", SheetSVG(g, sheet))
}

func mustPick(t *testing.T, g *geom.SchematicGeometry) *geom.SheetGeometry {
	t.Helper()
	sheet, err := PickSheet(g, "0")
	if err != nil {
		t.Fatal(err)
	}
	return sheet
}

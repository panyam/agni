package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/render"
)

// TestBuildRegistry covers the --class / --class-file plumbing (WS7-030): a flag rule steers
// classification, a file rule layers the same way, an unknown class is rejected with the known
// classes listed, and bad syntax errors.
func TestBuildRegistry(t *testing.T) {
	resComp := &ir.Component{RefDes: "R1", Sections: []*ir.ComponentSection{{PartRef: "res"}}}
	parts := map[string]*ir.PartType{"res": {Name: "res"}}

	// A flag override wins over the default res -> resistor.
	reg, err := buildRegistry([]string{"res=capacitor"}, "")
	if err != nil {
		t.Fatalf("buildRegistry: %v", err)
	}
	if got := reg.Classify(resComp, parts); got != graph.ClassCapacitor {
		t.Errorf("--class res=capacitor: classify = %q, want capacitor", got)
	}

	// A class-file rule layers the same way (comments and blanks ignored).
	dir := t.TempDir()
	f := filepath.Join(dir, "classes")
	if err := os.WriteFile(f, []byte("# my parts\nres = inductor\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err = buildRegistry(nil, f)
	if err != nil {
		t.Fatalf("buildRegistry(file): %v", err)
	}
	if got := reg.Classify(resComp, parts); got != graph.ClassInductor {
		t.Errorf("--class-file res=inductor: classify = %q, want inductor", got)
	}

	// Unknown class is rejected, and the error names the valid classes.
	if _, err := buildRegistry([]string{"res=wormhole"}, ""); err == nil || !strings.Contains(err.Error(), "resistor") {
		t.Errorf("unknown class should error listing known classes, got %v", err)
	}
	// Missing '=' is a syntax error.
	if _, err := buildRegistry([]string{"noequals"}, ""); err == nil {
		t.Error("rule without '=' should error")
	}
}

// A sheet whose id is numeric (e.g. a flat single-sheet .sch) must be selectable by that id, not
// misread as a positional index. Regression: the viewer requests the id from GetDesign, so an
// id of "1" was read as index 1 -> out of range -> a blank sheet.
func TestPickSheetIdBeatsIndex(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "1", Name: "root"}}}
	if s, err := render.PickSheet(g, "1"); err != nil || s.Id != "1" {
		t.Errorf(`render.PickSheet("1") = %v, %v; want the id-"1" sheet`, s, err)
	}
	if s, err := render.PickSheet(g, "root"); err != nil || s.Name != "root" {
		t.Errorf(`render.PickSheet("root") = %v, %v; want match by name`, s, err)
	}
	// A numeric selector with no id/name match still falls back to a positional index.
	if _, err := render.PickSheet(g, "0"); err != nil {
		t.Errorf(`render.PickSheet("0") index fallback failed: %v`, err)
	}
}

func oneSheetGeom() (*geom.SchematicGeometry, *geom.SheetGeometry) {
	sheet := &geom.SheetGeometry{
		Id:   "s0",
		Name: "S0",
		Placements: []*geom.SymbolPlacement{
			{RefDes: "R1", CellRef: "__node__", Transform: &geom.Transform{Origin: &geom.Point{}}},
		},
	}
	g := &geom.SchematicGeometry{Symbols: []*geom.SymbolDef{{CellRef: "__node__"}}, Sheets: []*geom.SheetGeometry{sheet}}
	return g, sheet
}

func TestRenderGeometrySVG(t *testing.T) {
	g, sheet := oneSheetGeom()
	var b bytes.Buffer
	if _, err := renderGeometry(&b, g, sheet, "svg"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "<svg") {
		t.Errorf("svg output missing <svg element: %q", b.String()[:min(40, b.Len())])
	}
}

func TestRenderGeometryPack(t *testing.T) {
	g, sheet := oneSheetGeom()
	var b bytes.Buffer
	if _, err := renderGeometry(&b, g, sheet, "pack"); err != nil {
		t.Fatal(err)
	}
	if b.Len() == 0 {
		t.Fatal("pack output empty")
	}
	if err := proto.Unmarshal(b.Bytes(), &geom.PackedSheet{}); err != nil {
		t.Errorf("pack output is not a PackedSheet: %v", err)
	}
}

func TestRenderGeometryPNGNotImplemented(t *testing.T) {
	g, sheet := oneSheetGeom()
	if _, err := renderGeometry(io.Discard, g, sheet, "png"); err == nil || !strings.Contains(err.Error(), "png") {
		t.Errorf("want a png-not-implemented error, got %v", err)
	}
}

func TestRenderGeometryUnknownFormat(t *testing.T) {
	g, sheet := oneSheetGeom()
	if _, err := renderGeometry(io.Discard, g, sheet, "bogus"); err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Errorf("want an unknown-format error naming the input, got %v", err)
	}
}

// TestWriteReport covers the conversion report (WS7-029): a text summary and a JSON document
// over the same components, and a bad format erroring.
func TestWriteReport(t *testing.T) {
	const fixture = "../../readers/edif/testdata/basic.edn"

	var b bytes.Buffer
	if err := writeReport(&b, fixture, symbolsGlyph, nil, "text"); err != nil {
		t.Fatalf("writeReport text: %v", err)
	}
	if out := b.String(); !strings.Contains(out, "components") {
		t.Errorf("text report missing the header, got %q", out)
	}

	b.Reset()
	if err := writeReport(&b, fixture, symbolsGlyph, nil, "json"); err != nil {
		t.Fatalf("writeReport json: %v", err)
	}
	var rep struct {
		Components []struct {
			RefDes string `json:"ref_des"`
			Kind   string `json:"kind"`
		} `json:"components"`
	}
	if err := json.Unmarshal(b.Bytes(), &rep); err != nil {
		t.Fatalf("report json invalid: %v", err)
	}
	if len(rep.Components) == 0 || rep.Components[0].RefDes == "" || rep.Components[0].Kind == "" {
		t.Errorf("json report components malformed: %+v", rep.Components)
	}

	if err := writeReport(io.Discard, fixture, symbolsGlyph, nil, "bogus"); err == nil {
		t.Error("unknown --report-format should error")
	}
}

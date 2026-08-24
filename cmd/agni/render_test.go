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

	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
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
	if _, err := renderGeometry(&b, g, sheet, "svg", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "<svg") {
		t.Errorf("svg output missing <svg element: %q", b.String()[:min(40, b.Len())])
	}
}

// TestRenderGeometrySVGHighlight: when specs are passed, the svg path bakes the highlight into
// the one document (a wider re-stroke of the matched net's wire), unlike the plain svg render.
func TestRenderGeometrySVGHighlight(t *testing.T) {
	g, sheet := oneSheetGeom()
	sheet.Wires = []*geom.WireGeometry{
		{Net: "NET1", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 100, Y: 0}}}}},
	}
	specs := []*geom.HighlightSpec{{Nets: []string{"NET1"}, Color: "#e11"}}
	var plain, hi bytes.Buffer
	if _, err := renderGeometry(&plain, g, sheet, "svg", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := renderGeometry(&hi, g, sheet, "svg", specs); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.String(), "#e11") {
		t.Error("plain render should not carry the highlight color")
	}
	if !strings.Contains(hi.String(), `stroke="#e11"`) {
		t.Error("highlighted render is missing the baked-in highlight stroke")
	}
}

// TestParseHighlightSpecs covers the --highlight grammar: each subject kind, style keys, the
// net default-to-path shape, and the error cases (no subject, bad pin, bad key, bad alpha).
func TestParseHighlightSpecs(t *testing.T) {
	specs, err := parseHighlightSpecs([]string{
		"net=SCL",
		"ref=U1,shape=rect,color=#0f0,alpha=0.5",
		"pin=U2:7,shape=circle",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("specs = %d, want 3", len(specs))
	}
	// A net defaults to the PATH marker shape (matching the web focus default).
	if specs[0].GetNets()[0] != "SCL" || specs[0].GetShape() != geom.HighlightShape_HIGHLIGHT_SHAPE_PATH {
		t.Errorf("net spec = %+v, want SCL as PATH", specs[0])
	}
	if specs[1].GetComponents()[0] != "U1" || specs[1].GetShape() != geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT ||
		specs[1].GetColor() != "#0f0" || specs[1].GetAlpha() != 0.5 {
		t.Errorf("component spec = %+v", specs[1])
	}
	if p := specs[2].GetPins(); len(p) != 1 || p[0].GetRefDes() != "U2" || p[0].GetPin() != "7" {
		t.Errorf("pin spec = %+v", specs[2].GetPins())
	}

	for _, bad := range []string{"", "shape=rect", "pin=U1", "net=X,zzz=1", "net=X,alpha=9", "notakv", "ref=", "net=", "pin=U1:"} {
		if _, err := parseHighlightSpecs([]string{bad}); err == nil {
			t.Errorf("parseHighlightSpecs(%q) should error", bad)
		}
	}
}

func TestRenderGeometryPack(t *testing.T) {
	g, sheet := oneSheetGeom()
	var b bytes.Buffer
	if _, err := renderGeometry(&b, g, sheet, "pack", nil); err != nil {
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
	if _, err := renderGeometry(io.Discard, g, sheet, "png", nil); err == nil || !strings.Contains(err.Error(), "png") {
		t.Errorf("want a png-not-implemented error, got %v", err)
	}
}

func TestRenderGeometryUnknownFormat(t *testing.T) {
	g, sheet := oneSheetGeom()
	if _, err := renderGeometry(io.Discard, g, sheet, "bogus", nil); err == nil || !strings.Contains(err.Error(), "bogus") {
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

// TestRenderResolvesDeclaredGeometryCompanion: pointing render at a DESIGN draws the schematic the
// design declares, rather than trying to open the folder as a file.
//
// It failed with `no reader for "" files` before, which is what a directory path looks like to the
// format registry. The information was never missing: `agni stats` on the same folder already
// reported "sheets from gateway.kicad_sch", because it resolves the descriptor and render did not.
func TestRenderResolvesDeclaredGeometryCompanion(t *testing.T) {
	const design = "../../examples/tutorial-project/designs/gateway"
	got, note := renderSource(design)
	if !strings.HasSuffix(got, "gateway.kicad_sch") {
		t.Fatalf("render source = %q, want the declared schematic companion", got)
	}
	// The redirect has to SAY so. Which artifact was drawn is not recoverable from an SVG, and a
	// silent redirect is the failure noteSource exists to prevent for the netlist side.
	if note == "" {
		t.Error("redirected to a companion without a note saying which file was drawn")
	}
}

// TestRenderCmdDrawsADesignFolder is the end-to-end half, and it is the one that matters. The helper
// test above passes on a build where renderCmd never calls renderSource at all, which is precisely
// the bug being fixed: the resolution existed elsewhere in the CLI and this command did not reach for
// it.
func TestRenderCmdDrawsADesignFolder(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sheet.svg")
	runCLI(t, renderCmd(), "../../examples/tutorial-project/designs/gateway", "-o", out)
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no svg written for a design folder: %v", err)
	}
	// The declared companion is a real schematic, so the drawing has wires in it. An empty or
	// placeholder sheet would still be an SVG.
	if !strings.Contains(string(b), "<svg") || !strings.Contains(string(b), "polyline") {
		t.Errorf("rendered %d bytes without the drawn sheet's wires", len(b))
	}
}

// TestRenderSourceLeavesALooseFileAlone: the ordinary case, a file belonging to no design, must pass
// through untouched and without a note. Without this the test above passes on a resolver that
// redirects everything.
func TestRenderSourceLeavesALooseFileAlone(t *testing.T) {
	const loose = "testdata/conformance/showcase.fires.kicad_sch"
	got, note := renderSource(loose)
	if got != loose {
		t.Errorf("render source = %q, want the path as named", got)
	}
	if note != "" {
		t.Errorf("note = %q, want silence when nothing was redirected", note)
	}
}

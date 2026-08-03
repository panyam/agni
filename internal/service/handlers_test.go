package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/graph"
	"github.com/panyam/agni/render"
)

// twoSheetGeom is a minimal drawable geometry with two identifiable sheets, enough for
// GetDesign's sheet listing and GetSheet's selector to have something to choose between.
func twoSheetGeom() *geom.SchematicGeometry {
	return &geom.SchematicGeometry{
		DesignRef: "TOPGEOM",
		Sheets: []*geom.SheetGeometry{
			{Id: "P1", Name: "Root", Wires: []*geom.WireGeometry{{Net: "N1", Polylines: []*geom.Polyline{
				{Points: []*geom.Point{{X: 0, Y: 0}, {X: 10, Y: 0}}},
			}}}},
			{Id: "P2", Name: "Power", ParentId: "P1"},
		},
	}
}

// availNative reports NATIVE as offerable without rendering anything, to observe the
// native_available bit independently of the render path.
type availNative struct{ noNative }

func (availNative) Available(string, string) bool { return true }

func TestGetDesignFaithful(t *testing.T) {
	svc := NewDesignService(fakeLoader{geom: twoSheetGeom()}, availNative{}, render.Style{})
	resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Mount: "m", Path: "x.eds"})
	if err != nil {
		t.Fatal(err)
	}
	m := resp
	if m.GetLayout() != "faithful" {
		t.Errorf("layout = %q, want faithful (the .eds default)", m.GetLayout())
	}
	if m.GetName() != "TOPGEOM" {
		t.Errorf("name = %q, want the geometry design_ref (this .eds schematic has no netlist instances)", m.GetName())
	}
	if m.GetComponentCount() != 0 || m.GetNetCount() != 0 {
		t.Errorf("counts = %d/%d, want 0/0 (this .eds schematic has no netlist instances)", m.GetComponentCount(), m.GetNetCount())
	}
	// .eds is dual-capability (netlist + faithful geometry), so it offers the netlist auto-layouts
	// alongside faithful — capability comes from the registry, not this fixture's (empty) instances.
	if !slicesEqual(m.GetAvailableLayouts(), []string{"faithful", "force", "grid", "layered", "orthogonal", "stress"}) {
		t.Errorf("available layouts = %v, want faithful + the netlist set", m.GetAvailableLayouts())
	}
	if !m.GetNativeAvailable() {
		t.Error("native_available not carried through from the NativeRenderer port")
	}
	if len(m.GetSheets()) != 2 || m.GetSheets()[0].GetId() != "P1" || m.GetSheets()[1].GetParentId() != "P1" {
		t.Errorf("sheets = %+v, want P1 then P2 with parent P1", m.GetSheets())
	}
}

func TestGetDesignAutoLayoutCountsFromIR(t *testing.T) {
	d := &ir.Design{
		Name:         "BOARD",
		SourceFormat: "edif-2.0.0",
		Components:   []*ir.Component{{RefDes: "R1"}, {RefDes: "R2"}},
		Nets:         []*ir.Net{{Name: "A"}, {Name: "B"}, {Name: "C"}},
	}
	svc := NewDesignService(fakeLoader{design: d, geom: twoSheetGeom()}, noNative{}, render.Style{})
	resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Mount: "m", Path: "x.edn"})
	if err != nil {
		t.Fatal(err)
	}
	m := resp
	if m.GetLayout() != "grid" {
		t.Errorf("layout = %q, want grid (the netlist default)", m.GetLayout())
	}
	if m.GetName() != "BOARD" || m.GetSourceFormat() != "edif-2.0.0" {
		t.Errorf("identity = %q/%q, want from the netlist IR", m.GetName(), m.GetSourceFormat())
	}
	if m.GetComponentCount() != 2 || m.GetNetCount() != 3 {
		t.Errorf("counts = %d/%d, want 2/3 from the IR", m.GetComponentCount(), m.GetNetCount())
	}
}

func TestGetSheetSelectorAndFormats(t *testing.T) {
	svc := NewDesignService(fakeLoader{geom: twoSheetGeom()}, noNative{}, render.Style{})
	get := func(sel string, format webapi.SheetFormat) (*webapi.GetSheetResponse, error) {
		resp, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{
			Mount: "m", Path: "x.eds", Sheet: sel, Format: format,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	// Empty selector: the first sheet, packed by default.
	m, err := get("", webapi.SheetFormat_SHEET_FORMAT_UNSPECIFIED)
	if err != nil {
		t.Fatal(err)
	}
	if m.GetPacked().GetSheetId() != "P1" {
		t.Errorf("default selection packed sheet = %q, want P1", m.GetPacked().GetSheetId())
	}
	if len(m.GetPacked().GetPrimitives()) == 0 {
		t.Error("packed P1 carries no primitives despite a wire on the sheet")
	}

	// Selection by id beats the index fallback.
	if m, err = get("P2", webapi.SheetFormat_SHEET_FORMAT_UNSPECIFIED); err != nil || m.GetPacked().GetSheetId() != "P2" {
		t.Errorf("sheet P2 = (%q, %v), want the P2 sheet", m.GetPacked().GetSheetId(), err)
	}

	// SVG format returns a document, not a pack.
	if m, err = get("Root", webapi.SheetFormat_SHEET_FORMAT_SVG); err != nil || !strings.HasPrefix(m.GetSvg(), "<svg") {
		t.Errorf("svg render = (%.20q, %v), want an <svg document", m.GetSvg(), err)
	}

	// An out-of-range selector classifies as not-found.
	if _, err = get("9", webapi.SheetFormat_SHEET_FORMAT_UNSPECIFIED); !errors.Is(err, ErrNotFound) {
		t.Errorf("sheet 9 err = %v, want ErrNotFound", err)
	}
}

func TestGetLayoutReport(t *testing.T) {
	rep := &graph.ConversionReport{Components: []graph.ComponentReport{
		{RefDes: "R1", Symbol: "res", Class: "resistor", Cell: "res_glyph", Kind: graph.KindGlyph},
		{RefDes: "U9", Cell: "box", Kind: graph.KindBox},
	}}
	svc := NewDesignService(fakeLoader{report: rep}, noNative{}, render.Style{})
	resp, err := svc.GetLayoutReport(context.Background(), &webapi.GetLayoutReportRequest{Mount: "m", Path: "x.edn"})
	if err != nil {
		t.Fatal(err)
	}
	cs := resp.GetReport().GetComponents()
	if len(cs) != 2 {
		t.Fatalf("components = %d, want 2", len(cs))
	}
	if cs[0].GetRefDes() != "R1" || cs[0].GetDeviceClass() != "resistor" || cs[0].GetKind() != graph.KindGlyph {
		t.Errorf("R1 = %+v, want ref/class/kind carried to the wire form", cs[0])
	}
	if cs[1].GetKind() != graph.KindBox || cs[1].GetDeviceClass() != "" {
		t.Errorf("U9 = %+v, want an unclassified box", cs[1])
	}

	// A build failure (e.g. no netlist to classify) yields an empty report, not an error.
	failing := NewDesignService(fakeLoader{err: errors.New("no netlist")}, noNative{}, render.Style{})
	resp, err = failing.GetLayoutReport(context.Background(), &webapi.GetLayoutReportRequest{Mount: "m", Path: "x.eds"})
	if err != nil || len(resp.GetReport().GetComponents()) != 0 {
		t.Errorf("build failure = (%+v, %v), want an empty report and no error", resp.GetReport(), err)
	}

	// But a resolve failure (unknown mount) is still an error, not an empty report.
	missing := NewDesignService(fakeLoader{err: ErrNotFound}, noNative{}, render.Style{})
	if _, err = missing.GetLayoutReport(context.Background(), &webapi.GetLayoutReportRequest{Mount: "no", Path: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown mount err = %v, want ErrNotFound", err)
	}
}

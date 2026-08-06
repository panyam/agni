package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/readers/formats"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// pairLoader serves a distinct design (and optionally geometry) per path — the shape
// DiffDesigns needs, where the single-design fakeLoader cannot distinguish the two sides.
// A path with no geometry entry errors, exercising the annotation's nil tolerance.
type pairLoader struct {
	designs map[string]*ir.Design
	geo     map[string]*geom.SchematicGeometry
}

func (p pairLoader) Design(_ context.Context, _, path string, _ ...ReadOption) (*ir.Design, error) {
	d, ok := p.designs[path]
	if !ok {
		return nil, fmt.Errorf("no design %q: %w", path, ErrNotFound)
	}
	return d, nil
}

func (p pairLoader) Geometry(_ context.Context, _, path, _ string, _ bool) (*geom.SchematicGeometry, error) {
	g, ok := p.geo[path]
	if !ok {
		return nil, fmt.Errorf("no geometry %q: %w", path, ErrNotFound)
	}
	return g, nil
}

func conns(refs ...string) []*ir.Connection {
	out := make([]*ir.Connection, 0, len(refs))
	for _, rp := range refs {
		parts := strings.SplitN(rp, ".", 2)
		out = append(out, &ir.Connection{ComponentRef: parts[0], PinRef: parts[1]})
	}
	return out
}

// TestDiffDesigns drives the full taxonomy through the RPC in one pair: component
// added/removed/changed, net new/deleted/renamed/hard/soft, and checks both the report and
// the highlight maps (renamed nets keyed under old AND new names; unchanged entities absent).
func TestDiffDesigns(t *testing.T) {
	old := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "R1"},
			{RefDes: "R2"},
			{RefDes: "C1", Attributes: map[string]string{"Value": "100n"}},
		},
		Nets: []*ir.Net{
			{Name: "KEEP", Connections: conns("R1.1", "C1.1")},
			{Name: "OLD", Connections: conns("R2.1"), Prov: &ir.Provenance{SourceFile: "a.edn"}},
			{Name: "SIG", Connections: conns("R1.2", "C1.2")},
			{Name: "CLK", Connections: conns("R1.3")},
			{Name: "STYLE", Connections: conns("R1.4"), NetClass: "default"},
		},
	}
	newer := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "R1"},
			{RefDes: "R4"},
			{RefDes: "C1", Attributes: map[string]string{"Value": "1u"}},
		},
		Nets: []*ir.Net{
			{Name: "KEEP", Connections: conns("R1.1", "C1.1")},
			{Name: "DATA", Connections: conns("R1.2", "C1.2")},
			{Name: "CLK", Connections: conns("R1.3", "C1.3")},
			{Name: "STYLE", Connections: conns("R1.4"), NetClass: "power"},
			{Name: "FRESH", Connections: conns("R4.1", "R4.2")},
		},
	}
	svc := NewDiffService(pairLoader{designs: map[string]*ir.Design{"a.edn": old, "b.edn": newer}})
	resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{
		AMount: "m", APath: "a.edn", BMount: "m", BPath: "b.edn",
	})
	if err != nil {
		t.Fatal(err)
	}
	rep := resp.GetReport()

	if got := rep.GetComponentsAdded(); !slicesEqual(got, []string{"R4"}) {
		t.Errorf("components added = %v, want [R4]", got)
	}
	if got := rep.GetComponentsRemoved(); !slicesEqual(got, []string{"R2"}) {
		t.Errorf("components removed = %v, want [R2]", got)
	}
	if got := rep.GetComponentsChanged(); len(got) != 1 ||
		got[0].GetRefDes() != "C1" || got[0].GetField() != "Value" || got[0].GetOld() != "100n" || got[0].GetNew() != "1u" {
		t.Errorf("components changed = %v, want one C1 Value 100n->1u", got)
	}

	byName := map[string]*webapi.DiffReport_NetChange{}
	for _, nc := range rep.GetNets() {
		byName[nc.GetName()] = nc
	}
	if _, ok := byName["KEEP"]; ok {
		t.Error("unchanged net KEEP must not be reported")
	}
	if nc := byName["FRESH"]; nc.GetKind() != "new" {
		t.Errorf("FRESH kind = %q, want new", nc.GetKind())
	}
	if nc := byName["OLD"]; nc.GetKind() != "deleted" || nc.GetOldProv().GetSourceFile() != "a.edn" {
		t.Errorf("OLD = %+v, want deleted with old_prov from a.edn", nc)
	}
	if nc := byName["DATA"]; nc.GetKind() != "renamed" || nc.GetOldName() != "SIG" {
		t.Errorf("DATA = %+v, want renamed from SIG", nc)
	}
	if nc := byName["CLK"]; nc.GetKind() != "hard" || !slicesEqual(nc.GetAdded(), []string{"C1.3"}) || len(nc.GetRemoved()) != 0 {
		t.Errorf("CLK = %+v, want hard +[C1.3]", nc)
	}
	if nc := byName["STYLE"]; nc.GetKind() != "soft" {
		t.Errorf("STYLE kind = %q, want soft", nc.GetKind())
	}

	wantComp := map[string]string{"R4": "added", "R2": "removed", "C1": "changed"}
	if got := resp.GetComponentStatus(); len(got) != len(wantComp) {
		t.Errorf("component_status = %v, want %v", got, wantComp)
	} else {
		for k, v := range wantComp {
			if got[k] != v {
				t.Errorf("component_status[%s] = %q, want %q", k, got[k], v)
			}
		}
	}
	// The rename appears under BOTH names so each side joins by the name its geometry carries.
	wantNet := map[string]string{"FRESH": "new", "OLD": "deleted", "SIG": "renamed", "DATA": "renamed", "CLK": "hard", "STYLE": "soft"}
	if got := resp.GetNetStatus(); len(got) != len(wantNet) {
		t.Errorf("net_status = %v, want %v", got, wantNet)
	} else {
		for k, v := range wantNet {
			if got[k] != v {
				t.Errorf("net_status[%s] = %q, want %q", k, got[k], v)
			}
		}
	}
}

// TestDiffDesignsSheetMaps: the response's per-side sheet maps cover exactly the changed
// entities that each side's geometry locates — a removed component only in a's map, an added
// one only in b's, a changed one in both (on that side's own sheets), and a renamed net
// under its OLD name in a and its NEW name in b. Unchanged entities are absent even when
// the geometry knows them.
func TestDiffDesignsSheetMaps(t *testing.T) {
	old := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "R2"}, {RefDes: "C1", Attributes: map[string]string{"Value": "100n"}}},
		Nets: []*ir.Net{
			{Name: "KEEP", Connections: conns("R1.1", "C1.1")},
			{Name: "SIG", Connections: conns("R1.2", "C1.2")},
		},
	}
	newer := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "R4"}, {RefDes: "C1", Attributes: map[string]string{"Value": "1u"}}},
		Nets: []*ir.Net{
			{Name: "KEEP", Connections: conns("R1.1", "C1.1")},
			{Name: "SIG_CLK", Connections: conns("R1.2", "C1.2")},
		},
	}
	sheet := func(id string, refs []string, nets []string) *geom.SheetGeometry {
		sh := &geom.SheetGeometry{Id: id}
		for _, r := range refs {
			sh.Placements = append(sh.Placements, &geom.SymbolPlacement{RefDes: r})
		}
		for _, n := range nets {
			sh.Wires = append(sh.Wires, &geom.WireGeometry{Net: n})
		}
		return sh
	}
	geoA := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		sheet("a-root", []string{"R1", "C1"}, []string{"KEEP", "SIG"}),
		sheet("a-power", []string{"R2", "C1"}, []string{"SIG"}), // C1 + SIG span both sheets
	}}
	geoB := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		sheet("b-root", []string{"R1", "C1", "R4"}, []string{"KEEP", "SIG_CLK"}),
	}}
	svc := NewDiffService(pairLoader{
		designs: map[string]*ir.Design{"a.edn": old, "b.edn": newer},
		geo:     map[string]*geom.SchematicGeometry{"a.edn": geoA, "b.edn": geoB},
	})
	resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{AMount: "m", APath: "a.edn", BMount: "m", BPath: "b.edn"})
	if err != nil {
		t.Fatal(err)
	}

	ids := func(m map[string]*webapi.DiffDesignsResponse_SheetIds, key string) []string { return m[key].GetIds() }
	if got := ids(resp.GetComponentSheetsA(), "R2"); !slicesEqual(got, []string{"a-power"}) {
		t.Errorf("component_sheets_a[R2] = %v, want [a-power]", got)
	}
	if _, ok := resp.GetComponentSheetsB()["R2"]; ok {
		t.Error("removed R2 must not appear in b's sheet map")
	}
	if got := ids(resp.GetComponentSheetsB(), "R4"); !slicesEqual(got, []string{"b-root"}) {
		t.Errorf("component_sheets_b[R4] = %v, want [b-root]", got)
	}
	if got := ids(resp.GetComponentSheetsA(), "C1"); !slicesEqual(got, []string{"a-root", "a-power"}) {
		t.Errorf("component_sheets_a[C1] = %v, want its spanning sheets", got)
	}
	if got := ids(resp.GetComponentSheetsB(), "C1"); !slicesEqual(got, []string{"b-root"}) {
		t.Errorf("component_sheets_b[C1] = %v, want [b-root]", got)
	}
	if got := ids(resp.GetNetSheetsA(), "SIG"); !slicesEqual(got, []string{"a-root", "a-power"}) {
		t.Errorf("net_sheets_a[SIG] = %v, want the old name's sheets", got)
	}
	if got := ids(resp.GetNetSheetsB(), "SIG_CLK"); !slicesEqual(got, []string{"b-root"}) {
		t.Errorf("net_sheets_b[SIG_CLK] = %v, want the new name's sheets", got)
	}
	if _, ok := resp.GetNetSheetsA()["SIG_CLK"]; ok {
		t.Error("a's geometry carries the old name only; SIG_CLK must not be in a's map")
	}
	// KEEP is unchanged: known to both geometries, listed in neither map.
	if _, ok := resp.GetComponentSheetsA()["R1"]; ok {
		t.Error("unchanged R1 must not appear in the sheet maps")
	}
	if _, ok := resp.GetNetSheetsA()["KEEP"]; ok {
		t.Error("unchanged KEEP must not appear in the sheet maps")
	}
}

// TestDiffDesignsSharedPlacements: the alignment sample covers components placed in BOTH
// sides' geometry — including unchanged ones (they are the alignment evidence) — with each
// side's own sheet and position; one-sided components are excluded, and a missing geometry
// on either side yields no sample at all (frame evidence alone must then decide).
func TestDiffDesignsSharedPlacements(t *testing.T) {
	place := func(ref string, x, y int64) *geom.SymbolPlacement {
		return &geom.SymbolPlacement{RefDes: ref, Transform: &geom.Transform{Origin: &geom.Point{X: x, Y: y}}}
	}
	geoA := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		{Id: "a-root", Placements: []*geom.SymbolPlacement{place("R1", 10, 20), place("C1", 30, 40), place("R2", 50, 60)}},
	}}
	geoB := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		{Id: "b-root", Placements: []*geom.SymbolPlacement{place("R1", 10, 21), place("C1", 300, 400), place("R4", 70, 80)}},
	}}
	designs := map[string]*ir.Design{"a.edn": {Components: []*ir.Component{{RefDes: "R2"}}}, "b.edn": {Components: []*ir.Component{{RefDes: "R4"}}}}
	svc := NewDiffService(pairLoader{designs: designs, geo: map[string]*geom.SchematicGeometry{"a.edn": geoA, "b.edn": geoB}})
	resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{AMount: "m", APath: "a.edn", BMount: "m", BPath: "b.edn"})
	if err != nil {
		t.Fatal(err)
	}
	pa, pb := resp.GetSharedPlacementsA(), resp.GetSharedPlacementsB()
	if len(pa) != 2 || len(pb) != 2 {
		t.Fatalf("shared placements = %d/%d entries, want 2/2 (R1, C1)", len(pa), len(pb))
	}
	if p := pa["R1"]; p.GetSheet() != "a-root" || p.GetX() != 10 || p.GetY() != 20 {
		t.Errorf("shared_placements_a[R1] = %+v, want a-root (10,20)", p)
	}
	if p := pb["R1"]; p.GetSheet() != "b-root" || p.GetX() != 10 || p.GetY() != 21 {
		t.Errorf("shared_placements_b[R1] = %+v, want b-root (10,21)", p)
	}
	if p := pb["C1"]; p.GetX() != 300 || p.GetY() != 400 {
		t.Errorf("shared_placements_b[C1] = %+v, want (300,400)", p)
	}
	if _, ok := pa["R2"]; ok {
		t.Error("R2 exists only in a's geometry; it is not alignment evidence")
	}
	if _, ok := pb["R4"]; ok {
		t.Error("R4 exists only in b's geometry; it is not alignment evidence")
	}

	// One side without geometry: no sample, not an error.
	svc = NewDiffService(pairLoader{designs: designs, geo: map[string]*geom.SchematicGeometry{"a.edn": geoA}})
	resp, err = svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{AMount: "m", APath: "a.edn", BMount: "m", BPath: "b.edn"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetSharedPlacementsA()) != 0 || len(resp.GetSharedPlacementsB()) != 0 {
		t.Errorf("sample must be empty without both geometries, got %v / %v", resp.GetSharedPlacementsA(), resp.GetSharedPlacementsB())
	}
}

// TestDiffDesignsNoGeometry: a side without geometry (netlist-only format, load failure)
// yields empty sheet maps for that side, never an error — the maps are an annotation, not a
// requirement (mirrors the findings' nil-geometry tolerance).
func TestDiffDesignsNoGeometry(t *testing.T) {
	old := &ir.Design{Components: []*ir.Component{{RefDes: "R2"}}}
	newer := &ir.Design{}
	svc := NewDiffService(pairLoader{designs: map[string]*ir.Design{"a.edn": old, "b.edn": newer}})
	resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{AMount: "m", APath: "a.edn", BMount: "m", BPath: "b.edn"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetComponentSheetsA()) != 0 || len(resp.GetComponentSheetsB()) != 0 {
		t.Errorf("sheet maps must be empty without geometry, got a=%v b=%v", resp.GetComponentSheetsA(), resp.GetComponentSheetsB())
	}
	if resp.GetComponentStatus()["R2"] != "removed" {
		t.Errorf("status maps must still be filled, got %v", resp.GetComponentStatus())
	}
}

// TestDiffDesignsLoadErrors: either side failing to load fails the whole call with the
// loader's mapped code — there is no partial diff.
func TestDiffDesignsLoadErrors(t *testing.T) {
	svc := NewDiffService(pairLoader{designs: map[string]*ir.Design{"a.edn": {}}})
	_, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{
		AMount: "m", APath: "a.edn", BMount: "m", BPath: "missing.edn",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for the missing b side, got %v", err)
	}
	_, err = svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{
		AMount: "m", APath: "missing.edn", BMount: "m", BPath: "a.edn",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for the missing a side, got %v", err)
	}
}

// TestDiffDesignsMultiSheetLocate drives the cross-sheet click-to-locate join (WS9-027) on a
// real two-sheet KiCad diff pair (web/testdata/diffdemo/msheet_rev_{a,b}): a sub-sheet
// component changes value and a sub-sheet net renames, both on sheet "/sub". It proves the
// diff panel can resolve a sub-sheet subject to its sheet id for navigation, and pins the
// fix: the sub-sheet net's wires carry no uuid, so the geometry-only channel yields no net
// badge — only the authoritative AttrSheets membership (each side's design as NetSource)
// reaches it. Components locate through placements on either channel.
func TestDiffDesignsMultiSheetLocate(t *testing.T) {
	l := &formats.Loader{}
	load := func(rev string) (*ir.Design, *geom.SchematicGeometry) {
		p := "../../web/testdata/diffdemo/msheet_rev_" + rev + ".kicad_sch"
		d, err := l.ReadDesign(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		g, err := l.FaithfulGeometry(p)
		if err != nil {
			t.Fatalf("geometry %s: %v", p, err)
		}
		return d, g
	}
	dA, gA := load("a")
	dB, gB := load("b")

	// The join gap this ticket closes, shown directly: the sub-sheet net's wires carry no
	// uuid, so the geometry-only index (the old nil-NetSource path) has no entry for it, while
	// the design's AttrSheets membership does. Components never had the gap — placements exist.
	// The index keys nets by per-instance id (WS9), so resolve /sub/SIG's id to look it up.
	var sigKey string
	for _, n := range check.NewModel(dA).Nets() {
		if n.GetName() == "/sub/SIG" {
			sigKey = netKey(n.GetId(), n.GetName())
		}
	}
	if sigKey == "" {
		t.Fatal("precondition: design has no /sub/SIG net")
	}
	if got := indexSheets(gA, nil).nets[sigKey]; len(got) != 0 {
		t.Fatalf("precondition: geometry-only net index should miss the wireless sub-sheet net, got %v", got)
	}
	if got := indexSheets(gA, check.NewModel(dA)).nets[sigKey]; !slicesEqual(got, []string{"/sub"}) {
		t.Fatalf("AttrSheets net index[/sub/SIG] = %v, want [/sub]", got)
	}

	svc := NewDiffService(pairLoader{
		designs: map[string]*ir.Design{"a": dA, "b": dB},
		geo:     map[string]*geom.SchematicGeometry{"a": gA, "b": gB},
	})
	resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{AMount: "m", APath: "a", BMount: "m", BPath: "b"})
	if err != nil {
		t.Fatal(err)
	}

	// The diff itself: one changed sub-sheet component, one renamed sub-sheet net.
	if got := resp.GetComponentStatus(); got["R101"] != "changed" {
		t.Errorf("component_status[R101] = %q, want changed", got["R101"])
	}
	if got := resp.GetNetStatus(); got["/sub/SIG"] != "renamed" || got["/sub/OUT"] != "renamed" {
		t.Errorf("net_status = %v, want /sub/SIG and /sub/OUT both renamed", got)
	}

	ids := func(m map[string]*webapi.DiffDesignsResponse_SheetIds, key string) []string { return m[key].GetIds() }
	// Changed component: locates to the sub-sheet on both sides.
	if got := ids(resp.GetComponentSheetsA(), "R101"); !slicesEqual(got, []string{"/sub"}) {
		t.Errorf("component_sheets_a[R101] = %v, want [/sub]", got)
	}
	if got := ids(resp.GetComponentSheetsB(), "R101"); !slicesEqual(got, []string{"/sub"}) {
		t.Errorf("component_sheets_b[R101] = %v, want [/sub]", got)
	}
	// Renamed net: the old name locates on a's side, the new name on b's — the case that only
	// resolves through AttrSheets (its wires are uuid-less, so the geometry channel is empty).
	if got := ids(resp.GetNetSheetsA(), "/sub/SIG"); !slicesEqual(got, []string{"/sub"}) {
		t.Errorf("net_sheets_a[/sub/SIG] = %v, want [/sub] (requires the AttrSheets channel)", got)
	}
	if got := ids(resp.GetNetSheetsB(), "/sub/OUT"); !slicesEqual(got, []string{"/sub"}) {
		t.Errorf("net_sheets_b[/sub/OUT] = %v, want [/sub] (requires the AttrSheets channel)", got)
	}
	if _, ok := resp.GetNetSheetsA()["/sub/OUT"]; ok {
		t.Error("a's side carries the old net name only; /sub/OUT must not be in a's map")
	}
}

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// nameOnlySheet is a companion .eds in miniature. Its wires name their nets and carry no id. SIG is
// the design's one net of its name, and two of the design's nets are named DUP.
func nameOnlySheet() (*geom.SchematicGeometry, *ir.Design) {
	wire := func(net string, x int64) *geom.WireGeometry {
		return &geom.WireGeometry{Net: net, Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: x, Y: 0}, {X: x + 100, Y: 0}}}}}
	}
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{
		Id:    "P1",
		Wires: []*geom.WireGeometry{wire("SIG", 0), wire("DUP", 200), wire("DUP", 400), wire("LOOSE", 600)},
	}}}
	d := &ir.Design{Nets: []*ir.Net{{Id: "sig-1", Name: "SIG"}, {Id: "dup-1", Name: "DUP"}, {Id: "dup-2", Name: "DUP"}}}
	return g, d
}

// TestGetSheetKeysANameOnlyCanvasByNetID checks that a packed name-only sheet keys each wire with
// its net's id, so the WebGL canvas can light a finding's net from the keys alone (agni issue
// 1443). A name two nets share, and a name no net has, keep the name as their only key.
func TestGetSheetKeysANameOnlyCanvasByNetID(t *testing.T) {
	g, d := nameOnlySheet()
	svc := NewDesignService(fakeLoader{design: d, geom: g}, noNative{}, render.Style{}, nil)
	snap := snapOf(t, svc.Snapshots(), "mount://m/x.edn")
	ctx := context.Background()

	resp, err := svc.GetSheet(ctx, &webapi.GetSheetRequest{Snapshot: snap, Sheet: "P1", Format: webapi.SheetFormat_SHEET_FORMAT_PACKED})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string][]string{}
	for _, k := range resp.GetPacked().GetKeys() {
		if k.GetNet() != "" {
			ids[k.GetNet()] = append(ids[k.GetNet()], k.GetNetId())
		}
	}
	if len(ids["SIG"]) == 0 || len(ids["DUP"]) < 2 || len(ids["LOOSE"]) == 0 {
		t.Fatalf("packed keys by net = %v, want SIG, both DUP wires and LOOSE", ids)
	}
	for _, id := range ids["SIG"] {
		if id != "sig-1" {
			t.Errorf("a SIG wire is keyed %q, want sig-1", id)
		}
	}
	for _, net := range []string{"DUP", "LOOSE"} {
		for _, id := range ids[net] {
			if id != "" {
				t.Errorf("a %s wire is keyed %q, want no id", net, id)
			}
		}
	}

	resp, err = svc.GetSheet(ctx, &webapi.GetSheetRequest{Snapshot: snap, Sheet: "P1", Format: webapi.SheetFormat_SHEET_FORMAT_SVG})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.GetSvg(), `data-net-id="sig-1"`) {
		t.Error("the SVG sheet keys SIG's wire with no data-net-id")
	}
	for _, w := range g.GetSheets()[0].GetWires() {
		if w.GetNetId() != "" {
			t.Fatalf("GetSheet stamped %q onto the cached geometry's %s wire, want a copy stamped", w.GetNetId(), w.GetNet())
		}
	}
}

// TestHighlightSheetLightsANameOnlyCanvasByID checks the overlay a name-only canvas draws for a
// finding's net id. SIG's id lights SIG alone. One DUP net's id lights both DUP wires, since the
// drawing cannot say which DUP net a wire is, and nothing that lit before stops lighting.
func TestHighlightSheetLightsANameOnlyCanvasByID(t *testing.T) {
	g, d := nameOnlySheet()
	svc := NewDesignService(fakeLoader{design: d, geom: g}, noNative{}, render.Style{}, nil)
	snap := snapOf(t, svc.Snapshots(), "mount://m/x.edn")
	ctx := context.Background()

	sheet, err := svc.GetSheet(ctx, &webapi.GetSheetRequest{Snapshot: snap, Sheet: "P1", Format: webapi.SheetFormat_SHEET_FORMAT_PACKED})
	if err != nil {
		t.Fatal(err)
	}
	netOf := map[uint32]string{}
	for _, k := range sheet.GetPacked().GetKeys() {
		netOf[k.GetPrimitive()] = k.GetNet()
	}
	lit := func(id string) map[string]int {
		resp, err := svc.HighlightSheet(ctx, &webapi.HighlightSheetRequest{
			Snapshot: snap, Sheet: "P1", Format: webapi.SheetFormat_SHEET_FORMAT_PACKED,
			Specs: []*geom.HighlightSpec{{NetIds: []string{id}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int{}
		for _, grp := range resp.GetPacked().GetGroups() {
			for _, p := range grp.GetPrimitives() {
				got[netOf[p]]++
			}
		}
		return got
	}
	if got := lit("sig-1"); len(got) != 1 || got["SIG"] == 0 {
		t.Errorf("sig-1 lit %v, want SIG alone", got)
	}
	if got := lit("dup-1"); len(got) != 1 || got["DUP"] < 2 {
		t.Errorf("dup-1 lit %v, want both DUP wires", got)
	}
}

// TestSheetWithIDsIsNotRestamped checks that a sheet whose wires carry ids already, as KiCad's do,
// comes back as the same sheet, so its per-instance ids are never replaced by a name join.
func TestSheetWithIDsIsNotRestamped(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Wires: []*geom.WireGeometry{{Net: "/SIG", NetId: "kicad-1"}}}}}
	d := &ir.Design{Nets: []*ir.Net{{Id: "other", Name: "/SIG"}}}
	svc := NewDesignService(fakeLoader{design: d, geom: g}, noNative{}, render.Style{}, nil)
	got := svc.withWireNetIDs(context.Background(), Snap{}, g, g.GetSheets()[0])
	if got != g.GetSheets()[0] || got.GetWires()[0].GetNetId() != "kicad-1" {
		t.Errorf("an id-capable sheet was restamped: %v", got.GetWires())
	}
}

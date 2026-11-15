package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/panyam/agni/core/render"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// viewsFixture is a netlist and its board. The board names VIN "Net-(R1-Pad1)" with the same pins,
// leaves U1 off, and places a mounting hole H1 the netlist lacks.
func viewsFixture() (*ir.Design, *geom.BoardGeometry) {
	conn := func(ref, pin string) *ir.Connection { return &ir.Connection{ComponentRef: ref, PinRef: pin} }
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "R2"}, {RefDes: "U1"}},
		Nets: []*ir.Net{
			{Id: "vin", Name: "VIN", Connections: []*ir.Connection{conn("R1", "1"), conn("R2", "1")}},
			{Id: "gnd", Name: "GND", Connections: []*ir.Connection{conn("R1", "2"), conn("R2", "2")}},
			{Id: "en", Name: "EN", Connections: []*ir.Connection{conn("U1", "1")}},
		},
	}
	pad := func(n, net string) *geom.Pad {
		return &geom.Pad{Number: n, Net: net, At: &geom.Point{}, Layers: []string{"F.Cu"}}
	}
	b := &geom.BoardGeometry{
		Placements: []*geom.ComponentPlacement{
			{RefDes: "R1", At: &geom.Point{}, Layer: "F.Cu", Pads: []*geom.Pad{pad("1", "Net-(R1-Pad1)"), pad("2", "GND")}},
			{RefDes: "R2", At: &geom.Point{}, Layer: "F.Cu", Pads: []*geom.Pad{pad("1", "Net-(R1-Pad1)"), pad("2", "GND")}},
			{RefDes: "H1", At: &geom.Point{}, Layer: "F.Cu", Pads: []*geom.Pad{pad("1", "")}},
		},
		Layers: []*geom.BoardLayer{{Name: "F.Cu", Kind: "signal"}},
		Nets:   []*geom.NetCopper{{Net: "Net-(R1-Pad1)", Segments: []*geom.TrackSegment{{A: &geom.Point{}, B: &geom.Point{X: 1}, Layer: "F.Cu", Width: 1}}}},
	}
	return d, b
}

// viewsSnap is the fixture's design with its board declared beside the netlist.
func viewsSnap(t *testing.T, ss *SnapshotService) string {
	return ingestWith(t, ss, &webapi.Snapshot{Uri: "mount://m/x.edn", BoardUri: "mount://m/x.kicad_pcb", AsNamed: true})
}

func TestCompareViewsCountsWhatBothViewsHold(t *testing.T) {
	d, b := viewsFixture()
	svc := NewDiffService(fakeLoader{design: d, board: b}, nil)
	resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{ViewsSnapshot: viewsSnap(t, svc.Snapshots())})
	if err != nil {
		t.Fatal(err)
	}
	v := resp.GetViews()
	if !v.GetCompared() {
		t.Fatalf("not compared: %s", v.GetNotCompared())
	}
	type tally struct{ m, s, b int32 }
	of := func(x *webapi.ViewAgreement_Tally) tally {
		return tally{x.GetMatched(), x.GetSchematic(), x.GetBoard()}
	}
	if got := of(v.GetParts()); got != (tally{2, 3, 3}) {
		t.Errorf("parts = %+v, want 2 of 3 schematic parts and 3 board parts", got)
	}
	if got := of(v.GetNets()); got != (tally{2, 3, 2}) {
		t.Errorf("nets = %+v, want VIN by its pins and GND by name, of 3 schematic nets and 2 board nets", got)
	}
	if got := of(v.GetPins()); got != (tally{4, 4, 4}) {
		t.Errorf("pins = %+v, want R1's and R2's 4", got)
	}
	rep := resp.GetReport()
	if !slices.Equal(rep.GetComponentsRemoved(), []string{"U1"}) || !slices.Equal(rep.GetComponentsAdded(), []string{"H1"}) {
		t.Errorf("schematic-only %v, board-only %v; want [U1] and [H1]", rep.GetComponentsRemoved(), rep.GetComponentsAdded())
	}
	var kinds []string
	for _, nc := range rep.GetNets() {
		kinds = append(kinds, nc.GetKind()+" "+nc.GetOldName()+">"+nc.GetName())
	}
	if want := []string{"deleted >EN", "renamed VIN>Net-(R1-Pad1)"}; !slices.Equal(kinds, want) {
		t.Errorf("net changes = %v, want %v", kinds, want)
	}
}

func TestCompareViewsSaysWhyItDidNotCompare(t *testing.T) {
	d, b := viewsFixture()
	for _, c := range []struct {
		name   string
		loader fakeLoader
		def    *webapi.Snapshot
		want   string
	}{
		{"no board", fakeLoader{design: d}, &webapi.Snapshot{Uri: "mount://m/x.edn"},
			"the design has no board to compare its schematic with"},
		{"the board alone", fakeLoader{design: d, board: b}, &webapi.Snapshot{Uri: "mount://m/x.kicad_pcb"},
			"the design is its board alone, with no schematic to compare it with"},
		{"an unreadable netlist", fakeLoader{err: errors.New("broken")}, &webapi.Snapshot{Uri: "mount://m/x.edn", BoardUri: "mount://m/x.kicad_pcb", AsNamed: true},
			"the schematic did not read: invalid argument: broken"},
	} {
		svc := NewDiffService(c.loader, nil)
		resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{ViewsSnapshot: ingestWith(t, svc.Snapshots(), c.def)})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		v := resp.GetViews()
		if v.GetCompared() || v.GetNotCompared() != c.want || v.GetParts().GetSchematic() != 0 || len(resp.GetReport().GetNets()) != 0 {
			t.Errorf("%s: views = %v, report = %v; want not compared, %q, and nothing counted", c.name, v, resp.GetReport(), c.want)
		}
	}
}

func TestCompareViewsTakesOneSnapshot(t *testing.T) {
	d, b := viewsFixture()
	svc := NewDiffService(fakeLoader{design: d, board: b}, nil)
	s := viewsSnap(t, svc.Snapshots())
	_, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{ViewsSnapshot: s, ASnapshot: s})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("views_snapshot with a_snapshot answered %v, want invalid argument", err)
	}
}

// TestCrossProbeFollowsANetTheBoardNamesDifferently holds the cross-probe to the agreement's
// pairing. The board names VIN "Net-(R1-Pad1)". Its drawing keys the net as VIN with VIN's id, and
// locating VIN on the board finds it.
func TestCrossProbeFollowsANetTheBoardNamesDifferently(t *testing.T) {
	d, b := viewsFixture()
	svc := NewDesignService(fakeLoader{design: d, board: b}, noNative{}, render.Style{}, nil)
	snap := viewsSnap(t, svc.Snapshots())
	ctx := context.Background()

	resp, err := svc.GetSheet(ctx, &webapi.GetSheetRequest{Snapshot: snap, Sheet: "board", Format: webapi.SheetFormat_SHEET_FORMAT_PACKED})
	if err != nil {
		t.Fatal(err)
	}
	keyed := map[string]string{}
	for _, k := range render.PackedKeys(resp.GetPacked()) {
		if k.GetNet() != "" {
			keyed[k.GetNet()] = k.GetNetId()
		}
	}
	if _, old := keyed["Net-(R1-Pad1)"]; keyed["VIN"] != "vin" || old {
		t.Errorf("the board keys its copper %v, want VIN as vin and no Net-(R1-Pad1)", keyed)
	}

	loc, err := svc.LocateSubjects(ctx, &webapi.LocateSubjectsRequest{
		Snapshot: snap, Board: true, Subjects: []*checkspb.Subject{{Kind: "net", Ref: "VIN"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := loc.GetSubjects()[0]
	if !slices.Contains(got.GetSheetIds(), "board") || !slices.Equal(got.GetBoardLayers(), []string{"F.Cu"}) {
		t.Errorf("VIN locates on %v with layers %v, want the board and F.Cu", got.GetSheetIds(), got.GetBoardLayers())
	}
}

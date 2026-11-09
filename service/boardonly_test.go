package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/render"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// geomCountingLoader counts the auto-layout reads, which a board-only design must not make.
type geomCountingLoader struct {
	fakeLoader
	reads *int
}

func (l geomCountingLoader) Geometry(ctx context.Context, u artifact.URI, layout string, faithful bool, opts ...ReadOption) (*geom.SchematicGeometry, error) {
	*l.reads++
	return l.fakeLoader.Geometry(ctx, u, layout, faithful, opts...)
}

// A board opened on its own lists the board alone, with no auto-layout sheet and no layout to pick
// (agni issue 1466). It never lays its netlist out.
func TestABoardOnlyDesignListsItsBoardAlone(t *testing.T) {
	d, g, b := locateFixture()
	for _, uri := range []string{"mount://m/x.brd", "mount://m/x.PcbDoc", "mount://m/x.xml", "mount://m/x.tgz", "mount://m/x.kicad_pcb"} {
		reads := 0
		svc := NewDesignService(geomCountingLoader{fakeLoader{design: d, geom: g, board: b}, &reads}, noNative{}, render.DefaultStyle, nil)
		resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Snapshot: snapOf(t, svc.Snapshots(), uri)})
		if err != nil {
			t.Fatalf("%s: %v", uri, err)
		}
		var ids []string
		for _, s := range resp.GetSheets() {
			ids = append(ids, s.GetId())
		}
		if !slicesEqual(ids, []string{boardSheetID}) {
			t.Errorf("%s lists sheets %v, want [board]", uri, ids)
		}
		if resp.GetLayout() != "" || len(resp.GetAvailableLayouts()) != 0 || resp.GetDrawingNote() != "" {
			t.Errorf("%s offers layout %q of %v with note %q, want none", uri, resp.GetLayout(), resp.GetAvailableLayouts(), resp.GetDrawingNote())
		}
		if reads != 0 {
			t.Errorf("%s laid its netlist out %d times to list the board", uri, reads)
		}
		if resp.GetComponentCount() != 2 {
			t.Errorf("%s counts %d components, want the netlist's 2", uri, resp.GetComponentCount())
		}
	}
}

// A netlist with no board keeps its auto-layout, since that is the only drawing it has.
func TestANetlistWithNoBoardKeepsItsAutoLayout(t *testing.T) {
	d, g, _ := locateFixture()
	svc := NewDesignService(fakeLoader{design: d, geom: g}, noNative{}, render.DefaultStyle, nil)
	resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.edn")})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range resp.GetSheets() {
		ids = append(ids, s.GetId())
	}
	if !slicesEqual(ids, []string{"S1", "S2"}) {
		t.Errorf("x.edn lists sheets %v, want its auto-layout [S1 S2]", ids)
	}
	if resp.GetLayout() == "" || len(resp.GetAvailableLayouts()) == 0 {
		t.Errorf("x.edn offers layout %q of %v, want an auto-layout to pick", resp.GetLayout(), resp.GetAvailableLayouts())
	}
}

// Every answer about a board-only design locates on its board, since that is the only sheet it
// lists. Before agni issue 1466 they named the auto-layout's sheets.
func TestABoardOnlyDesignLocatesOnItsBoard(t *testing.T) {
	d, g, b := locateFixture()
	ld := fakeLoader{design: d, geom: g, board: b}
	ctx := context.Background()
	const uri = "mount://m/x.brd"
	board := []string{boardSheetID}

	designs := NewDesignService(ld, noNative{}, render.DefaultStyle, nil)
	loc, err := designs.LocateSubjects(ctx, &webapi.LocateSubjectsRequest{
		Snapshot: snapOf(t, designs.Snapshots(), uri), Subjects: []*checkspb.Subject{part("U1"), netNamed("N1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range loc.GetSubjects() {
		if !slicesEqual(s.GetSheetIds(), board) {
			t.Errorf("LocateSubjects puts %v on %v, want [board]", s.GetSubject(), s.GetSheetIds())
		}
	}

	checks := NewCheckService(ld, check.DefaultCatalog(), nil, "", nil, nil)
	run, err := checks.CheckDesign(ctx, &webapi.CheckDesignRequest{Snapshot: snapOf(t, checks.Snapshots(), uri)})
	if err != nil {
		t.Fatal(err)
	}
	located := 0
	for _, v := range run.GetVerdicts() {
		if len(v.GetSheets()) == 0 {
			continue
		}
		located++
		if !slicesEqual(v.GetSheets(), board) {
			t.Errorf("verdict %s badges %v, want [board]", v.GetId(), v.GetSheets())
		}
	}
	if located == 0 {
		t.Fatal("no verdict of the run was located, so the test checks nothing")
	}

	tr, err := designs.TraceDesign(ctx, &webapi.TraceDesignRequest{
		Snapshot: snapOf(t, designs.Snapshots(), uri),
		From:     &webapi.TraceEndpoint{RefDes: "U1", Pin: "1"},
		To:       &webapi.TraceEndpoint{RefDes: "R1", Pin: "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.GetTrace().GetNets()) == 0 {
		t.Fatal("the trace crossed no net, so the test checks nothing")
	}
	for _, n := range tr.GetTrace().GetNets() {
		if !slicesEqual(n.GetSheetIds(), board) {
			t.Errorf("trace net %s is on %v, want [board]", n.GetName(), n.GetSheetIds())
		}
	}

	walk, err := designs.WalkDesign(ctx, &webapi.WalkDesignRequest{
		Snapshot: snapOf(t, designs.Snapshots(), uri),
		From:     &webapi.TraceEndpoint{RefDes: "U1", Pin: "1"},
		Kind:     "all",
		Hops:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(walk.GetWalk().GetNets()) == 0 {
		t.Fatal("the walk reached no net, so the test checks nothing")
	}
	for _, n := range walk.GetWalk().GetNets() {
		if !slicesEqual(n.GetNet().GetSheetIds(), board) {
			t.Errorf("walk net %s is on %v, want [board]", n.GetNet().GetName(), n.GetNet().GetSheetIds())
		}
	}

	queries := NewQueryService(ld, nil, nil)
	ans, err := queries.RunQuery(ctx, &webapi.RunQueryRequest{
		Snapshot: snapOf(t, queries.Snapshots(), uri),
		Query:    `entity(?name, ?kind), ?kind = "component"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ans.GetRows()) != 2 {
		t.Fatalf("the query answered %d components, want 2", len(ans.GetRows()))
	}
	for _, row := range ans.GetRows() {
		if sheets, _ := CellLocation(ans, row, 0); !slicesEqual(sheets, board) {
			t.Errorf("query cell %s is on %v, want [board]", row.GetCells()[0], sheets)
		}
	}
}

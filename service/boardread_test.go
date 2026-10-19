package service

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// boardCountingLoader counts every time a request asks for the design's board.
type boardCountingLoader struct {
	fakeLoader
	boards *atomic.Int32
}

func (l boardCountingLoader) Board(ctx context.Context, u artifact.URI) (*geom.BoardGeometry, error) {
	l.boards.Add(1)
	return l.fakeLoader.Board(ctx, u)
}

// routedDesign is one net, THIN, joining two pins, with a board routing it on F.Cu.
func routedDesign() (boardCountingLoader, *atomic.Int32) {
	n := &atomic.Int32{}
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1"}},
		Nets: []*ir.Net{{Name: "THIN", Connections: []*ir.Connection{
			{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "U1", PinRef: "2"},
		}}},
	}
	return boardCountingLoader{fakeLoader: fakeLoader{design: d, board: thinBoard()}, boards: n}, n
}

func runOn(t *testing.T, l boardCountingLoader, q string, overlay *webapi.OverlayConfig) *webapi.RunQueryResponse {
	t.Helper()
	svc := NewQueryService(l, nil, nil)
	snap := ingestWith(t, svc.Snapshots(), &webapi.Snapshot{Uri: "mount://m/x.kicad_sch", Overlay: overlay})
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Snapshot: snap, Query: q})
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return resp
}

// A query that asks nothing of the copper is answered without reading the board, which is most of
// what a large design's model holds (agni issue 987).
func TestAQueryNamingNoBoardRelationReadsNoBoard(t *testing.T) {
	l, boards := routedDesign()
	resp := runOn(t, l, "component.net(?r, ?n) => ?r, ?n", nil)
	if len(resp.GetRows()) == 0 {
		t.Fatal("the query answered nothing, so it proves nothing about the board")
	}
	if n := boards.Load(); n != 0 {
		t.Errorf("a query naming no board relation asked for the board %d times", n)
	}
}

// Each way a query can reach a board relation reads the board: naming it, a rule of its own that
// names it, and a library member that does.
func TestAQueryReachingABoardRelationReadsTheBoard(t *testing.T) {
	lib := inlineReq(&webapi.LibraryModule{Path: "house", Text: "routed(?n: net) :- board.layer(?n, ?l);"})
	for _, tc := range []struct {
		name, q string
		overlay *webapi.OverlayConfig
	}{
		{"named", "board.layer(?n, ?l) => ?n, ?l", nil},
		{"own rule", "routed(?n) :- board.layer(?n, ?l); routed(?n) => ?n", nil},
		{"library member", "house.routed(?n) => ?n", lib},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, boards := routedDesign()
			resp := runOn(t, l, tc.q, tc.overlay)
			if boards.Load() == 0 {
				t.Error("the query reached a board relation and the board was never read")
			}
			if len(resp.GetRows()) != 1 || resp.GetRows()[0].GetCells()[0] != "THIN" {
				t.Errorf("rows = %v, want THIN, the one routed net", resp.GetRows())
			}
		})
	}
}

// A set reads the board once for all its queries when any of them needs it.
func TestAQuerySetReadsTheBoardWhenAnyQueryNeedsIt(t *testing.T) {
	for _, tc := range []struct {
		name  string
		qs    []string
		reads bool
	}{
		{"none", []string{"component.net(?r, ?n) => ?r", "component.net(?r, ?n) => ?n"}, false},
		{"one", []string{"component.net(?r, ?n) => ?r", "board.layer(?n, ?l) => ?n"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, boards := routedDesign()
			set := &webapi.QuerySet{}
			for i, q := range tc.qs {
				set.Queries = append(set.Queries, &webapi.NamedQuery{Name: string(rune('a' + i)), Query: q})
			}
			svc := NewQueryService(l, nil, nil)
			resp, err := svc.RunQueries(context.Background(), &webapi.RunQueriesRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.kicad_sch"), Set: set})
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range resp.GetResults() {
				if r.GetError() != "" || len(r.GetResult().GetRows()) == 0 {
					t.Fatalf("%s answered %q with %d rows", r.GetName(), r.GetError(), len(r.GetResult().GetRows()))
				}
			}
			if got := boards.Load() > 0; got != tc.reads {
				t.Errorf("read the board: %v, want %v", got, tc.reads)
			}
		})
	}
}

// Interface coverage and a part's parameters read no board fact, so neither reads the board.
func TestCoverageAndPartParametersReadNoBoard(t *testing.T) {
	l, boards := routedDesign()
	svc := NewCheckService(l, check.DefaultCatalog(), nil, "", nil, nil)
	snap := snapOf(t, svc.Snapshots(), "mount://m/x.kicad_sch")
	if _, err := svc.GetInterfaceCoverage(context.Background(), &webapi.GetInterfaceCoverageRequest{Snapshot: snap}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetComponentParams(context.Background(), &webapi.GetComponentParamsRequest{Snapshot: snap}); err != nil {
		t.Fatal(err)
	}
	if n := boards.Load(); n != 0 {
		t.Errorf("coverage and parameters asked for the board %d times", n)
	}
}

// A check run still reads the board, since its board-tier rules need it.
func TestACheckRunStillReadsTheBoard(t *testing.T) {
	l, boards := routedDesign()
	svc := NewCheckService(l, check.DefaultCatalog(), nil, "", nil, nil)
	if _, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.kicad_sch")}); err != nil {
		t.Fatal(err)
	}
	if boards.Load() == 0 {
		t.Error("a check run never read the board, so every board-tier rule ran over no copper")
	}
}

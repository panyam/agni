package service

import (
	"context"
	"errors"
	"testing"

	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/readers/formats"
	"google.golang.org/protobuf/proto"
)

// jetsonCutBoard serves the Jetson cut's real board behind a schematic snapshot.
func jetsonCutBoard(t *testing.T) (*DesignViewService, string) {
	t.Helper()
	b, err := (&formats.Loader{}).BoardGeometry("../examples/cross-probe-project/designs/jetson-cut/jetson-cut.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.GetPlacements()) == 0 || len(b.GetNets()) == 0 || len(b.GetTexts()) == 0 || len(b.GetGraphics()) == 0 {
		t.Fatalf("the cut has %d placements, %d nets, %d texts and %d graphics; the paging tests need some of each",
			len(b.GetPlacements()), len(b.GetNets()), len(b.GetTexts()), len(b.GetGraphics()))
	}
	ld := boardLoader{fakeLoader: fakeLoader{geom: &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "graph"}}}}, board: b}
	svc := NewDesignViewService(ld, noNative{}, render.DefaultStyle, nil)
	return svc, schematicWithBoard(t, svc)
}

// allPages pages through the board at budget, and returns the pages.
func allPages(t *testing.T, svc *DesignViewService, snap string, budget int64) []*geom.BoardGeometry {
	t.Helper()
	var pages []*geom.BoardGeometry
	token := ""
	for {
		resp, err := svc.GetBoardGeometry(context.Background(), &webapi.GetBoardGeometryRequest{Snapshot: snap, MaxBytes: budget, PageToken: token})
		if err != nil {
			t.Fatalf("page %d at %d bytes: %v", len(pages), budget, err)
		}
		pages = append(pages, resp.GetBoard())
		if token = resp.GetNextPageToken(); token == "" {
			return pages
		}
		if len(pages) > 10_000 {
			t.Fatalf("paging at %d bytes did not end", budget)
		}
	}
}

func elementsOn(p *geom.BoardGeometry) int {
	return len(p.GetPlacements()) + len(p.GetNets()) + len(p.GetZones()) + len(p.GetTexts()) + len(p.GetGraphics())
}

func TestTheCutsBoardComesInOnePageByDefault(t *testing.T) {
	svc, snap := jetsonCutBoard(t)
	pages := allPages(t, svc, snap, 0)
	if len(pages) != 1 {
		t.Fatalf("the 51 KB cut came in %d pages at the 1 MiB default, want 1", len(pages))
	}
	if pages[0].GetUnitNm() != 1 || len(pages[0].GetLayers()) == 0 || pages[0].GetOutline() == nil {
		t.Errorf("the page has unit_nm %d, %d layers and outline %v, want the header", pages[0].GetUnitNm(), len(pages[0].GetLayers()), pages[0].GetOutline())
	}
}

func TestBoardPagesMergeBackIntoTheWholeBoard(t *testing.T) {
	svc, snap := jetsonCutBoard(t)
	// The KiCad reader writes a back-side pad's 0° as -0, and proto.Merge writes a proto3 -0 as 0. So
	// the whole board goes through the same merge as the pages before they compare.
	whole := &geom.BoardGeometry{}
	proto.Merge(whole, allPages(t, svc, snap, 0)[0])
	for _, budget := range []int64{8_000, 1_000, 1} {
		pages := allPages(t, svc, snap, budget)
		if len(pages) < 2 {
			t.Fatalf("at %d bytes the board came in %d page, want several", budget, len(pages))
		}
		merged := &geom.BoardGeometry{}
		for i, p := range pages {
			n := elementsOn(p)
			if i > 0 && n == 0 {
				t.Errorf("at %d bytes page %d holds no element", budget, i)
			}
			if size := proto.Size(p); size > int(budget) && n > 1 {
				t.Errorf("at %d bytes page %d is %d bytes with %d elements, want under budget or one element alone", budget, i, size, n)
			}
			if i > 0 && (p.GetUnitNm() != 0 || len(p.GetLayers()) != 0) {
				t.Errorf("at %d bytes page %d repeats the header", budget, i)
			}
			proto.Merge(merged, p)
		}
		if !proto.Equal(merged, whole) {
			t.Errorf("at %d bytes the %d pages merge to a different board than the whole one", budget, len(pages))
		}
	}
}

func TestABoardPageTokenIsRefusedOutsideItsRevision(t *testing.T) {
	svc, snap := jetsonCutBoard(t)
	for name, token := range map[string]string{
		"another revision": boardCursor{header: true, section: 1}.token("not-this-one"),
		"not a token":      "!!",
		"past the lists":   boardCursor{header: true, section: len(boardSections)}.token(""),
	} {
		_, err := svc.GetBoardGeometry(context.Background(), &webapi.GetBoardGeometryRequest{Snapshot: snap, PageToken: token})
		if !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want invalid_argument", name, err)
		}
	}
	if _, err := svc.GetBoardGeometry(context.Background(), &webapi.GetBoardGeometryRequest{Snapshot: snap, MaxBytes: -1}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("max_bytes -1: err = %v, want invalid_argument", err)
	}
}

func TestADesignWithNoBoardHasNoBoardGeometry(t *testing.T) {
	d, g, _ := locateFixture()
	svc := NewDesignViewService(fakeLoader{design: d, geom: g}, noNative{}, render.DefaultStyle, nil)
	_, err := svc.GetBoardGeometry(context.Background(), &webapi.GetBoardGeometryRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.edn")})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want not_found for a netlist with no board", err)
	}
}

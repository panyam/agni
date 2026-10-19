package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// boardOnlyAt answers a board for one URI alone, as a loader reading exactly what it is handed does:
// the folder and the netlist entry carry none, and the declared companion does.
type boardOnlyAt struct {
	fakeLoader
	at    string
	board *geom.BoardGeometry
}

func (b boardOnlyAt) Board(_ context.Context, u artifact.URI) (*geom.BoardGeometry, error) {
	if u.String() == b.at {
		return b.board, nil
	}
	return nil, nil
}

// TestTheBoardSheetGetDesignListsOpens covers agni issue 865. GetDesign lists a Board sheet from the
// design's resolved board tier, so the sheet and its highlight must read the board there too, for
// every spelling of the design, rather than at the URI the request named.
func TestTheBoardSheetGetDesignListsOpens(t *testing.T) {
	design := &webapi.Design{
		Uri:           "mount://m/d",
		EntryUri:      "mount://m/d/gateway.edn",
		CompanionUris: []string{"mount://m/d/gateway.kicad_pcb"},
	}
	ld := boardOnlyAt{
		fakeLoader: fakeLoader{geom: &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "graph"}}}},
		at:         "mount://m/d/gateway.kicad_pcb",
		board:      testBoard(),
	}
	svc := NewDesignService(ld, noNative{}, render.DefaultStyle, &ProjectResolver{Store: declaringStore{design: design}})
	ctx := context.Background()
	for _, uri := range []string{"mount://m/d", "mount://m/d/gateway.edn", "mount://m/d/gateway.kicad_pcb"} {
		snap := snapOf(t, svc.Snapshots(), uri)
		d, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Snapshot: snap})
		if err != nil {
			t.Fatalf("%s: GetDesign: %v", uri, err)
		}
		listed := false
		for _, s := range d.GetSheets() {
			listed = listed || s.GetId() == boardSheetID
		}
		if !listed {
			t.Fatalf("%s: GetDesign lists no board sheet, so opening one proves nothing", uri)
		}
		if _, err := svc.GetSheet(ctx, &webapi.GetSheetRequest{Snapshot: snap, Sheet: boardSheetID}); err != nil {
			t.Errorf("%s: GetSheet board: %v", uri, err)
		}
		if _, err := svc.HighlightSheet(ctx, &webapi.HighlightSheetRequest{Snapshot: snap, Sheet: boardSheetID, Specs: []*geom.HighlightSpec{{Nets: []string{"SIG"}}}}); err != nil {
			t.Errorf("%s: HighlightSheet board: %v", uri, err)
		}
	}
}

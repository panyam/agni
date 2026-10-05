package wasmengine

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

func gatewayQuery(t *testing.T, req *webapi.RunQueryRequest) *webapi.RunQueryResponse {
	t.Helper()
	dir := filepath.Join("..", "..", "examples", "tutorial-project", "designs", "gateway")
	eng, err := New(fshost.Mount{Name: "m", FS: fshost.MemFS(readTree(t, dir))})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(eng.Handler)
	t.Cleanup(srv.Close)
	req.Uri = "mount://m"
	r, err := webapiconnect.NewQueryServiceClient(srv.Client(), srv.URL).RunQuery(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	return r.Msg
}

// TestAnAnswersTablesSayWhatItsRowsSay holds the normalized answer (agni issue 916) to the per-row
// fields it replaces: every entity cell's entry carries that cell's sheets and reason, every row's
// citations are its source indices, and an entity named on many rows is kept once.
func TestAnAnswersTablesSayWhatItsRowsSay(t *testing.T) {
	r := gatewayQuery(t, &webapi.RunQueryRequest{Query: `pin.net(?r, ?p, ?n) => ?n, ?r, ?p`})
	if len(r.GetRows()) == 0 || len(r.GetEntities()) == 0 {
		t.Fatalf("%d rows and %d entities, so nothing below is checked", len(r.GetRows()), len(r.GetEntities()))
	}
	located, cells := 0, 0
	for ri, row := range r.GetRows() {
		if len(row.GetCellEntity()) != len(row.GetCells()) {
			t.Fatalf("row %d: %d cell_entity for %d cells", ri, len(row.GetCellEntity()), len(row.GetCells()))
		}
		for i, n := range row.GetCellEntity() {
			if n == 0 {
				continue
			}
			cells++
			e := r.GetEntities()[n-1]
			if !slices.Equal(e.GetSheetIds(), row.GetCellSheets()[i].GetSheetIds()) || e.GetReason() != row.GetCellReasons()[i] {
				t.Fatalf("row %d cell %d: entity %v disagrees with the row's sheets %v and reason %v", ri, i, e, row.GetCellSheets()[i].GetSheetIds(), row.GetCellReasons()[i])
			}
			if len(e.GetSheetIds()) > 0 {
				located++
			}
		}
		var cites []string
		for _, ci := range row.GetCiteIndex() {
			cites = append(cites, r.GetSources()[ci])
		}
		if !slices.Equal(cites, row.GetCites()) {
			t.Fatalf("row %d: sources %v, cites %v", ri, cites, row.GetCites())
		}
	}
	if located == 0 {
		t.Fatal("no entity cell is located on a sheet, so the sheets were never compared")
	}
	if len(r.GetEntities()) >= cells {
		t.Errorf("%d entities for %d entity cells: an entity named on several rows was kept more than once", len(r.GetEntities()), cells)
	}
}

// TestOmittingLocationsKeepsTheEntitiesUnlocated: the table still names each entity, so a client
// can type and link a cell, but carries no sheets.
func TestOmittingLocationsKeepsTheEntitiesUnlocated(t *testing.T) {
	r := gatewayQuery(t, &webapi.RunQueryRequest{Query: `pin.net(?r, ?p, ?n) => ?n, ?r, ?p`, OmitLocations: true})
	if len(r.GetEntities()) == 0 {
		t.Fatal("no entities")
	}
	for _, e := range r.GetEntities() {
		if len(e.GetSheetIds()) > 0 {
			t.Fatalf("entity %v carries sheets though locations were omitted", e)
		}
	}
}

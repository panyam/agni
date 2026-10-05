package wasmengine

import (
	"context"
	"net/http/httptest"
	"path/filepath"
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

// TestAnAnswerKeepsEachEntityAndSourceOnce holds the normalized answer (agni issue 916): every entity
// cell points at an entry naming that cell's entity, every citation index resolves, an entity named
// on many rows is kept once, and the answer still locates its cells.
func TestAnAnswerKeepsEachEntityAndSourceOnce(t *testing.T) {
	r := gatewayQuery(t, &webapi.RunQueryRequest{Query: `pin.net(?r, ?p, ?n) => ?n, ?r, ?p`})
	if len(r.GetRows()) == 0 || len(r.GetEntities()) == 0 || len(r.GetSources()) == 0 {
		t.Fatalf("%d rows, %d entities, %d sources, so nothing below is checked", len(r.GetRows()), len(r.GetEntities()), len(r.GetSources()))
	}
	kinds := r.GetColumnKinds()
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
			want := row.GetCells()[i]
			if kinds[i] == "pin" {
				if e.GetPin() != want || e.GetRef() != row.GetCellRefs()[i] {
					t.Fatalf("row %d cell %d: entity %v, want pin %q of %q", ri, i, e, want, row.GetCellRefs()[i])
				}
			} else if e.GetRef() != want || e.GetKind() != kinds[i] {
				t.Fatalf("row %d cell %d: entity %v, want %s %q", ri, i, e, kinds[i], want)
			}
			if len(e.GetSheetIds()) > 0 {
				located++
			}
		}
		if len(row.GetCiteIndex()) == 0 {
			t.Fatalf("row %d carries no citation", ri)
		}
		for _, ci := range row.GetCiteIndex() {
			if int(ci) >= len(r.GetSources()) {
				t.Fatalf("row %d: cite index %d past %d sources", ri, ci, len(r.GetSources()))
			}
		}
	}
	if located == 0 {
		t.Fatal("no entity is located on a sheet, so the answer lost its locations")
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

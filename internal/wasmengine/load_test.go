package wasmengine

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/goapplib/wasmhost"
)

// The tests here cover Ingest with read_board (agni issue 1061). The page sends it from a worker that
// ends when it answers. The worker that later runs the checks then restores the board instead of
// parsing it.

// ingestEngine sends req to eng over Connect and returns the snapshot it made.
func ingestEngine(t *testing.T, eng *Engine, req *webapi.IngestRequest) *webapi.Snapshot {
	t.Helper()
	srv := httptest.NewServer(eng.Handler)
	defer srv.Close()
	resp, err := webapiconnect.NewSnapshotServiceClient(srv.Client(), srv.URL).Ingest(context.Background(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg
}

// snapAt is the gateway's snapshot on the engine srv serves.
func snapAt(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	return ingestAt(t, srv.Client(), srv.URL, &webapi.Snapshot{Uri: "mount://m"})
}

func loadDesign(t *testing.T, eng *Engine) *webapi.Snapshot {
	t.Helper()
	return ingestEngine(t, eng, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m"}, ReadBoard: true})
}

// TestABoardLoadedInOneWorkerIsRestoredByTheNext runs the page's two-worker shape. The prepare worker
// loads the design and ends. A fresh worker over the same store ingests it and runs the checks. As a
// control, a fresh worker over an empty store parses the board itself.
func TestABoardLoadedInOneWorkerIsRestoredByTheNext(t *testing.T) {
	store := &wasmhost.MemCache{}
	got := loadDesign(t, gatewayEngine(t, store, "v1", nil)).GetTiers()
	if !got.GetBoardRead() || !got.GetBoardPersisted() {
		t.Fatalf("Ingest said board_read %v, board_persisted %v; the gateway has a board and the store takes it", got.GetBoardRead(), got.GetBoardPersisted())
	}
	next := gatewayEngine(t, store, "v1", nil)
	rt, _ := timedCheck(t, next, ingestEngine(t, next, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m"}}).GetName(), true)
	if src := lookup(rt, "board"); src != "store" {
		t.Errorf("the checking worker's board came from %q, want the store the loading worker filled", src)
	}
	coldEng := gatewayEngine(t, &wasmhost.MemCache{}, "v1", nil)
	cold, _ := timedCheck(t, coldEng, ingestEngine(t, coldEng, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m"}}).GetName(), true)
	if src := lookup(cold, "board"); src != "built" {
		t.Errorf("control: with nothing loaded first, the board came from %q, want built", src)
	}
}

// TestABuildThatPersistsNothingSaysSo checks that an engine with no version keeps nothing past
// itself. The page then learns that loading in a worker that ends gained nothing.
func TestABuildThatPersistsNothingSaysSo(t *testing.T) {
	got := loadDesign(t, gatewayEngine(t, &wasmhost.MemCache{}, "", nil)).GetTiers()
	if !got.GetBoardRead() || got.GetBoardPersisted() {
		t.Errorf("Ingest said board_read %v, board_persisted %v; want a board that was not persisted", got.GetBoardRead(), got.GetBoardPersisted())
	}
}

// TestACheckAfterALoadInTheSameWorkerStartsFromItsModel runs the shape agni serve has, with one
// process. The read_board ingest keeps a model, and a check of the snapshot finds it.
func TestACheckAfterALoadInTheSameWorkerStartsFromItsModel(t *testing.T) {
	eng := gatewayEngine(t, &wasmhost.MemCache{}, "v1", nil)
	snap := loadDesign(t, eng).GetName()
	rt, _ := timedCheck(t, eng, snap, true)
	if src := lookup(rt, "model"); src != "memory" {
		t.Errorf("the check's model came from %q, want the one the ingest kept, from memory", src)
	}
}

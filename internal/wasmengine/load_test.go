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

// The tests here cover LoadDesign (agni issue 1061), which the page sends from a worker that ends
// when it answers, so the worker that later runs the checks restores the board instead of parsing it.

func loadDesign(t *testing.T, eng *Engine) *webapi.LoadDesignResponse {
	t.Helper()
	srv := httptest.NewServer(eng.Handler)
	defer srv.Close()
	resp, err := webapiconnect.NewCheckServiceClient(srv.Client(), srv.URL).LoadDesign(context.Background(), connect.NewRequest(&webapi.LoadDesignRequest{Uri: "mount://m"}))
	if err != nil {
		t.Fatal(err)
	}
	return resp.Msg
}

// TestABoardLoadedInOneWorkerIsRestoredByTheNext is the page's two-worker shape: the prepare worker
// loads the design and ends, and a fresh worker over the same store runs the checks. control: a fresh
// worker over a store nothing loaded into parses the board itself.
func TestABoardLoadedInOneWorkerIsRestoredByTheNext(t *testing.T) {
	store := &wasmhost.MemCache{}
	got := loadDesign(t, gatewayEngine(t, store, "v1", nil))
	if !got.GetHasBoard() || !got.GetBoardPersisted() {
		t.Fatalf("LoadDesign said has_board %v, board_persisted %v; the gateway has a board and the store takes it", got.GetHasBoard(), got.GetBoardPersisted())
	}
	rt, _ := timedCheck(t, gatewayEngine(t, store, "v1", nil), true)
	if src := lookup(rt, "board"); src != "store" {
		t.Errorf("the checking worker's board came from %q, want the store the loading worker filled", src)
	}
	cold, _ := timedCheck(t, gatewayEngine(t, &wasmhost.MemCache{}, "v1", nil), true)
	if src := lookup(cold, "board"); src != "built" {
		t.Errorf("control: with nothing loaded first, the board came from %q, want built", src)
	}
}

// TestABuildThatPersistsNothingSaysSo: an engine with no version keeps nothing past itself, so the page
// learns that loading in a worker that ends bought nothing.
func TestABuildThatPersistsNothingSaysSo(t *testing.T) {
	got := loadDesign(t, gatewayEngine(t, &wasmhost.MemCache{}, "", nil))
	if !got.GetHasBoard() || got.GetBoardPersisted() {
		t.Errorf("LoadDesign said has_board %v, board_persisted %v; want a board that was not persisted", got.GetHasBoard(), got.GetBoardPersisted())
	}
}

// TestACheckAfterALoadInTheSameWorkerStartsFromItsModel is the shape agni serve has, one process: the
// check finds the model LoadDesign kept.
func TestACheckAfterALoadInTheSameWorkerStartsFromItsModel(t *testing.T) {
	eng := gatewayEngine(t, &wasmhost.MemCache{}, "v1", nil)
	loadDesign(t, eng)
	rt, _ := timedCheck(t, eng, true)
	if src := lookup(rt, "model"); src != "memory" {
		t.Errorf("the check's model came from %q, want the one LoadDesign kept, from memory", src)
	}
}

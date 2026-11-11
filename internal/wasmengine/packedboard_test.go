package wasmengine

import (
	"context"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/goapplib/wasmhost"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// The tests here cover the packed board kept as a layer of its own (agni issue 1522), so a board view
// restores the packed answer rather than restoring, copying and packing the board.

// timedBoardSheet asks srv for the packed Board sheet with its timing, and returns the answer and
// which tier served each layer.
func timedBoardSheet(t *testing.T, srv *httptest.Server, snap string) (*webapi.GetSheetResponse, *webapi.RequestTiming) {
	t.Helper()
	req := connect.NewRequest(&webapi.GetSheetRequest{Snapshot: snap, Sheet: "board", Format: webapi.SheetFormat_SHEET_FORMAT_PACKED})
	req.Header().Set("Agni-Timing", "1")
	resp, err := webapiconnect.NewDesignServiceClient(srv.Client(), srv.URL).GetSheet(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var rt webapi.RequestTiming
	if err := protojson.Unmarshal([]byte(resp.Header().Get("Agni-Timing")), &rt); err != nil {
		t.Fatalf("the Agni-Timing header is not a RequestTiming: %v", err)
	}
	return resp.Msg, &rt
}

func serving(t *testing.T, eng *Engine) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(eng.Handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestAPackedBoardIsKeptAndAskedAgainFromMemory(t *testing.T) {
	srv := serving(t, gatewayEngine(t, nil, "", nil))
	snap := snapAt(t, srv)
	first, rt := timedBoardSheet(t, srv, snap)
	if got := lookup(rt, "packedboard"); got != "built" {
		t.Fatalf("the first Board sheet came from %q, want built", got)
	}
	if len(first.GetPacked().GetPrimitives()) == 0 || len(first.GetBoardLayers()) == 0 {
		t.Fatal("the packed board has no primitives or no layer rows, so keeping it proves nothing")
	}
	again, rt := timedBoardSheet(t, srv, snap)
	if got := lookup(rt, "packedboard"); got != "memory" {
		t.Errorf("the second Board sheet came from %q, want memory", got)
	}
	if !proto.Equal(again, first) {
		t.Error("the kept packed board differs from the one built")
	}
}

// TestAReloadedWorkerRestoresThePackedBoardWithoutTheBoard runs a reload's shape after #1515. The
// prepare worker has the board's bytes and stores the packed board. A worker holding the board only
// as a placeholder restores the packed answer, and restores no board layer to get it.
func TestAReloadedWorkerRestoresThePackedBoardWithoutTheBoard(t *testing.T) {
	store := &wasmhost.MemCache{}
	first := serving(t, gatewayEngine(t, store, "v1", withSums))
	want, _ := timedBoardSheet(t, first, snapAt(t, first))

	reload := serving(t, gatewayEngine(t, store, "v1", func(f map[string][]byte) { withSums(f); holdBoard(f) }))
	got, rt := timedBoardSheet(t, reload, snapAt(t, reload))
	if src := lookup(rt, "packedboard"); src != "store" {
		t.Errorf("the reloaded worker's packed board came from %q, want the store", src)
	}
	if src := lookup(rt, "board"); src != "" {
		t.Errorf("the reloaded worker looked up the board (%q) for a packed board it had kept", src)
	}
	if !proto.Equal(got, want) {
		t.Error("the restored packed board differs from the one the first worker built")
	}
}

// TestAnEditedBoardPacksAgain checks that the kept packed board goes stale with its board. A worker
// over the same store, with the board's file edited, packs the board again rather than restoring the
// old answer.
func TestAnEditedBoardPacksAgain(t *testing.T) {
	store := &wasmhost.MemCache{}
	first := serving(t, gatewayEngine(t, store, "v1", withSums))
	timedBoardSheet(t, first, snapAt(t, first))

	edited := serving(t, gatewayEngine(t, store, "v1", func(f map[string][]byte) {
		f[gatewayBoard] = append(append([]byte{}, f[gatewayBoard]...), '\n')
		withSums(f)
	}))
	_, rt := timedBoardSheet(t, edited, snapAt(t, edited))
	if src := lookup(rt, "packedboard"); src != "built" {
		t.Errorf("an edited board's packed board came from %q, want built", src)
	}
}

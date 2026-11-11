package wasmengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/goapplib/wasmhost"
)

// gatewaySeed builds the gateway's seed under version v, as a server would.
func gatewaySeed(t *testing.T, v string) []byte {
	t.Helper()
	var files []SeedFile
	for p, b := range gatewayAlone(t) {
		files = append(files, SeedFile{Path: p, Bytes: b})
	}
	seed, err := Seed(context.Background(), "m", "mount://m", files, v)
	if err != nil {
		t.Fatal(err)
	}
	return seed
}

// postSeed posts a seed to an engine's SeedPath and returns how many entries it put.
func postSeed(t *testing.T, srv *httptest.Server, seed []byte, v string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+SeedPath, bytes.NewReader(seed))
	req.Header.Set(SeedVersionHeader, v)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got struct{ Put int }
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("seed answer: %v (status %d)", err, resp.StatusCode)
	}
	return got.Put
}

// TestAFirstVisitRestoresTheSeedWithoutTheBoard covers agni issue 1525. A server builds the gateway's
// seed natively. A worker with an empty store takes it, holds the board only as a placeholder, and
// still reads the design with its board and draws the Board sheet, all from the store. As a control,
// the same worker without the seed is refused for the board's bytes.
func TestAFirstVisitRestoresTheSeedWithoutTheBoard(t *testing.T) {
	seed := gatewaySeed(t, "v1")
	if len(seed) == 0 {
		t.Fatal("the gateway's seed is empty")
	}
	visit := func(t *testing.T, seeded bool) (*httptest.Server, error) {
		srv := serving(t, gatewayEngine(t, &wasmhost.MemCache{}, "v1", func(f map[string][]byte) { withSums(f); holdBoard(f) }))
		if seeded {
			if n := postSeed(t, srv, seed, "v1"); n == 0 {
				t.Fatal("the worker put none of the seed's entries")
			}
		}
		_, err := webapiconnect.NewSnapshotServiceClient(srv.Client(), srv.URL).Ingest(context.Background(), connect.NewRequest(&webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m"}, ReadBoard: true}))
		return srv, err
	}

	srv, err := visit(t, true)
	if err != nil {
		t.Fatalf("reading the design with its board from the seed: %v", err)
	}
	got, rt := timedBoardSheet(t, srv, snapAt(t, srv))
	if src := lookup(rt, "packedboard"); src != "store" {
		t.Errorf("the seeded worker's packed board came from %q, want the store", src)
	}
	if len(got.GetPacked().GetPrimitives()) == 0 {
		t.Error("the seeded Board sheet drew nothing")
	}

	_, err = visit(t, false)
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeFailedPrecondition {
		t.Errorf("without the seed the read answered %v, want the refusal for the board's bytes", err)
	}
}

func TestASeedFromAnotherVersionPutsNothing(t *testing.T) {
	seed := gatewaySeed(t, "v1")
	srv := serving(t, gatewayEngine(t, &wasmhost.MemCache{}, "v2", nil))
	if n := postSeed(t, srv, seed, "v1"); n != 0 {
		t.Errorf("a v2 worker put %d entries of a v1 seed", n)
	}
	if n := postSeed(t, srv, gatewaySeed(t, "v2"), "v2"); n == 0 {
		t.Error("a v2 worker put none of a v2 seed, so the refusal above proves nothing")
	}
}

func TestAnEngineWithNoVersionBuildsNoSeed(t *testing.T) {
	if seed := gatewaySeed(t, ""); len(seed) != 0 {
		t.Errorf("an unversioned build seeded %d bytes", len(seed))
	}
}

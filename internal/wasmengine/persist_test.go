package wasmengine

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/service"
	"github.com/panyam/goapplib/wasmhost"
	"google.golang.org/protobuf/proto"
)

// The browser hands the engine goapplib's cache, so the port must be its shape.
var _ service.BlobStore = (*wasmhost.MemCache)(nil)

// countingStore is a store that counts what was written, since every layer a worker READ is written
// and a layer it restored is not.
type countingStore struct {
	wasmhost.MemCache
	puts atomic.Int64
}

func (s *countingStore) Put(ctx context.Context, key string, b []byte) error {
	s.puts.Add(1)
	return s.MemCache.Put(ctx, key, b)
}

// opened is what a viewer asks of the gateway as it opens and runs its checks.
func opened(t *testing.T, eng *Engine) []proto.Message {
	t.Helper()
	srv := httptest.NewServer(eng.Handler)
	defer srv.Close()
	ctx := context.Background()
	d, err := webapiconnect.NewDesignServiceClient(srv.Client(), srv.URL).GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Uri: "mount://m"}))
	if err != nil {
		t.Fatal(err)
	}
	c, err := webapiconnect.NewCheckServiceClient(srv.Client(), srv.URL).CheckDesign(ctx, connect.NewRequest(&webapi.CheckDesignRequest{Uri: "mount://m"}))
	if err != nil {
		t.Fatal(err)
	}
	return []proto.Message{d.Msg, c.Msg}
}

func gatewayEngine(t *testing.T, store service.BlobStore, version string, edit func(map[string][]byte)) *Engine {
	t.Helper()
	files := readTree(t, filepath.Join("..", "..", "examples", "tutorial-project", "designs", "gateway"))
	if edit != nil {
		edit(files)
	}
	// MemFS stamps every file with the moment it was made, as a browser worker's mount does, so a
	// second engine's files never stamp as the first's did.
	eng, err := NewWith(Options{Store: store, Version: version}, fshost.Mount{Name: "m", FS: fshost.MemFS(files)})
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

func TestASecondWorkerRestoresTheReadAndAnswersTheSame(t *testing.T) {
	store := &countingStore{}
	first := gatewayEngine(t, store, "v1", nil)
	want := opened(t, first)
	written := store.puts.Load()
	if written < 2 {
		t.Fatalf("the first worker stored %d layers; the gateway's design and board should both be kept", written)
	}
	if first.Cache.Restored() != 0 {
		t.Fatalf("the first worker restored %d layers from an empty store", first.Cache.Restored())
	}
	if len(want[1].(*webapi.CheckDesignResponse).GetFindings()) == 0 {
		t.Fatal("the gateway has no findings, so agreeing on them proves nothing")
	}

	second := gatewayEngine(t, store, "v1", nil)
	got := opened(t, second)
	if store.puts.Load() != written {
		t.Errorf("the second worker stored %d more layers, so it read what it should have restored", store.puts.Load()-written)
	}
	if second.Cache.Restored() != written {
		t.Errorf("the second worker restored %d layers; the first stored %d", second.Cache.Restored(), written)
	}
	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("answer %d from the restored worker differs from the reading one", i)
		}
	}
}

func TestAnEditedFileIsReadAgainAndTheRestStillRestores(t *testing.T) {
	store := &countingStore{}
	opened(t, gatewayEngine(t, store, "v1", nil))
	written := store.puts.Load()

	// Whitespace after the board's last form, so it parses to the same board and only the bytes say
	// it changed.
	edited := gatewayEngine(t, store, "v1", func(f map[string][]byte) { f["gateway.kicad_pcb"] = append(f["gateway.kicad_pcb"], '\n') })
	opened(t, edited)
	if store.puts.Load() == written {
		t.Error("an edited board was restored rather than read")
	}
	if edited.Cache.Restored() == 0 {
		t.Error("nothing was restored, though only the board changed")
	}
}

func TestAnotherEngineVersionRestoresNothing(t *testing.T) {
	store := &countingStore{}
	opened(t, gatewayEngine(t, store, "v1", nil))
	other := gatewayEngine(t, store, "v2", nil)
	opened(t, other)
	if other.Cache.Restored() != 0 {
		t.Errorf("an engine of another version restored %d layers another version parsed", other.Cache.Restored())
	}
}

func TestAnEngineWithNoVersionStoresNothing(t *testing.T) {
	store := &countingStore{}
	eng, err := NewWith(Options{Store: store, Version: ""}, fshost.Mount{Name: "m", FS: fshost.MemFS(readTree(t, filepath.Join("..", "..", "examples", "tutorial-project", "designs", "gateway")))})
	if err != nil {
		t.Fatal(err)
	}
	if StoreVersion() != "" {
		t.Skip("this test binary carries a version, so the empty Version falls back to it")
	}
	opened(t, eng)
	if store.puts.Load() != 0 {
		t.Errorf("a build that cannot say what it is stored %d layers", store.puts.Load())
	}
}

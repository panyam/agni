package wasmengine

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"

	"connectrpc.com/connect"
	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/service"
)

// A seed is the store a browser worker would write reading a design, built ahead of the visit
// (agni issue 1525). A first wasm visit then restores the parsed board, the drawing and the packed
// Board sheet, instead of fetching the board's bytes and parsing them in the browser. On Jetson the
// whole store is about 14 MB against an 85 MB board file, and a native engine builds it in about
// 0.6 s against wasm's 1.7 s parse of the board alone.
//
// The entries are keyed as the worker keys them, under one store version. A worker takes a seed only
// when its own version is the same, since another build could read a file differently.

// SeedPath is the engine's route a page posts a seed to. The body is the seed as the server sent it,
// and SeedVersionHeader names the version it was built under.
const SeedPath = "/agni-seed"

// SeedVersionHeader names the store version a seed was built under.
const SeedVersionHeader = "Agni-Store-Version"

// SeedFile is one design file as the server lists it.
type SeedFile struct {
	Path  string
	Bytes []byte
}

// Seed builds a design's store entries the way a browser worker would write them. The engine reads
// the design at uri with its board and draws the packed Board sheet, over a mount holding exactly
// files and the manifest the page writes beside them (SumsFile). That mount is what a worker holds,
// so every entry's fingerprint matches the worker's own reading of it.
//
// It returns no seed for an empty version, since an engine with no version persists nothing. A
// design with no board still seeds what it read.
func Seed(ctx context.Context, mount, uri string, files []SeedFile, version string) ([]byte, error) {
	if version == "" {
		return nil, nil
	}
	mem := map[string][]byte{}
	for _, f := range files {
		mem[f.Path] = f.Bytes
	}
	b, err := sumsFileFor(files)
	if err != nil {
		return nil, err
	}
	mem[SumsFile] = b
	store := &capturingStore{entries: map[string][]byte{}}
	eng, err := NewWith(Options{Store: store, Version: version}, fshost.Mount{Name: mount, FS: fshost.MemFS(mem)})
	if err != nil {
		return nil, err
	}
	c := handlerClient{eng.Handler}
	snap, err := webapiconnect.NewSnapshotServiceClient(c, "http://seed").Ingest(ctx, connect.NewRequest(&webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: uri}, ReadBoard: true}))
	if err != nil {
		return nil, fmt.Errorf("reading %s for its seed: %w", uri, err)
	}
	if snap.Msg.GetTiers().GetBoardRead() {
		if _, err := webapiconnect.NewDesignServiceClient(c, "http://seed").GetSheet(ctx, connect.NewRequest(&webapi.GetSheetRequest{Snapshot: snap.Msg.GetName(), Sheet: "board", Format: webapi.SheetFormat_SHEET_FORMAT_PACKED})); err != nil {
			return nil, fmt.Errorf("drawing %s's board for its seed: %w", uri, err)
		}
	}
	return store.encode(), nil
}

// sumsFileFor writes the manifest the page writes beside these files (SumsFile), with each file's
// sha256 and size.
func sumsFileFor(files []SeedFile) ([]byte, error) {
	m := sumsManifest{Files: map[string]listedSum{}}
	for _, f := range files {
		sum := sha256.Sum256(f.Bytes)
		m.Files[f.Path] = listedSum{SHA256: "sha256:" + hex.EncodeToString(sum[:]), Size: int64(len(f.Bytes))}
	}
	return json.Marshal(m)
}

// ApplySeed puts a seed's entries into store and reports how many it put. It puts nothing when the
// seed was built under another version than want.
func ApplySeed(ctx context.Context, store service.BlobStore, seed []byte, version, want string) (int, error) {
	if store == nil || want == "" || version != want {
		return 0, nil
	}
	entries, err := decodeSeed(seed)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if err := store.Put(ctx, e.key, e.bytes); err != nil {
			return 0, err
		}
	}
	return len(entries), nil
}

// seedHandler serves SeedPath on an engine over store.
func seedHandler(store service.BlobStore, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		n, err := ApplySeed(r.Context(), store, body, r.Header.Get(SeedVersionHeader), version)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"put": n})
	}
}

// capturingStore keeps what an engine writes, to send as a seed.
type capturingStore struct {
	mu      sync.Mutex
	entries map[string][]byte
}

func (s *capturingStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.entries[key]
	if !ok {
		return nil, errNoEntry
	}
	return b, nil
}

func (s *capturingStore) Put(_ context.Context, key string, b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = append([]byte(nil), b...)
	return nil
}

var errNoEntry = errors.New("no such entry")

type seedEntry struct {
	key   string
	bytes []byte
}

// encode writes each entry as a uvarint-prefixed key and a uvarint-prefixed value, in key order.
func (s *capturingStore) encode() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.entries))
	for k := range s.entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []byte
	for _, k := range keys {
		out = binary.AppendUvarint(out, uint64(len(k)))
		out = append(out, k...)
		out = binary.AppendUvarint(out, uint64(len(s.entries[k])))
		out = append(out, s.entries[k]...)
	}
	return out
}

func decodeSeed(b []byte) ([]seedEntry, error) {
	var out []seedEntry
	next := func() ([]byte, error) {
		n, k := binary.Uvarint(b)
		if k <= 0 || uint64(len(b)-k) < n {
			return nil, errors.New("a seed entry runs past the end")
		}
		v := b[k : k+int(n)] // #nosec G115 -- n <= len(b)-k, checked just above
		b = b[k+int(n):]     // #nosec G115 -- as above
		return v, nil
	}
	for len(b) > 0 {
		key, err := next()
		if err != nil {
			return nil, err
		}
		val, err := next()
		if err != nil {
			return nil, err
		}
		out = append(out, seedEntry{key: string(key), bytes: val})
	}
	return out, nil
}

// handlerClient answers a Connect client's requests with an in-process handler.
type handlerClient struct{ h http.Handler }

func (c handlerClient) Do(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, r)
	return rec.Result(), nil
}

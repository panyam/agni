package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/wasmengine"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
)

// seeds serves /seed/<mount>/<path>, a design's store as a browser worker would write it (agni issue
// 1525). A page analysing the design in the browser puts it into its store before the first read, and
// restores the parsed board rather than fetching and parsing it.
//
// It builds a seed natively over exactly the files ListDesignFiles names, under this binary's
// wasmengine.StoreVersion. The page's worker takes it only under the same version, so a server built
// from another tree than the page's agni.wasm costs one wasted fetch and nothing else. A binary that
// cannot name its version answers 204 with no seed. A built seed is kept per version and file set.
//
// Building Jetson's seed parses its board, about 860 ms, and a first browser-engine visit used to wait
// for it (agni issue 1572). warm starts the build in the background as soon as the server hears of a
// design, so the visit finds it built or under way. A request and a warm for one seed share one build.
type seeds struct {
	ws      *service.WorkspaceService
	ms      []mounts.Mount
	version func() string

	mu       sync.Mutex
	kept     map[string][]byte
	order    []string
	building map[string]*seedBuild
	// builds counts the seeds built, for a test that one warm and one request build once.
	builds atomic.Int32
}

// seedBuild is one build under way, which every caller asking for the same seed waits on.
type seedBuild struct {
	done chan struct{}
	seed []byte
	err  error
}

// errNoDesign says the path names no design this server lists, which the handler answers with 404.
var errNoDesign = errors.New("no such design")

// keptSeeds is how many seeds a server keeps. The oldest goes first when it is full.
const keptSeeds = 4

func newSeeds(ws *service.WorkspaceService, ms []mounts.Mount, version func() string) *seeds {
	return &seeds{ws: ws, ms: ms, version: version, kept: map[string][]byte{}, building: map[string]*seedBuild{}}
}

func (s *seeds) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	v := s.version()
	if v == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	mount, p, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/seed/"), "/")
	seed, err := s.get(r.Context(), mount, p, v)
	switch {
	case errors.Is(err, errNoDesign):
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	case err != nil:
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set(wasmengine.SeedVersionHeader, v)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(seed))
}

// warm builds a design's seed in the background, when this binary can name its version. It returns at
// once, and a failed build is dropped, since the page that asks for the seed reports its own.
func (s *seeds) warm(mount, p string) {
	v := s.version()
	if v == "" {
		return
	}
	go func() { _, _ = s.get(context.Background(), mount, p, v) }()
}

// get answers a design's seed under version v from the cache, from a build already under way, or
// from a build of its own.
func (s *seeds) get(ctx context.Context, mount, p, v string) ([]byte, error) {
	u, err := artifact.New(mount, p)
	if err != nil {
		return nil, errNoDesign
	}
	listed, err := s.ws.ListDesignFiles(ctx, &webapi.ListDesignFilesRequest{Uri: u.String()})
	if err != nil {
		return nil, errNoDesign
	}
	id := []string{v, u.String()}
	for _, f := range listed.GetFiles() {
		id = append(id, f.GetPath(), f.GetSha256())
	}
	key := strings.Join(id, "\x00")

	s.mu.Lock()
	if seed, ok := s.kept[key]; ok {
		s.mu.Unlock()
		return seed, nil
	}
	b, ok := s.building[key]
	if !ok {
		b = &seedBuild{done: make(chan struct{})}
		s.building[key] = b
		// The build runs apart from the request that started it, so a page that goes away doesn't
		// cancel it for the next one.
		go s.build(key, b, u, listed.GetFiles(), v)
	}
	s.mu.Unlock()
	select {
	case <-b.done:
		return b.seed, b.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *seeds) build(key string, b *seedBuild, u artifact.URI, listed []*webapi.DesignFile, v string) {
	s.builds.Add(1)
	b.seed, b.err = s.buildSeed(u, listed, v)
	s.mu.Lock()
	delete(s.building, key)
	if b.err == nil {
		s.kept[key] = b.seed
		s.order = append(s.order, key)
		for len(s.order) > keptSeeds {
			delete(s.kept, s.order[0])
			s.order = s.order[1:]
		}
	}
	s.mu.Unlock()
	close(b.done)
}

func (s *seeds) buildSeed(u artifact.URI, listed []*webapi.DesignFile, v string) ([]byte, error) {
	files := make([]wasmengine.SeedFile, 0, len(listed))
	for _, f := range listed {
		fu, err := artifact.New(u.Mount, f.GetPath())
		if err != nil {
			return nil, errNoDesign
		}
		abs, err := mounts.Resolve(s.ms, fu)
		if err != nil {
			return nil, errNoDesign
		}
		b, err := os.ReadFile(abs) // #nosec G304 G703 -- abs comes from mounts.Resolve, which refuses a path outside its mount
		if err != nil {
			return nil, errNoDesign
		}
		files = append(files, wasmengine.SeedFile{Path: f.GetPath(), Bytes: b})
	}
	return wasmengine.Seed(context.Background(), u.Mount, u.String(), files, v)
}

// warmSeeds is a connect interceptor that warms a design's seed whenever a request names the design:
// a page listing its files, as a browser-engine visit does before it asks for the seed, or the server
// reading it, as a server-engine visit does. A teammate's first browser-engine visit then finds the
// seed built.
func warmSeeds(s *seeds) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err == nil {
				if u, ok := designNamed(req.Any()); ok {
					s.warm(u.Mount, u.Path)
				}
			}
			return resp, err
		}
	}
}

// designNamed is the design a request lists or reads. It answers false for any other request, and for
// a design sent as files, which has no seed to warm.
func designNamed(msg any) (artifact.URI, bool) {
	var uri string
	switch m := msg.(type) {
	case *webapi.ListDesignFilesRequest:
		uri = m.GetUri()
	case *webapi.IngestRequest:
		if len(m.GetSnapshot().GetFiles()) > 0 {
			return artifact.URI{}, false
		}
		uri = m.GetSnapshot().GetUri()
	}
	if uri == "" {
		return artifact.URI{}, false
	}
	u, err := artifact.Parse(uri)
	return u, err == nil
}

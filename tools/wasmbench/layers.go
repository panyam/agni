package main

import (
	"context"
	"fmt"
	"io/fs"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/panyam/agni"
	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/num"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/fshost"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/service"
	"github.com/panyam/goapplib/wasmhost"
)

// A Layer is one thing the browser engine's worker holds for an open design, measured as the Go heap
// after building it and collecting garbage (agni issue 981).
type Layer struct {
	Name string `json:"name"`
	MS   int64  `json:"ms"`
	// RetainedMB is how much the live heap grew for this layer, which is what keeping it costs.
	RetainedMB float64 `json:"retained_mb"`
	// AllocatedMB is everything allocated while building it, garbage included. Wasm memory never
	// shrinks, so a worker's size follows the largest of these more than the live heap.
	AllocatedMB float64 `json:"allocated_mb"`
	// HeapMB is the live heap once this layer is held, and SysMB what the runtime has obtained from
	// the system by then, the native counterpart of a worker's linear memory.
	HeapMB float64 `json:"heap_mb"`
	SysMB  float64 `json:"sys_mb"`
}

// LayersResult is the layer split of one board, from a fresh read and from a restore out of the
// persistent tier, which is how a returning visitor's worker gets the design.
type LayersResult struct {
	Host     string  `json:"host"`
	Design   string  `json:"design"`
	Fresh    []Layer `json:"fresh"`
	Restored []Layer `json:"restored"`
	// RestoredReads is how many of the second pass's reads the persistent tier answered.
	RestoredReads int64 `json:"restored_reads"`
}

// Layers builds a design's layers in the order a Run checks needs them, as the worker composes them:
// through a CachingLoader over the folder held in memory, with the design's project overlay and its
// declared tiers. The first pass reads and persists; the second starts from an empty in-memory cache
// over the same store, so its design, drawing and board are restored rather than parsed.
func Layers(dir, design string) (LayersResult, error) {
	res := LayersResult{Host: runtime.GOOS + "/" + runtime.GOARCH, Design: design}
	before := heap()
	t := time.Now()
	files, _, err := readDir(dir)
	if err != nil {
		return res, err
	}
	mfs := fshost.MemFS(files)
	after := heap()
	held := Layer{Name: "design files", MS: time.Since(t).Milliseconds(), RetainedMB: mb(num.Int64(after.HeapAlloc) - num.Int64(before.HeapAlloc)),
		AllocatedMB: mb(num.Int64(after.TotalAlloc) - num.Int64(before.TotalAlloc)), HeapMB: mb(num.Int64(after.HeapAlloc)), SysMB: mb(num.Int64(after.Sys))}
	store := &wasmhost.MemCache{}
	if res.Fresh, _, err = layerPass(mfs, store, design); err != nil {
		return res, err
	}
	res.Fresh = append([]Layer{held}, res.Fresh...)
	if res.Restored, res.RestoredReads, err = layerPass(mfs, store, design); err != nil {
		return res, err
	}
	return res, nil
}

// layerPass composes what wasmengine.NewWith does, less the transport, and builds each layer in turn.
// Everything it built stays reachable until the pass ends, as it would in a worker. It returns how
// many reads the persistent tier answered.
func layerPass(mfs fs.FS, store service.BlobStore, design string) ([]Layer, int64, error) {
	ctx := context.Background()
	mount := fshost.Mount{Name: "bench", FS: mfs}
	host := fshost.New(mount)
	root := host.Root()
	cache := service.NewDesignCache(8).Persist(store, func(fp []byte) (*formats.Touched, bool) {
		return formats.CheckFingerprint(root, fp)
	}, "wasmbench-layers")
	loader := service.NewCachingLoader(host, cache)
	resolver := &service.ProjectResolver{
		Store:  projects.NewFSStore(projects.Tree{Mount: mount.Name, FS: mfs}),
		Config: &fshost.ConfigResolver{Mounts: []fshost.Mount{mount}},
	}
	eng, err := agni.New(agni.WithProjectResolver(resolver))
	if err != nil {
		return nil, 0, err
	}
	u, err := service.ParseArtifactURI("mount://bench/" + strings.TrimPrefix(design, "/"))
	if err != nil {
		return nil, 0, err
	}
	ov, err := resolver.Overlay(ctx, u, nil, "")
	if err != nil {
		return nil, 0, err
	}
	nu, bu, gu, err := resolver.TierURIs(ctx, u, artifact.URI{}, false)
	if err != nil {
		return nil, 0, err
	}
	// A design declaring no board companion has its board read from its own URI, as BuildModel does
	// with no board URI. Passing that URI to BuildModel would make it an explicit board, which is an
	// error when the file carries none.
	boardFrom := bu
	if boardFrom.IsZero() {
		boardFrom = nu
	}
	opts := ov.ReadOptions()

	var (
		layers []Layer
		d      *ir.Design
		sg     *geom.SchematicGeometry
		bg     *geom.BoardGeometry
		m      check.Model
		base   *query.Base
		probe  check.Model
	)
	prev := heap()
	step := func(name string, build func() error) error {
		before := heap()
		t := time.Now()
		if err := build(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		ms := time.Since(t).Milliseconds()
		after := heap()
		layers = append(layers, Layer{
			Name:        name,
			MS:          ms,
			RetainedMB:  mb(num.Int64(after.HeapAlloc) - num.Int64(prev.HeapAlloc)),
			AllocatedMB: mb(num.Int64(after.TotalAlloc) - num.Int64(before.TotalAlloc)),
			HeapMB:      mb(num.Int64(after.HeapAlloc)),
			SysMB:       mb(num.Int64(after.Sys)),
		})
		prev = after
		return nil
	}
	steps := []struct {
		name  string
		build func() error
	}{
		{"design", func() (err error) { d, err = loader.Design(ctx, nu, opts...); return }},
		{"schematic geometry", func() error { sg = service.BuildGeometry(ctx, loader, gu, opts...); return nil }},
		// A probe, dropped once measured: the model as a design whose board is never read would hold
		// it, which is what reading the board lazily would leave a query-only worker holding.
		{"check model without the board (probe, dropped)", func() (err error) {
			probe, err = service.BuildModel(ctx, noBoard{loader}, nu, artifact.URI{}, ov.SpecsOver(nil), opts...)
			return
		}},
		{"board geometry", func() (err error) { bg, err = loader.Board(ctx, boardFrom); return }},
		// BuildModel through the cache reads the design and board again, and both are hits, so this
		// step is the model alone.
		{"check model", func() (err error) {
			m, err = service.BuildModel(ctx, loader, nu, bu, ov.SpecsOver(nil), opts...)
			return
		}},
		{"fact base", func() error {
			reg, err := ov.Registry()
			if err == nil {
				base = query.NewBaseFrom(reg, m)
			}
			return err
		}},
		// The model memoizes what rules ask of it, so a check run leaves the worker holding more.
		{"after a check run", func() error {
			cat, err := ov.Catalog(eng.Catalog())
			if err == nil {
				_, err = check.Run(ctx, m, cat.Rules())
			}
			return err
		}},
	}
	for _, s := range steps {
		if err := step(s.name, s.build); err != nil {
			return nil, 0, err
		}
		if probe != nil {
			probe = nil
			prev = heap()
		}
	}
	runtime.KeepAlive(d)
	runtime.KeepAlive(sg)
	runtime.KeepAlive(bg)
	runtime.KeepAlive(base)
	runtime.KeepAlive(cache)
	return layers, cache.Restored(), nil
}

func heap() runtime.MemStats {
	runtime.GC()
	runtime.GC()
	var s runtime.MemStats
	runtime.ReadMemStats(&s)
	return s
}

// mb is b in megabytes to the kilobyte, so a small design's layers do not all round to zero.
func mb(b int64) float64 { return math.Round(float64(b)/(1<<20)*1000) / 1000 }

// noBoard is a loader whose designs have no board, for the probe.
type noBoard struct{ service.ModelLoader }

func (noBoard) Board(context.Context, artifact.URI) (*geom.BoardGeometry, error) { return nil, nil }

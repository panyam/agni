package main

import (
	"context"
	"errors"
	"os"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/graph"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/review"
)

// osLoader is the OS-backed service.Loader adapter: it resolves a (mount, path) to an absolute host
// path under the mount root and reuses the engine's readers/auto-layout (readDesign,
// resolveGeometry, buildConversionReport). All file I/O stays at the cmd edge (CONSTRAINTS C1/C13);
// the service package is os-free. A future WASM build would inject a seededLoader instead.
type osLoader struct {
	mounts []mounts.Mount
	loader *formats.Loader
}

func (l *osLoader) Design(_ context.Context, mountName, path string) (*ir.Design, error) {
	abs, err := mounts.Resolve(l.mounts, mountName, path)
	if err != nil {
		return nil, err
	}
	return l.loader.ReadDesign(abs)
}

func (l *osLoader) Geometry(_ context.Context, mountName, path, layout string, faithfulSymbols bool) (*geom.SchematicGeometry, error) {
	abs, err := mounts.Resolve(l.mounts, mountName, path)
	if err != nil {
		return nil, err
	}
	return l.loader.ResolveGeometry(abs, layout, nil, symbolsFor(faithfulSymbols))
}

func (l *osLoader) Report(_ context.Context, mountName, path string, faithfulSymbols bool) (*graph.ConversionReport, error) {
	abs, err := mounts.Resolve(l.mounts, mountName, path)
	if err != nil {
		return nil, err
	}
	return l.loader.ConversionReport(abs, symbolsFor(faithfulSymbols), nil)
}

// Expectations loads the design's `<path>.expect.yaml` sidecar. No sidecar is the normal case, so a
// missing file returns (nil, nil) rather than an error; only a bad mount/path or a malformed sidecar
// is an error.
func (l *osLoader) Expectations(_ context.Context, mountName, path string) (*expect.Expectations, error) {
	abs, err := mounts.Resolve(l.mounts, mountName, path)
	if err != nil {
		return nil, err
	}
	sidecar := abs + ".expect.yaml"
	if _, err := os.Stat(sidecar); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return expect.Load(sidecar)
}

// symbolsFor maps the service's faithful-symbols bool to the engine's --symbols string.
func symbolsFor(faithful bool) string {
	if faithful {
		return symbolsFaithful
	}
	return symbolsGlyph
}

// Board resolves the physical board sidecar (WS1-006) through the formats registry; formats
// without one yield (nil, nil) — absence is normal, and the service lists no board sheet.
func (l *osLoader) Board(_ context.Context, mountName, path string) (*geom.BoardGeometry, error) {
	abs, err := mounts.Resolve(l.mounts, mountName, path)
	if err != nil {
		return nil, err
	}
	return l.loader.BoardGeometry(abs)
}

// Manifest resolves and parses a review checklist manifest (YAML) under the mount (WS9-047). Unlike
// Expectations, a manifest is a required input, so an absent or malformed file is an error — the
// review would otherwise run against no items and report a hollow pass.
func (l *osLoader) Manifest(_ context.Context, mountName, path string) (review.Manifest, error) {
	abs, err := mounts.Resolve(l.mounts, mountName, path)
	if err != nil {
		return review.Manifest{}, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return review.Manifest{}, err
	}
	defer f.Close()
	return review.Load(f)
}

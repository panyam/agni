package main

import (
	"context"
	"errors"
	"os"

	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/review"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/readers/formats"
)

// localLoader is the CLI's service.Loader: it resolves a bare LOCAL path with NO mount containment
// (the mount arg is ignored). Containment is a serve-only policy that lives in osLoader's
// mounts.Resolve; the CLI runs on the user's own files, so localLoader is the deliberate
// no-containment sibling behind the same Loader interface (WS9-048). It satisfies the full
// service.Loader (the check/query thin clients) plus Manifest (the review thin client), so a CLI
// command constructs its service over this and calls the same method the web serves.
type localLoader struct{ loader *formats.Loader }

func (l *localLoader) Design(_ context.Context, _, path string, opts ...service.ReadOption) (*ir.Design, error) {
	return readerFor(l.loader, opts...).ReadDesign(path)
}

func (l *localLoader) Board(_ context.Context, _, path string) (*geom.BoardGeometry, error) {
	return l.loader.BoardGeometry(path)
}

func (l *localLoader) Geometry(_ context.Context, _, path, layout string, faithful bool) (*geom.SchematicGeometry, error) {
	return l.loader.ResolveGeometry(path, layout, nil, symbolsFor(faithful))
}

func (l *localLoader) Report(_ context.Context, _, path string, faithful bool) (*graph.ConversionReport, error) {
	return l.loader.ConversionReport(path, symbolsFor(faithful), nil)
}

func (l *localLoader) Expectations(_ context.Context, _, path string) (*expect.Expectations, error) {
	sidecar := path + ".expect.yaml"
	if _, err := os.Stat(sidecar); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return expect.Load(sidecar)
}

func (l *localLoader) Manifest(_ context.Context, _, ref string) (review.Manifest, error) {
	return loadManifest(ref)
}

// loadManifest reads and validates a checklist from a local path. It is a package function rather
// than only a loader method because `agni review` no longer goes through a loader to get its
// manifest: the checklist travels to the service as a VALUE (WS9-050), so the CLI reads it at its own
// edge and sends it. The loader method remains for GetReviewManifest, which serves a client that
// holds a ref instead, and both paths share this one read so they cannot disagree about what a
// well-formed manifest is.
func loadManifest(path string) (review.Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return review.Manifest{}, err
	}
	defer f.Close()
	return review.Load(f)
}

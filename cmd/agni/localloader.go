package main

import (
	"context"
	"errors"
	"os"

	"github.com/panyam/agni/readers/formats"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/graph"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/review"
)

// localLoader is the CLI's service.Loader: it resolves a bare LOCAL path with NO mount containment
// (the mount arg is ignored). Containment is a serve-only policy that lives in osLoader's
// mounts.Resolve; the CLI runs on the user's own files, so localLoader is the deliberate
// no-containment sibling behind the same Loader interface (WS9-048). It satisfies the full
// service.Loader (the check/query thin clients) plus Manifest (the review thin client), so a CLI
// command constructs its service over this and calls the same method the web serves.
type localLoader struct{ loader *formats.Loader }

func (l *localLoader) Design(_ context.Context, _, path string) (*ir.Design, error) {
	return l.loader.ReadDesign(path)
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

func (l *localLoader) Manifest(_ context.Context, _, path string) (review.Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return review.Manifest{}, err
	}
	defer f.Close()
	return review.Load(f)
}

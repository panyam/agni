package main

import (
	"context"
	"errors"
	"os"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/review"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/service"
)

// osLoader is the OS-backed service.Loader adapter. It resolves an artifact.URI to a host path under
// its mount root and reads it through the engine's formats.Loader (via readerFor).
type osLoader struct {
	mounts []mounts.Mount
	loader *formats.Loader
}

func (l *osLoader) Design(_ context.Context, uri artifact.URI, opts ...service.ReadOption) (*ir.Design, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return nil, err
	}
	return readerFor(l.loader, opts...).ReadDesign(abs)
}

func (l *osLoader) Geometry(_ context.Context, uri artifact.URI, layout string, faithfulSymbols bool, opts ...service.ReadOption) (*geom.SchematicGeometry, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return nil, err
	}
	// Through readerFor, as in Design, because a project's declared symbol library changes what the
	// geometry read CONTAINS and arrives as a read option (agni issue 347).
	reader := readerFor(l.loader, opts...)
	// The companion is what GetDesign, GetSheet and HighlightSheet draw, so all three funnel through
	// here (WS1-047). The sibling sits in the SAME mount dir as the already-contained abs, so it needs
	// no extra containment check.
	if comp := reader.Companion(abs); comp != "" {
		return reader.FaithfulGeometry(comp)
	}
	return reader.ResolveGeometry(abs, layout, nil, symbolsFor(faithfulSymbols))
}

func (l *osLoader) Report(_ context.Context, uri artifact.URI, faithfulSymbols bool, opts ...service.ReadOption) (*graph.ConversionReport, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return nil, err
	}
	return readerFor(l.loader, opts...).ConversionReport(abs, symbolsFor(faithfulSymbols), nil)
}

// Expectations loads the design's `<path>.expect.yaml` sidecar. No sidecar is the normal case, so a
// missing file returns (nil, nil). Only a bad URI or a malformed sidecar is an error.
func (l *osLoader) Expectations(ctx context.Context, uri artifact.URI) (*expect.Expectations, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return nil, err
	}
	sidecar := abs + ".expect.yaml"
	if _, err := os.Stat(sidecar); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return expect.Load(sidecar)
}

// symbolsFor is formats.SymbolsFor, named for the CLI's call sites.
func symbolsFor(faithful bool) string { return formats.SymbolsFor(faithful) }

// Board resolves the physical board sidecar (WS1-006) through the formats registry. A format
// without one yields (nil, nil), and the service then lists no board sheet.
func (l *osLoader) Board(ctx context.Context, uri artifact.URI) (*geom.BoardGeometry, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return nil, err
	}
	return l.loader.BoardGeometry(abs)
}

// Manifest resolves and parses a review checklist manifest (YAML) under the mount (WS9-047). Unlike
// Expectations, a manifest is a required input, so an absent or malformed file is an error rather
// than a review over no items reporting a hollow pass.
func (l *osLoader) Manifest(ctx context.Context, uri artifact.URI) (review.Manifest, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
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

// Convention resolves and parses a naming-convention config (YAML) under the mount (WS9-128). Like a
// review manifest it is a required input once named, so an absent or malformed file is an error
// rather than silently falling back to the server's vocabulary.
func (l *osLoader) Convention(_ context.Context, uri artifact.URI) (*configpb.NamingConvention, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return nil, err
	}
	return naming.Load(abs)
}

// DesignHash hashes a mounted design's entry file for a stored run's provenance (WS9-053). A URI
// escaping its mount is an error, since containment is a security boundary. An unreadable file
// inside the mount yields "", as hashSource documents.
func (l *osLoader) DesignHash(_ context.Context, uri artifact.URI) (string, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return "", err
	}
	return hashSource(abs), nil
}

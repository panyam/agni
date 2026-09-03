package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/review"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/service"
)

// osLoader is the OS-backed service.Loader adapter: it resolves a (mount, path) to an absolute host
// path under the mount root and reuses the engine's readers/auto-layout (readDesign,
// resolveGeometry, buildConversionReport). All file I/O stays at the cmd edge (CONSTRAINTS C1/C13);
// the service package is os-free. A future WASM build would inject a seededLoader instead.
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
	// Through readerFor for the same reason Design is: a project's declared symbol library is config
	// that changes what the geometry read CONTAINS, and it reaches this call as a read option rather
	// than through the loader the process was built with (agni issue 347).
	reader := readerFor(l.loader, opts...)
	// Companion (WS1-047): a netlist opened alongside a sibling <stem>.eds draws on that schematic
	// instead of the auto-layout graph, so the viewer shows the design's OWN drawing. The netlist
	// stays analysis truth (checks/query read it via Design); only the picture comes from the .eds,
	// joined to findings by net name (C21). All three geometry RPCs funnel through here, so
	// GetDesign / GetSheet / HighlightSheet stay consistent. The sibling sits in the SAME mount dir
	// as the already-contained abs, so no extra containment check is needed (mirrors Expectations).
	if comp := companionEds(abs); comp != "" {
		return reader.FaithfulGeometry(comp)
	}
	return reader.ResolveGeometry(abs, layout, nil, symbolsFor(faithfulSymbols))
}

// companionEds returns a sibling <stem>.eds schematic for a NETLIST design, or "" when the design
// already carries its own geometry (an .eds/.kicad_sch draws itself) or no sibling exists. Filename
// only — it never reads a file's contents.
func companionEds(abs string) string {
	if formats.HasFaithful(abs) {
		return "" // the design already draws itself; no companion needed
	}
	sib := strings.TrimSuffix(abs, filepath.Ext(abs)) + ".eds"
	if sib == abs {
		return ""
	}
	if st, err := os.Stat(sib); err == nil && !st.IsDir() {
		return sib
	}
	return ""
}

func (l *osLoader) Report(_ context.Context, uri artifact.URI, faithfulSymbols bool, opts ...service.ReadOption) (*graph.ConversionReport, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return nil, err
	}
	return readerFor(l.loader, opts...).ConversionReport(abs, symbolsFor(faithfulSymbols), nil)
}

// Expectations loads the design's `<path>.expect.yaml` sidecar. No sidecar is the normal case, so a
// missing file returns (nil, nil) rather than an error; only a bad mount/path or a malformed sidecar
// is an error.
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

// symbolsFor maps the service's faithful-symbols bool to the engine's --symbols string.
func symbolsFor(faithful bool) string {
	if faithful {
		return symbolsFaithful
	}
	return symbolsGlyph
}

// Board resolves the physical board sidecar (WS1-006) through the formats registry; formats
// without one yield (nil, nil) — absence is normal, and the service lists no board sheet.
func (l *osLoader) Board(ctx context.Context, uri artifact.URI) (*geom.BoardGeometry, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return nil, err
	}
	return l.loader.BoardGeometry(abs)
}

// Manifest resolves and parses a review checklist manifest (YAML) under the mount (WS9-047). Unlike
// Expectations, a manifest is a required input, so an absent or malformed file is an error — the
// review would otherwise run against no items and report a hollow pass.
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
// review manifest it is a required input once named, so an absent or malformed file is an error: a
// caller that asked for its own vocabulary and silently got the server's would read the resulting
// findings as being about their naming when they are about somebody else's.
func (l *osLoader) Convention(_ context.Context, uri artifact.URI) (*configpb.NamingConvention, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return nil, err
	}
	return naming.Load(abs)
}

// DesignHash hashes a mounted design's entry file for a stored run's provenance (WS9-053). A ref that
// escapes its mount is still an error, because containment is a security boundary and not a
// provenance nicety; an unreadable file inside the mount yields "" the way hashSource documents.
func (l *osLoader) DesignHash(_ context.Context, uri artifact.URI) (string, error) {
	abs, err := mounts.Resolve(l.mounts, uri)
	if err != nil {
		return "", err
	}
	return hashSource(abs), nil
}

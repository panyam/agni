package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/panyam/agni/core/check/naming"
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
type localLoader struct {
	loader *formats.Loader
	// notes is where a descriptor-resolution note is written, nil for os.Stderr. Notes are emitted
	// once per named path (noted), because one BuildModel asks this loader for the netlist, the board,
	// and the geometry of the SAME path, and a user does not need to be told three times which file
	// was read.
	notes io.Writer
	mu    sync.Mutex
	noted map[string]bool
}

// resolve runs the named path through the enclosing design's descriptor and emits the note at most
// once. Every path-taking method below goes through it, so the CLI's thin clients (check, query,
// review) honour a declared entry the same way the direct readers do.
func (l *localLoader) resolve(path string) (designSource, error) {
	src, err := resolveSource(path)
	if err != nil {
		return designSource{}, err
	}
	if src.Note == "" {
		return src, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.noted[path] {
		return src, nil
	}
	if l.noted == nil {
		l.noted = map[string]bool{}
	}
	l.noted[path] = true
	w := l.notes
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprint(w, src.Note)
	return src, nil
}

func (l *localLoader) Design(_ context.Context, _, path string, opts ...service.ReadOption) (*ir.Design, error) {
	src, err := l.resolve(path)
	if err != nil {
		return nil, err
	}
	return readerFor(l.loader, opts...).ReadDesign(src.Netlist)
}

func (l *localLoader) Board(_ context.Context, _, path string) (*geom.BoardGeometry, error) {
	src, err := l.resolve(path)
	if err != nil {
		return nil, err
	}
	return l.loader.BoardGeometry(src.Board)
}

func (l *localLoader) Geometry(_ context.Context, _, path, layout string, faithful bool) (*geom.SchematicGeometry, error) {
	src, err := l.resolve(path)
	if err != nil {
		return nil, err
	}
	return l.loader.ResolveGeometry(src.Geometry, layout, nil, symbolsFor(faithful))
}

func (l *localLoader) Report(_ context.Context, _, path string, faithful bool) (*graph.ConversionReport, error) {
	src, err := l.resolve(path)
	if err != nil {
		return nil, err
	}
	return l.loader.ConversionReport(src.Geometry, symbolsFor(faithful), nil)
}

// Expectations reads the sidecar beside the ENTRY rather than beside the named file: an expectation
// set states what this design should read, and the design is its entry.
func (l *localLoader) Expectations(_ context.Context, _, path string) (*expect.Expectations, error) {
	src, err := l.resolve(path)
	if err != nil {
		return nil, err
	}
	sidecar := src.Netlist + ".expect.yaml"
	if _, err := os.Stat(sidecar); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return expect.Load(sidecar)
}

func (l *localLoader) Manifest(_ context.Context, _, ref string) (review.Manifest, error) {
	return loadManifest(ref)
}

// Convention resolves a naming-convention config from a local path. The CLI reads its own
// --conventions at the edge and sends the value, so nothing in the CLI calls this; it exists because
// localLoader is the no-containment sibling of osLoader behind the same interfaces, and a loader that
// satisfied all of them but one would make the two impossible to swap.
func (l *localLoader) Convention(_ context.Context, _, ref string) (naming.Config, error) {
	return naming.Load(ref)
}

// DesignHash hashes the design's entry file for a stored run's provenance (WS9-053). It reuses the
// same hashSource the CLI's --results-out path uses, so a document written by `agni review` and one
// created through the service record identical revision identity for identical bytes. An unreadable
// file yields "" rather than an error, which is what DesignRef.content_hash documents for a producer
// that did not hash.
//
// It hashes the ENTRY the descriptor declares, not the ref the caller passed, so a run recorded
// against a companion and one recorded against the design folder carry the same revision identity:
// they analysed the same bytes.
func (l *localLoader) DesignHash(_ context.Context, _, ref string) (string, error) {
	src, err := l.resolve(ref)
	if err != nil {
		return "", err
	}
	return hashSource(src.Netlist), nil
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

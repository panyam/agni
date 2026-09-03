package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

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

// localLoader is the CLI's service.Loader: it resolves a bare LOCAL path with NO mount containment
// (the mount arg is ignored). Containment is a serve-only policy that lives in osLoader's
// mounts.Resolve; the CLI runs on the user's own files, so localLoader is the deliberate
// no-containment sibling behind the same Loader interface (WS9-048). It satisfies the full
// service.Loader (the check/query thin clients) plus Manifest (the review thin client), so a CLI
// command constructs its service over this and calls the same method the web serves.
type localLoader struct {
	loader   *formats.Loader
	resolver *designResolver
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
func (l *localLoader) resolve(ctx context.Context, path string) (designSource, error) {
	r := l.resolver
	if r == nil {
		ws, err := workspace()
		if err != nil {
			return designSource{}, err
		}
		r = newDesignResolver(ws)
	}
	src, err := r.Resolve(ctx, path)
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

func (l *localLoader) Design(ctx context.Context, uri artifact.URI, opts ...service.ReadOption) (*ir.Design, error) {
	src, err := l.resolve(ctx, localPath(uri))
	if err != nil {
		return nil, err
	}
	return readerFor(l.loader, opts...).ReadDesign(localOf(src.NetlistURI))
}

func (l *localLoader) Board(ctx context.Context, uri artifact.URI) (*geom.BoardGeometry, error) {
	src, err := l.resolve(ctx, localPath(uri))
	if err != nil {
		return nil, err
	}
	return l.loader.BoardGeometry(localOf(src.BoardURI))
}

func (l *localLoader) Geometry(ctx context.Context, uri artifact.URI, layout string, faithful bool, opts ...service.ReadOption) (*geom.SchematicGeometry, error) {
	src, err := l.resolve(ctx, localPath(uri))
	if err != nil {
		return nil, err
	}
	return readerFor(l.loader, opts...).ResolveGeometry(localOf(src.GeometryURI), layout, nil, symbolsFor(faithful))
}

func (l *localLoader) Report(ctx context.Context, uri artifact.URI, faithful bool, opts ...service.ReadOption) (*graph.ConversionReport, error) {
	src, err := l.resolve(ctx, localPath(uri))
	if err != nil {
		return nil, err
	}
	return readerFor(l.loader, opts...).ConversionReport(localOf(src.GeometryURI), symbolsFor(faithful), nil)
}

// Expectations reads the sidecar beside the ENTRY rather than beside the named file: an expectation
// set states what this design should read, and the design is its entry.
func (l *localLoader) Expectations(ctx context.Context, uri artifact.URI) (*expect.Expectations, error) {
	src, err := l.resolve(ctx, localPath(uri))
	if err != nil {
		return nil, err
	}
	sidecar := localOf(src.NetlistURI) + ".expect.yaml"
	if _, err := os.Stat(sidecar); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return expect.Load(sidecar)
}

func (l *localLoader) Manifest(_ context.Context, uri artifact.URI) (review.Manifest, error) {
	return loadManifest(localPath(uri))
}

// Convention resolves a naming-convention config from a local path. The CLI reads its own
// --conventions at the edge and sends the value, so nothing in the CLI calls this; it exists because
// localLoader is the no-containment sibling of osLoader behind the same interfaces, and a loader that
// satisfied all of them but one would make the two impossible to swap.
func (l *localLoader) Convention(_ context.Context, uri artifact.URI) (*configpb.NamingConvention, error) {
	return naming.Load(localPath(uri))
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
func (l *localLoader) DesignHash(ctx context.Context, uri artifact.URI) (string, error) {
	e, err := l.designEntry(ctx, uri)
	if err != nil {
		return "", err
	}
	return hashSource(localPath(e)), nil
}

// designEntry resolves the ONE artifact a read of this design actually opens: the descriptor's
// declared ENTRY when the caller named a design folder or a declared companion, and the named URI
// itself when the folder carries no descriptor.
//
// It is exported from DesignHash's body rather than left inside it because the ENTRY is what a
// verdict link has to be built from, not only what it has to be hashed from (agni issue 489). The
// link used to take its path from the caller's argument and its hash from here, so the two halves
// named different artifacts: a folder argument produced a path the viewer cannot open, and a
// companion argument produced a correct path with the entry's hash, which the viewer then reported
// as a revision mismatch on a design that was perfectly in sync.
func (l *localLoader) designEntry(ctx context.Context, uri artifact.URI) (artifact.URI, error) {
	src, err := l.resolve(ctx, localPath(uri))
	if err != nil {
		return artifact.URI{}, err
	}
	return artifact.Parse(src.NetlistURI)
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

// localPath turns an artifact URI back into the local path the CLI's readers take.
//
// This is the ONLY place the CLI unpacks a URI, and it is deliberately at the port boundary: above
// it every caller holds one contained value, below it the readers see the plain names they have
// always seen. It resolves through the run's mount table exactly as the served adapter does, so the
// CLI is no longer the "no containment" sibling it used to be: every path it reads is inside a mount,
// even when that mount was minted from the argument.
func localPath(uri artifact.URI) string {
	ws, err := workspace()
	if err != nil {
		return filepath.FromSlash(uri.Path)
	}
	abs, err := mounts.Resolve(ws.Mounts(), uri)
	if err != nil {
		return filepath.FromSlash(uri.Path)
	}
	return abs
}

// localOf resolves an artifact URI string to a local path, for the direct readers that never go
// through a port. It is localPath with the parse folded in, and it passes a non-URI through
// unchanged so a caller holding a plain path is not broken by it.
func localOf(s string) string {
	u, err := artifact.Parse(s)
	if err != nil {
		return s
	}
	return localPath(u)
}

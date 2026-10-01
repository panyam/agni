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
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/readers/formats"
	"github.com/panyam/agni/service"
)

// localLoader is the CLI's service.Loader, osLoader's sibling behind the same interfaces (WS9-048).
// It resolves a URI to a local path through the run's mount table (see localPath) and runs it
// through the enclosing design's descriptor. It satisfies the full service.Loader (the check and
// query thin clients) plus Manifest (the review thin client), so a CLI command constructs its
// service over this and calls the same method the web serves.
type localLoader struct {
	loader   *formats.Loader
	resolver *designResolver
	// notes is where a descriptor-resolution note is written, nil for os.Stderr. Notes are emitted
	// once per named path (noted), because one BuildModel asks this loader for the netlist, the board,
	// and the geometry of the SAME path.
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
	l.note(path, src.Note)
	return src, nil
}

// note writes text to the notes writer the first time key is seen, and never when text is empty.
func (l *localLoader) note(key, text string) {
	if text == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.noted[key] {
		return
	}
	if l.noted == nil {
		l.noted = map[string]bool{}
	}
	l.noted[key] = true
	w := l.notes
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprint(w, text)
}

// Design reads the netlist tier and says, once per file, when the read left out hierarchical blocks,
// so check, query, review and trace warn the way readDesign's commands do (agni issue 707).
func (l *localLoader) Design(ctx context.Context, uri artifact.URI, opts ...service.ReadOption) (*ir.Design, error) {
	src, err := l.resolve(ctx, localPath(uri))
	if err != nil {
		return nil, err
	}
	netlist := localOf(src.NetlistURI)
	d, err := readerFor(l.loader, opts...).ReadDesign(netlist)
	if err != nil {
		return nil, err
	}
	l.note("hierarchy:"+netlist, hierarchyNote(netlist, d.GetInputDiagnostics().GetUnexpandedHierarchy()))
	return d, nil
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

// Convention resolves a naming-convention config from a local path. Nothing in the CLI calls it,
// since the CLI reads its own --conventions at the edge and sends the value. It exists so localLoader
// and osLoader satisfy the same interfaces and stay swappable.
func (l *localLoader) Convention(_ context.Context, uri artifact.URI) (*configpb.NamingConvention, error) {
	return naming.Load(localPath(uri))
}

// DesignHash hashes the design's entry file for a stored run's provenance (WS9-053). It uses the
// same hashSource as the CLI's --results-out path, so `agni review` and the service record the same
// revision identity for the same bytes. An unreadable file yields "" rather than an error (see
// DesignRef.content_hash).
//
// It hashes the ENTRY the descriptor declares, not the ref the caller passed, so a run recorded
// against a companion and one recorded against the design folder carry the same revision identity.
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
// A verdict link takes both its path and its hash from this ENTRY, so the two halves name the same
// artifact (agni issue 489). A link built from the caller's argument instead would open nothing for
// a folder argument and report a false revision mismatch for a companion.
func (l *localLoader) designEntry(ctx context.Context, uri artifact.URI) (artifact.URI, error) {
	src, err := l.resolve(ctx, localPath(uri))
	if err != nil {
		return artifact.URI{}, err
	}
	return artifact.Parse(src.NetlistURI)
}

// loadManifest reads and validates a checklist from a local path. `agni review` calls it directly
// and sends the checklist to the service as a VALUE (WS9-050), while the Manifest method uses it for
// GetReviewManifest, which serves a client holding a ref. Both share this one read so they agree
// on what a well-formed manifest is.
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
// This is the ONLY place the CLI unpacks a URI, at the port boundary, so every caller above it holds
// one contained value and the readers below it see plain paths. It resolves through the run's mount
// table as the served adapter does, so every path the CLI reads is inside a mount, even when that
// mount was minted from the argument (#179). A URI that does not resolve falls back to its path.
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

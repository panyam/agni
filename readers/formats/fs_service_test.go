// Package formats_test holds the tests that reach ABOVE the formats package. The service tier
// imports formats, so a test proving formats can back a service.Loader has to live in the external
// test package or the import cycle is unbuildable.
package formats_test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/graph"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/artifact"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/readers/formats"
)

// mountFS mirrors the helper in the internal test file: fixtures loaded from disk but mounted
// under FS names that do NOT resolve on the host, so a read that escapes to os fails outright.
func mountFS(t *testing.T, files map[string]string) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	for name, disk := range files {
		data, err := os.ReadFile(disk)
		if err != nil {
			t.Fatalf("fixture %s: %v", disk, err)
		}
		if _, err := os.Stat(name); err == nil {
			t.Fatalf("FS name %q also exists on disk; pick a name that cannot resolve via os", name)
		}
		m[name] = &fstest.MapFile{Data: data}
	}
	return m
}

// memLoader is a service.Loader backed entirely by an in-memory fs.FS — the shape a WASM build or
// an embedder would inject where the CLI injects its os-backed one. It is the ticket's real
// acceptance criterion, and the point is what it does NOT import: no os, no path resolution
// against a host root, just a mount name selecting an FS. Expectations returns (nil, nil) because
// an in-memory host carries no `.expect.yaml` sidecars; absence is the normal case there, as it is
// on disk.
type memLoader struct{ mounts map[string]fs.FS }

func (m *memLoader) loader(mount string, opts ...service.ReadOption) (*formats.Loader, error) {
	fsys, ok := m.mounts[mount]
	if !ok {
		return nil, fmt.Errorf("no such mount %q", mount)
	}
	return &formats.Loader{FS: fsys, Lexicon: service.ReadOpts(opts...).Lexicon}, nil
}

func (m *memLoader) Design(_ context.Context, uri artifact.URI, opts ...service.ReadOption) (*ir.Design, error) {
	mount, p := uri.Mount, uri.Path
	l, err := m.loader(mount, opts...)
	if err != nil {
		return nil, err
	}
	return l.ReadDesign(p)
}

func (m *memLoader) Geometry(_ context.Context, uri artifact.URI, layout string, faithfulSymbols bool, _ ...service.ReadOption) (*geom.SchematicGeometry, error) {
	mount, p := uri.Mount, uri.Path
	l, err := m.loader(mount)
	if err != nil {
		return nil, err
	}
	symbols := formats.SymbolsGlyph
	if faithfulSymbols {
		symbols = formats.SymbolsFaithful
	}
	return l.ResolveGeometry(p, layout, nil, symbols)
}

func (m *memLoader) Report(_ context.Context, uri artifact.URI, faithfulSymbols bool, _ ...service.ReadOption) (*graph.ConversionReport, error) {
	mount, p := uri.Mount, uri.Path
	l, err := m.loader(mount)
	if err != nil {
		return nil, err
	}
	symbols := formats.SymbolsGlyph
	if faithfulSymbols {
		symbols = formats.SymbolsFaithful
	}
	return l.ConversionReport(p, symbols, nil)
}

func (m *memLoader) Expectations(context.Context, artifact.URI) (*expect.Expectations, error) {
	return nil, nil
}

func (m *memLoader) Board(_ context.Context, uri artifact.URI) (*geom.BoardGeometry, error) {
	mount, p := uri.Mount, uri.Path
	l, err := m.loader(mount)
	if err != nil {
		return nil, err
	}
	return l.BoardGeometry(p)
}

var _ service.Loader = (*memLoader)(nil)

// TestInMemoryServiceLoader is the done-when from WS1-049: the service tier's Loader port can be
// satisfied by a host with no filesystem, reusing the engine's dispatch and stamps rather than
// re-implementing them. Before the FS seam, memLoader could not exist — the only way in was a host
// path, so an in-memory host would have had to call each reader directly and copy the post-read
// pass sequence, which is the drift bug the single entry point prevents.
func TestInMemoryServiceLoader(t *testing.T) {
	l := &memLoader{mounts: map[string]fs.FS{
		"designs": mountFS(t, map[string]string{
			"boards/basic.edn":       "../edif/testdata/basic.edn",
			"boards/board.kicad_pcb": "../kicad/testdata/board.kicad_pcb",
		}),
	}}
	ctx := context.Background()

	d, err := l.Design(ctx, uriOf(t, "designs", "boards/basic.edn"))
	if err != nil || len(d.Components) == 0 {
		t.Fatalf("Design over an in-memory mount = (%v components, %v), want a read design", len(d.GetComponents()), err)
	}
	// The per-request lexicon override must survive the FS: readerFor-style copying carries the
	// whole Loader, so a served request can set project conventions without losing its host.
	lex := classify.DefaultLexicon()
	if _, err := l.Design(ctx, uriOf(t, "designs", "boards/basic.edn"), service.WithLexicon(lex)); err != nil {
		t.Errorf("Design with a lexicon option over an FS mount: %v", err)
	}
	if g, err := l.Geometry(ctx, uriOf(t, "designs", "boards/basic.edn"), "grid", false); err != nil || len(g.Sheets) == 0 {
		t.Errorf("Geometry over an in-memory mount = (%v sheets, %v), want an auto-layout", len(g.GetSheets()), err)
	}
	if r, err := l.Report(ctx, uriOf(t, "designs", "boards/basic.edn"), false); err != nil || r == nil {
		t.Errorf("Report over an in-memory mount = (%v, %v)", r, err)
	}
	if b, err := l.Board(ctx, uriOf(t, "designs", "boards/board.kicad_pcb")); err != nil || b == nil {
		t.Errorf("Board over an in-memory mount = (%v, %v)", b, err)
	}
	if _, err := l.Design(ctx, uriOf(t, "nosuch", "boards/basic.edn")); err == nil {
		t.Error("an unknown mount resolved; want an error")
	}
}

// uriOf builds an artifact URI for a test, failing rather than returning an error: a hard-coded
// fixture URI that will not parse is a broken test, not a condition under test.
func uriOf(t *testing.T, mount, p string) artifact.URI {
	t.Helper()
	u, err := artifact.New(mount, p)
	if err != nil {
		t.Fatalf("artifact.New(%q, %q): %v", mount, p, err)
	}
	return u
}

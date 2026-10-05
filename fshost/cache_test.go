package fshost

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/param"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

// hierCopy copies the KiCad hierarchy fixture (a root and the child sheet it instantiates twice) into
// a temp folder and returns a host over it, served from the real filesystem so a stamp sees real
// modification times.
func hierCopy(t *testing.T) (string, *service.CachingLoader, *service.DesignCache) {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"hier_root.kicad_sch", "hier_child.kicad_sch"} {
		b, err := os.ReadFile(filepath.Join("..", "readers", "kicad", "testdata", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := service.NewDesignCache(8)
	return dir, service.NewCachingLoader(New(Mount{Name: "m", FS: os.DirFS(dir)}), c), c
}

func netNames(t *testing.T, l *service.CachingLoader, opts ...service.ReadOption) map[string]bool {
	t.Helper()
	d, err := l.Design(context.Background(), artifact.URI{Mount: "m", Path: "hier_root.kicad_sch"}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, n := range d.GetNets() {
		out[n.GetName()] = true
	}
	if len(out) == 0 {
		t.Fatal("the fixture read with no nets, so nothing below can tell a stale read from a fresh one")
	}
	return out
}

func hasSuffix(names map[string]bool, suffix string) bool {
	for n := range names {
		if len(n) >= len(suffix) && n[len(n)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

// TestASecondReadIsServedFromTheCache is the point of agni issue 895: the same design asked twice is
// read once.
func TestASecondReadIsServedFromTheCache(t *testing.T) {
	_, l, c := hierCopy(t)
	netNames(t, l)
	netNames(t, l)
	if hits, misses := c.Stats(); hits != 1 || misses != 1 {
		t.Fatalf("hits %d misses %d, want one read and one hit", hits, misses)
	}
}

// TestEditingASubSheetReadsTheDesignAgain edits the CHILD sheet, a file the request never names.
// Keying on the entry file's hash would serve the old nets; the read's recorded files catch it.
func TestEditingASubSheetReadsTheDesignAgain(t *testing.T) {
	dir, l, c := hierCopy(t)
	if !hasSuffix(netNames(t, l), "SIG") {
		t.Fatal("the child sheet's SIG label is not in the read")
	}
	child := filepath.Join(dir, "hier_child.kicad_sch")
	b, _ := os.ReadFile(child)
	if err := os.WriteFile(child, bytes.ReplaceAll(b, []byte(`(label "SIG"`), []byte(`(label "SIGNAL_RENAMED"`)), 0o644); err != nil {
		t.Fatal(err)
	}
	// Some filesystems stamp to the second; move the time on so the edit is visible however coarse.
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(child, later, later)

	names := netNames(t, l)
	if !hasSuffix(names, "SIGNAL_RENAMED") || hasSuffix(names, "/SIG") {
		t.Fatalf("the edited sub-sheet was not read again: %v", names)
	}
	if _, misses := c.Stats(); misses != 2 {
		t.Fatalf("misses %d, want 2 (the first read and the read after the edit)", misses)
	}
}

// TestOptionsWithNoIdentityAreNeverCached: a lexicon passed without the overlay's identity could be
// anything, so two reads under it are two reads.
func TestOptionsWithNoIdentityAreNeverCached(t *testing.T) {
	_, l, c := hierCopy(t)
	opt := service.WithLexicon(classify.DefaultLexicon())
	netNames(t, l, opt)
	netNames(t, l, opt)
	if hits, _ := c.Stats(); hits != 0 {
		t.Fatalf("hits %d: a read whose options carry no identity was served from the cache", hits)
	}
	// And an identity that does not cover every option is not one.
	partial := []service.ReadOption{service.WithIdentity("id", 0), opt}
	netNames(t, l, partial...)
	netNames(t, l, partial...)
	if hits, _ := c.Stats(); hits != 0 {
		t.Fatalf("hits %d: an identity covering fewer options than the read applied was used", hits)
	}
}

// TestACopyIsHandedOut: a caller that stamps into the design it was handed (check.NewModel's
// datasheet pass does) must not change what the next caller reads.
func TestACopyIsHandedOut(t *testing.T) {
	_, l, _ := hierCopy(t)
	u := artifact.URI{Mount: "m", Path: "hier_root.kicad_sch"}
	d1, err := l.Design(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	d1.Nets = nil
	d2, err := l.Design(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(d2.GetNets()) == 0 {
		t.Fatal("a caller's edit to the design it was handed reached the cache")
	}
}

// TestAModelIsKeptWhenTheCorpusIsLocal builds a project design's model twice with a local shared
// corpus, as `agni serve --params` has. SpecsOver wraps every corpus in a Layered, which implements
// Prefetch, and the cache used to read that as a remote corpus and rebuild the model and its fact
// base on every request: about 250 ms per query on a 3,980-component board.
func TestAModelIsKeptWhenTheCorpusIsLocal(t *testing.T) {
	ms := []Mount{{Name: "m", FS: os.DirFS(filepath.Join("..", "examples", "tutorial-project"))}}
	trees := []projects.Tree{{Mount: "m", FS: ms[0].FS}}
	res := &service.ProjectResolver{Store: projects.NewFSStore(trees...), Config: &ConfigResolver{Mounts: ms}}
	loader := service.NewCachingLoader(New(ms...), service.NewDesignCache(8))
	ctx := context.Background()
	u := artifact.URI{Mount: "m", Path: "designs/gateway"}
	ov, err := res.Overlay(ctx, u, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ov.Identity(); !ok {
		t.Fatal("the tutorial project's overlay does not identify, so no model could be kept and this proves nothing")
	}
	nu, bu, _, err := res.TierURIs(ctx, u, artifact.URI{}, false)
	if err != nil {
		t.Fatal(err)
	}
	shared := param.ParamSet{}
	first, _, err := service.BuildModelCached(ctx, loader, nu, bu, ov, shared)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := service.BuildModelCached(ctx, loader, nu, bu, ov, shared)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("the second request rebuilt the model, so a local corpus was treated as one fetched over the network")
	}
}

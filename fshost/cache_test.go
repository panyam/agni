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
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/proto"
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
// read once. A read misses twice, at the parse and at the design derived from it (agni issue 1061),
// and the second ask hits the derived design without reaching the parse.
func TestASecondReadIsServedFromTheCache(t *testing.T) {
	_, l, c := hierCopy(t)
	netNames(t, l)
	netNames(t, l)
	if hits, misses := c.Stats(); hits != 1 || misses != 2 {
		t.Fatalf("hits %d misses %d, want one read (the parse and the derive) and one hit", hits, misses)
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
	if !hasSuffix(names, "SIGNAL_RENAMED") || hasSuffix(names, "/amp1/SIG") {
		t.Fatalf("the edited sub-sheet was not read again: %v", names)
	}
	if _, misses := c.Stats(); misses != 4 {
		t.Fatalf("misses %d, want 4 (the first read and the read after the edit, each at the parse and the design)", misses)
	}
}

// TestOptionsWithNoIdentityAreNeverCached: a lexicon passed without the overlay's identity could be
// anything, so two reads under it derive twice. The parse reads no options, so every read after the
// first hits it, and nothing else (agni issue 1061).
func TestOptionsWithNoIdentityAreNeverCached(t *testing.T) {
	_, l, c := hierCopy(t)
	opt := service.WithLexicon(classify.DefaultLexicon())
	netNames(t, l, opt)
	netNames(t, l, opt)
	if hits, misses := c.Stats(); hits != 1 || misses != 1 {
		t.Fatalf("hits %d misses %d: want the parse read once and hit once, and no derived design kept", hits, misses)
	}
	// And an identity that does not cover every option is not one.
	partial := []service.ReadOption{service.WithIdentity("id", 0), opt}
	netNames(t, l, partial...)
	netNames(t, l, partial...)
	if hits, misses := c.Stats(); hits != 3 || misses != 1 {
		t.Fatalf("hits %d misses %d: an identity covering fewer options than the read applied was used", hits, misses)
	}
}

// TestAnotherConventionDerivesFromTheSameParse is agni issue 1061's step 3: the parse is keyed on no
// configuration, so a second overlay pays the derive and not the read, and its design is the one an
// uncached read under it gives. control: an edited file reads the parse again.
func TestAnotherConventionDerivesFromTheSameParse(t *testing.T) {
	dir, l, c := hierCopy(t)
	u := artifact.URI{Mount: "m", Path: "hier_root.kicad_sch"}
	a := []service.ReadOption{service.WithIdentity("a", 0)}
	// A vocabulary naming no role at all, so a design derived under the wrong one cannot pass.
	lex := &classify.Lexicon{Role: &classify.RoleVocab{}, Class: classify.DefaultClassVocab(), Value: classify.DefaultValueVocab()}
	b := []service.ReadOption{service.WithLexicon(lex), service.WithIdentity("b", 1)}
	da, err := l.Design(context.Background(), u, a...)
	if err != nil {
		t.Fatal(err)
	}
	if _, misses := c.Stats(); misses != 2 {
		t.Fatalf("misses %d, want the parse and the first derive", misses)
	}
	db, err := l.Design(context.Background(), u, b...)
	if err != nil {
		t.Fatal(err)
	}
	if hits, misses := c.Stats(); hits != 1 || misses != 3 {
		t.Fatalf("hits %d misses %d: the second overlay read the files again instead of deriving from the parse", hits, misses)
	}
	if roled(da) == 0 {
		t.Fatal("the default vocabulary gave no net a role, so the other vocabulary's design cannot differ")
	}
	if n := roled(db); n != 0 {
		t.Errorf("%d nets carry a role under a vocabulary naming none, so the derive read the first overlay's design", n)
	}
	uncached, err := New(Mount{Name: "m", FS: os.DirFS(dir)}).Design(context.Background(), u, b...)
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(db, uncached) {
		t.Error("the design derived from the kept parse differs from an uncached read under the same options")
	}
	child := filepath.Join(dir, "hier_child.kicad_sch")
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(child, later, later)
	netNames(t, l, b...)
	if hits, misses := c.Stats(); hits != 1 || misses != 5 {
		t.Fatalf("hits %d misses %d: after an edit, want the parse read again and derived again", hits, misses)
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

func roled(d *ir.Design) int {
	n := 0
	for _, net := range d.GetNets() {
		if len(net.GetRoles()) > 0 {
			n++
		}
	}
	return n
}

// TestAfterLoadRunsOncePerRead covers agni issue 1000: the browser engine collects after each layer
// the cache reads, and a hit, which made nothing, collects nothing.
func TestAfterLoadRunsOncePerRead(t *testing.T) {
	_, l, c := hierCopy(t)
	loads := 0
	c.AfterLoad(func() { loads++ })
	netNames(t, l)
	netNames(t, l)
	if _, misses := c.Stats(); loads != int(misses) || loads != 2 {
		t.Fatalf("AfterLoad ran %d times over %d levels read, want once per level (the parse and the design)", loads, misses)
	}
}

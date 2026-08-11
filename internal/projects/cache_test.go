package projects

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The cache is only allowed to exist because it cannot go stale. These tests are that claim, one
// edit at a time: an operator changes something while the server runs, and the very next request has
// to see it. A cache that failed any of these would reintroduce the exact failure this workstream
// was built to remove — a confident answer computed from config the operator has already fixed.

// diskTree writes a project tree to a real directory and returns a store over it, plus the root.
//
// A real filesystem rather than fstest.MapFS, because mtime is the entire mechanism under test and a
// synthetic FS's timestamps prove nothing about the one the server will run on.
func diskTree(t *testing.T) (*FSStore, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "proj/project.yaml", "name: gw\ntitle: Gateway\n")
	writeFile(t, root, "proj/conventions.yaml", "name: gw\nlexicon:\n  rail:\n    patterns: [\"^OLD\"]\n")
	writeFile(t, root, "proj/designs/board/design.yaml", "name: board\nentry: board.edn\n")
	writeFile(t, root, "proj/designs/board/board.edn", "x")
	return NewFSStore(Tree{Mount: "m", FS: os.DirFS(root)}), root
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// Filesystems record mtime at limited resolution, and a test writes far faster than that. Nudging
	// the timestamp forward keeps the test measuring the CACHE rather than the clock's granularity;
	// an operator editing a file by hand never hits this.
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(full, future, future); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Dir(full), future, future); err != nil {
		t.Fatal(err)
	}
}

// TestEditedDescriptorIsSeenImmediately: the entry a design declares is what analysis reads, so
// serving the previous one after an operator fixed it would send every later run at the wrong file.
func TestEditedDescriptorIsSeenImmediately(t *testing.T) {
	s, root := diskTree(t)
	ctx := context.Background()

	ds, err := s.Designs(ctx, "projects/gw")
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].GetEntryUri() != "mount://m/proj/designs/board/board.edn" {
		t.Fatalf("designs = %+v", ds)
	}

	writeFile(t, root, "proj/designs/board/design.yaml", "name: board\nentry: fixed.edn\n")
	ds, err = s.Designs(ctx, "projects/gw")
	if err != nil {
		t.Fatal(err)
	}
	if ds[0].GetEntryUri() != "mount://m/proj/designs/board/fixed.edn" {
		t.Errorf("entry after edit = %q, want the edited one", ds[0].GetEntryUri())
	}
}

// TestEditedConventionsIsSeenImmediately is the edit whose staleness is least visible: the rules
// still run, they just run under the vocabulary from before the fix. Nothing in a findings list says
// so, which is why the cache keys on every file a load reads rather than on the descriptor alone.
func TestEditedConventionsIsSeenImmediately(t *testing.T) {
	s, root := diskTree(t)
	ctx := context.Background()

	p, err := s.Project(ctx, "projects/gw")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.GetConventions().GetLexicon().GetRail().GetPatterns(); len(got) != 1 || got[0] != "^OLD" {
		t.Fatalf("rail patterns = %v", got)
	}

	writeFile(t, root, "proj/conventions.yaml", "name: gw\nlexicon:\n  rail:\n    patterns: [\"^NEW\"]\n")
	p, err = s.Project(ctx, "projects/gw")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.GetConventions().GetLexicon().GetRail().GetPatterns(); len(got) != 1 || got[0] != "^NEW" {
		t.Errorf("rail patterns after edit = %v, want the edited vocabulary", got)
	}
}

// TestNewProjectIsSeenImmediately: discovery is keyed on directory mtimes precisely so a descriptor
// appearing is a change the cache notices. A remembered walk would hide a project until restart.
func TestNewProjectIsSeenImmediately(t *testing.T) {
	s, root := diskTree(t)
	ctx := context.Background()

	if ps, err := s.Projects(ctx); err != nil || len(ps) != 1 {
		t.Fatalf("projects = %+v, %v", ps, err)
	}
	writeFile(t, root, "second/project.yaml", "name: second\n")
	ps, err := s.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Errorf("projects after adding one = %d, want both", len(ps))
	}
}

// TestNewDesignIsSeenImmediately is the same property one level down, and it is a different code
// path: designs are discovered under a project's own folder rather than from the tree root.
func TestNewDesignIsSeenImmediately(t *testing.T) {
	s, root := diskTree(t)
	ctx := context.Background()

	if ds, err := s.Designs(ctx, "projects/gw"); err != nil || len(ds) != 1 {
		t.Fatalf("designs = %+v, %v", ds, err)
	}
	writeFile(t, root, "proj/designs/second/design.yaml", "name: second\nentry: b.edn\n")
	ds, err := s.Designs(ctx, "projects/gw")
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 2 {
		t.Errorf("designs after adding one = %d, want both", len(ds))
	}
}

// TestRemovedProjectIsSeenImmediately: a deleted descriptor has to disappear as readily as an added
// one appears. A cache that only noticed additions would keep serving a project that is gone, and a
// client would get NotFound from every call against a name the listing still advertised.
func TestRemovedProjectIsSeenImmediately(t *testing.T) {
	s, root := diskTree(t)
	ctx := context.Background()

	writeFile(t, root, "second/project.yaml", "name: second\n")
	if ps, err := s.Projects(ctx); err != nil || len(ps) != 2 {
		t.Fatalf("projects = %+v, %v", ps, err)
	}

	if err := os.RemoveAll(filepath.Join(root, "second")); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(root, future, future); err != nil {
		t.Fatal(err)
	}
	ps, err := s.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Errorf("projects after removing one = %d, want just the survivor", len(ps))
	}
}

// TestCachedValuesAreNotAliased: the store MUTATES what it loads, filling in resource names and
// rewriting descriptor-relative refs into URIs. Handing out the cached message by pointer would let
// one request's fill-in become the next request's starting point, and the second call would join a
// URI onto a URI.
func TestCachedValuesAreNotAliased(t *testing.T) {
	s, _ := diskTree(t)
	ctx := context.Background()

	first, err := s.Designs(ctx, "projects/gw")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Designs(ctx, "projects/gw")
	if err != nil {
		t.Fatal(err)
	}
	if first[0] == second[0] {
		t.Fatal("two loads returned the same pointer; a caller mutating one would corrupt the cache")
	}
	if first[0].GetEntryUri() != second[0].GetEntryUri() {
		t.Errorf("entry drifted between loads: %q then %q", first[0].GetEntryUri(), second[0].GetEntryUri())
	}
}

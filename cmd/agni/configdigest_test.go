package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func digestOf(t *testing.T, paths []string, names []string) string {
	t.Helper()
	d, err := digestConfig(paths, names)
	if err != nil {
		t.Fatalf("digestConfig: %v", err)
	}
	return d
}

// Equal trees digest alike whatever order the walk happened to visit them in, or the digest would
// change on its own and the overlay identity built from it would never match twice.
func TestConfigDigestIsStableAcrossEqualTrees(t *testing.T) {
	files := map[string]string{"a.yaml": "one", "sub/b.yaml": "two", "sub/c.yaml": "three"}
	a := digestOf(t, []string{writeTree(t, files)}, nil)
	b := digestOf(t, []string{writeTree(t, files)}, nil)
	if a != b {
		t.Errorf("two identical trees digested %q and %q", a, b)
	}
}

// CONTENT, not stat metadata. A file restored from a copy keeps its size and can keep its mtime,
// which is a state a developer reaches by switching branches.
func TestConfigDigestFollowsContent(t *testing.T) {
	base := map[string]string{"a.yaml": "one"}
	changed := map[string]string{"a.yaml": "ONE"}
	if digestOf(t, []string{writeTree(t, base)}, nil) == digestOf(t, []string{writeTree(t, changed)}, nil) {
		t.Error("a changed file left the digest alone, so a run would reuse work done under different config")
	}
}

// The same bytes under a different name are different config: a profile set is addressed by file.
func TestConfigDigestFollowsNames(t *testing.T) {
	a := digestOf(t, []string{writeTree(t, map[string]string{"a.yaml": "same"})}, nil)
	b := digestOf(t, []string{writeTree(t, map[string]string{"b.yaml": "same"})}, nil)
	if a == b {
		t.Error("renaming a file left the digest alone")
	}
}

// Symbol paths are URIs this resolver never opens. Statting one is an error rather than a digest,
// which is why they fold in as names and not as paths.
func TestConfigDigestFoldsUnreadNamesWithoutStattingThem(t *testing.T) {
	dir := writeTree(t, map[string]string{"a.yaml": "one"})
	withLib, err := digestConfig([]string{dir}, []string{"mount://sym/lib"})
	if err != nil {
		t.Fatalf("a symbol library URI must not be statted: %v", err)
	}
	if withLib == digestOf(t, []string{dir}, nil) {
		t.Error("naming a symbol library left the digest alone, so two runs searching different libraries identify alike")
	}
	if other, _ := digestConfig([]string{dir}, []string{"mount://sym/OTHER"}); other == withLib {
		t.Error("two different symbol libraries digested alike")
	}
}

// A resolution that read nothing still has an answer. Empty means "this resolver reports no digest",
// which service.Overlay.Identity treats as a refusal, so the two must not collide.
func TestConfigDigestOfNothingIsAValue(t *testing.T) {
	d, err := digestConfig(nil, nil)
	if err != nil {
		t.Fatalf("digestConfig(nil, nil): %v", err)
	}
	if d == "" {
		t.Error("a resolution that read nothing reported no digest, which reads as unidentifiable")
	}
}

// Two roots holding one file each must not digest as one root holding both. Without a boundary
// between roots the byte streams are identical, which would let a run with profiles and params in
// separate directories identify as one with them combined.
func TestConfigDigestKeepsRootsApart(t *testing.T) {
	split := digestOf(t, []string{
		writeTree(t, map[string]string{"a.yaml": "one"}),
		writeTree(t, map[string]string{"b.yaml": "two"}),
	}, nil)
	together := digestOf(t, []string{writeTree(t, map[string]string{"a.yaml": "one", "b.yaml": "two"})}, nil)
	if split == together {
		t.Error("two roots digested the same as one root holding both files")
	}
}

// The root's own name is not part of the config. The same profile set mounted at two paths is the
// same config, and a digest that said otherwise would never match twice across deployments.
func TestConfigDigestIgnoresTheRootsOwnName(t *testing.T) {
	files := map[string]string{"a.yaml": "one"}
	outer := t.TempDir()
	var digests []string
	for _, name := range []string{"profiles", "PROFILES-RENAMED"} {
		root := filepath.Join(outer, name)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		for f, body := range files {
			if err := os.WriteFile(filepath.Join(root, f), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		digests = append(digests, digestOf(t, []string{root}, nil))
	}
	if digests[0] != digests[1] {
		t.Errorf("the same config under two directory names digested %q and %q", digests[0], digests[1])
	}
}

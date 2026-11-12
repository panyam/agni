package mounts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMounts(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("valid", func(t *testing.T) {
		mounts, err := Parse([]string{"a=" + dir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mounts) != 1 || mounts[0].Name != "a" || mounts[0].Root != dir {
			t.Fatalf("got %+v", mounts)
		}
	})

	t.Run("order preserved", func(t *testing.T) {
		mounts, err := Parse([]string{"a=" + dir, "b=" + dir})
		if err != nil {
			t.Fatal(err)
		}
		if len(mounts) != 2 || mounts[0].Name != "a" || mounts[1].Name != "b" {
			t.Fatalf("order not preserved: %+v", mounts)
		}
	})

	for _, tc := range []struct{ name, spec string }{
		{"no equals", "abc"},
		{"empty name", "=" + dir},
		{"empty path", "a="},
		{"path is file", "a=" + file},
		{"path missing", "a=" + filepath.Join(dir, "nope")},
	} {
		t.Run("reject "+tc.name, func(t *testing.T) {
			if _, err := Parse([]string{tc.spec}); err == nil {
				t.Fatalf("expected error for %q", tc.spec)
			}
		})
	}

	t.Run("reject duplicate name", func(t *testing.T) {
		if _, err := Parse([]string{"a=" + dir, "a=" + dir}); err == nil {
			t.Fatal("expected duplicate-name error")
		}
	})
}

func TestSafeResolve(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "srv", "mount")

	ok := []struct{ rel, want string }{
		{"", root},
		{".", root},
		{"sub/file.edn", filepath.Join(root, "sub", "file.edn")},
		{"a/../b", filepath.Join(root, "b")},
	}
	for _, tc := range ok {
		got, err := SafeResolve(root, tc.rel)
		if err != nil {
			t.Fatalf("SafeResolve(%q) errored: %v", tc.rel, err)
		}
		if got != tc.want {
			t.Fatalf("SafeResolve(%q) = %q, want %q", tc.rel, got, tc.want)
		}
	}

	bad := []string{"..", "../etc", "a/../../etc", filepath.Join(root, "abs")}
	for _, rel := range bad {
		if _, err := SafeResolve(root, rel); err == nil {
			t.Fatalf("SafeResolve(%q) should have been rejected", rel)
		}
	}
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"boards", "datasheets", ".git"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "loose.kicad_sch"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("subdirectories become mounts, sorted", func(t *testing.T) {
		got, err := Discover(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Name != "boards" || got[1].Name != "datasheets" {
			t.Fatalf("got %+v, want boards then datasheets", got)
		}
		if got[0].Root != filepath.Join(root, "boards") {
			t.Fatalf("root not joined: %+v", got[0])
		}
	})

	// A stray .git or .DS_Store in a bind-mounted parent is not a design folder, and a loose
	// file at the root is addressed through its own mount, not by becoming one.
	t.Run("skips dotfiles and non-directories", func(t *testing.T) {
		got, err := Discover(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range got {
			if m.Name == ".git" || m.Name == "loose.kicad_sch" {
				t.Fatalf("should have been skipped: %+v", got)
			}
		}
	})

	// The container always passes --mount-root; an operator who bind-mounts nothing must still
	// get a running server with an empty tree, not a startup failure.
	t.Run("missing root is empty, not an error", func(t *testing.T) {
		got, err := Discover(filepath.Join(root, "nope"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %+v, want none", got)
		}
	})

	t.Run("root that is a file is an error", func(t *testing.T) {
		if _, err := Discover(filepath.Join(root, "loose.kicad_sch")); err == nil {
			t.Fatal("expected an error for a file mount root")
		}
	})

	t.Run("empty root directory yields no mounts", func(t *testing.T) {
		got, err := Discover(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("got %+v, want none", got)
		}
	})
}

func TestMerge(t *testing.T) {
	d := []Mount{{Name: "boards", Root: "/w/boards"}, {Name: "ds", Root: "/w/ds"}}

	t.Run("explicit wins a name collision", func(t *testing.T) {
		got := Merge(d, []Mount{{Name: "boards", Root: "/elsewhere"}})
		if len(got) != 2 {
			t.Fatalf("got %+v, want 2", got)
		}
		for _, m := range got {
			if m.Name == "boards" && m.Root != "/elsewhere" {
				t.Fatalf("discovered mount shadowed the explicit one: %+v", got)
			}
		}
	})

	t.Run("non-colliding mounts all survive", func(t *testing.T) {
		got := Merge(d, []Mount{{Name: "extra", Root: "/x"}})
		if len(got) != 3 {
			t.Fatalf("got %+v, want 3", got)
		}
	})

	t.Run("either side empty", func(t *testing.T) {
		if got := Merge(d, nil); len(got) != 2 {
			t.Fatalf("got %+v, want the discovered set", got)
		}
		if got := Merge(nil, d); len(got) != 2 {
			t.Fatalf("got %+v, want the explicit set", got)
		}
	})
}

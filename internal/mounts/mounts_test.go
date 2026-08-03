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

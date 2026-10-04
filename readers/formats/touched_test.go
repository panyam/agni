package formats

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func touchedOver(t *testing.T) (string, *Loader, *Touched) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := NewTouched()
	return dir, &Loader{FS: os.DirFS(dir), Touched: rec}, rec
}

func TestAnEmptyRecorderIsNeverUnchanged(t *testing.T) {
	if NewTouched().Unchanged() {
		t.Fatal("a recorder that saw nothing claims nothing changed, so a read around the Loader would be cached")
	}
}

func TestAReadFileIsStampedAndAnEditIsSeen(t *testing.T) {
	dir, l, rec := touchedOver(t)
	if _, err := l.ReadFile("a.txt"); err != nil {
		t.Fatal(err)
	}
	if !rec.Unchanged() {
		t.Fatal("nothing changed and the recorder says something did")
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("ab"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec.Unchanged() {
		t.Fatal("an edited file reads as unchanged")
	}
}

// A sibling looked for and missing is part of what the read depended on.
func TestAMissingFileThatAppearsIsAChange(t *testing.T) {
	dir, l, rec := touchedOver(t)
	if _, err := l.Open("sidecar.kicad_pcb"); err == nil {
		t.Fatal("the fixture has no sidecar")
	}
	if !rec.Unchanged() {
		t.Fatal("nothing changed and the recorder says something did")
	}
	if err := os.WriteFile(filepath.Join(dir, "sidecar.kicad_pcb"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec.Unchanged() {
		t.Fatal("a sidecar that appeared after the read reads as unchanged")
	}
}

// A walk that finds a symbol by name depends on the directory's entries, which no file's stamp shows.
func TestAFileAddedToAWalkedDirectoryIsAChange(t *testing.T) {
	dir, l, rec := touchedOver(t)
	if err := l.walkDir("lib", func(string, os.DirEntry, error) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if !rec.Unchanged() {
		t.Fatal("nothing changed and the recorder says something did")
	}
	later := time.Now().Add(-time.Hour) // an old time, so only the entry names can tell
	p := filepath.Join(dir, "lib", "new.sym")
	if err := os.WriteFile(p, []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(filepath.Join(dir, "lib"), later, later)
	if rec.Unchanged() {
		t.Fatal("a symbol file added to a walked library reads as unchanged")
	}
}

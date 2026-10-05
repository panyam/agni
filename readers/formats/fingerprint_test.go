package formats

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

// fingerprinted reads a file, looks for a missing sidecar and walks a library, as a design read does,
// and returns the fingerprint and the files it ran over.
func fingerprinted(t *testing.T) ([]byte, fstest.MapFS) {
	t.Helper()
	_, l, rec := touchedOver(t)
	if _, err := l.ReadFile("a.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Open("sidecar.kicad_pcb"); err == nil {
		t.Fatal("the fixture has no sidecar")
	}
	if err := l.walkDir("lib", func(string, os.DirEntry, error) error { return nil }); err != nil {
		t.Fatal(err)
	}
	fp, ok := rec.Fingerprint()
	if !ok {
		t.Fatal("a read through a Loader's FS could not be fingerprinted")
	}
	// The same files in another name space, every one stamped at another time, as a reloaded browser
	// worker holds them.
	copied := fstest.MapFS{
		"a.txt": {Data: []byte("a"), ModTime: time.Unix(1, 0)},
		"lib":   {Mode: fs.ModeDir, ModTime: time.Unix(2, 0)},
	}
	return fp, copied
}

func TestAFingerprintMatchesTheSameContentStampedAtAnotherTime(t *testing.T) {
	fp, copied := fingerprinted(t)
	rec, ok := CheckFingerprint(copied, fp)
	if !ok {
		t.Fatal("the same files at other times read as changed, so a reload would never restore")
	}
	if rec.Len() != 3 || !rec.Unchanged() {
		t.Fatalf("the restored recorder holds %d names, unchanged %v; want the three the read touched, unchanged", rec.Len(), rec.Unchanged())
	}
	copied["a.txt"] = &fstest.MapFile{Data: []byte("b")}
	if rec.Unchanged() {
		t.Fatal("the restored recorder does not see a later edit, so the in-memory entry would go stale")
	}
}

func TestAFingerprintSeesEachKindOfChange(t *testing.T) {
	for name, change := range map[string]func(fstest.MapFS){
		"an edit of the same size": func(m fstest.MapFS) { m["a.txt"] = &fstest.MapFile{Data: []byte("b")} },
		"a deleted file":           func(m fstest.MapFS) { delete(m, "a.txt") },
		"a sidecar that appeared":  func(m fstest.MapFS) { m["sidecar.kicad_pcb"] = &fstest.MapFile{Data: []byte("x")} },
		"a file added to a walked library": func(m fstest.MapFS) {
			m["lib/new.sym"] = &fstest.MapFile{Data: []byte("v")}
		},
	} {
		t.Run(name, func(t *testing.T) {
			fp, copied := fingerprinted(t)
			change(copied)
			if _, ok := CheckFingerprint(copied, fp); ok {
				t.Fatalf("%s still matches the fingerprint, so a stale design would be restored", name)
			}
		})
	}
}

func TestAReadOffTheHostFilesystemIsNotFingerprinted(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := NewTouched()
	if _, err := (&Loader{Touched: rec}).ReadFile(p); err != nil {
		t.Fatal(err)
	}
	if rec.Len() != 1 {
		t.Fatalf("the read recorded %d names; the check below needs one", rec.Len())
	}
	if _, ok := rec.Fingerprint(); ok {
		t.Fatal("a host path was fingerprinted, though no other process resolves it to the same file")
	}
}

func TestAFingerprintThatDoesNotDecodeIsAMiss(t *testing.T) {
	_, copied := fingerprinted(t)
	for _, fp := range []string{"", "not json", `{"v":0,"names":[{"n":"a.txt"}]}`, `{"v":1,"names":[]}`} {
		if _, ok := CheckFingerprint(copied, []byte(fp)); ok {
			t.Fatalf("%q matched", fp)
		}
	}
}

func TestAFileEditedSinceTheReadIsNotFingerprinted(t *testing.T) {
	dir, l, rec := touchedOver(t)
	if _, err := l.ReadFile("a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("ab"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.Fingerprint(); ok {
		t.Fatal("the fingerprint hashed the edited file, which is not what the read saw")
	}
}

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

// summingFS knows each listed file's sum, and refuses to open one, so a fingerprint over it proves
// the file was never read.
type summingFS struct {
	fstest.MapFS
	sums map[string]string
}

func (s summingFS) Open(name string) (fs.File, error) {
	if _, ok := s.sums[name]; ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return s.MapFS.Open(name)
}

func (s summingFS) ContentSum(name string) (string, int64, bool) {
	sum, ok := s.sums[name]
	return sum, 85, ok
}

// TestAFileWithAKnownSumIsFingerprintedWithoutBeingRead covers agni issue 1515. A browser worker knows
// a file's listed hash, and may hold only a placeholder for it. A read recorded over one copy is
// checked over another by that hash, without opening either. As a control, a file system with no
// sums fingerprints the same name by its bytes, and a different listed hash is a miss.
func TestAFileWithAKnownSumIsFingerprintedWithoutBeingRead(t *testing.T) {
	stored := summingFS{MapFS: fstest.MapFS{"b.pcb": {Data: []byte("the real board")}}, sums: map[string]string{"b.pcb": "sha256:aa"}}
	rec := NewTouched()
	rec.note(stored, "b.pcb")
	fp, ok := rec.Fingerprint()
	if !ok {
		t.Fatal("a file with a known sum could not be fingerprinted")
	}
	held := summingFS{MapFS: fstest.MapFS{"b.pcb": {Data: []byte("\x00a placeholder")}}, sums: map[string]string{"b.pcb": "sha256:aa"}}
	if _, ok := CheckFingerprint(held, fp); !ok {
		t.Fatal("the same listed hash over a placeholder read as changed")
	}
	moved := summingFS{MapFS: held.MapFS, sums: map[string]string{"b.pcb": "sha256:bb"}}
	if _, ok := CheckFingerprint(moved, fp); ok {
		t.Fatal("another listed hash read as the same file")
	}
	if _, ok := CheckFingerprint(fstest.MapFS{"b.pcb": {Data: []byte("the real board")}}, fp); ok {
		t.Fatal("a file system with no sums matched a fingerprint taken by sum")
	}
}

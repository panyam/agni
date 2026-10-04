package fshost

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/panyam/goutils/memfs"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestExpandZipsReadsAZipAsItsFolder holds the expansion to fs.FS's own conformance checks, with an
// archive at the top, one inside a folder, one inside another archive, and a `.zip` that is not one.
func TestExpandZipsReadsAZipAsItsFolder(t *testing.T) {
	inner := zipOf(t, map[string]string{"deep/x.edn": "x"})
	board := zipOf(t, map[string]string{"board/board.kicad_sch": "sch", "board/board.kicad_pcb": "pcb", "nested.zip": string(inner)})
	base := MemFS(map[string][]byte{
		"board.zip":     board,
		"sub/again.zip": zipOf(t, map[string]string{"a.eds": "eds"}),
		"plain.txt":     []byte("p"),
		"broken.zip":    []byte("not a zip"),
	})
	fsys := ExpandZips(base)
	if err := fstest.TestFS(fsys,
		"board.zip/board/board.kicad_sch", "board.zip/board/board.kicad_pcb",
		"board.zip/nested.zip/deep/x.edn", "sub/again.zip/a.eds", "plain.txt", "broken.zip",
	); err != nil {
		t.Fatal(err)
	}
	if st, err := fs.Stat(fsys, "board.zip"); err != nil || !st.IsDir() {
		t.Errorf("board.zip should read as a directory: %v %v", st, err)
	}
	if st, err := fs.Stat(fsys, "broken.zip"); err != nil || st.IsDir() {
		t.Errorf("a .zip that is not an archive should stay a file: %v %v", st, err)
	}
}

// TestExpandZipsReadsTheOverlayOverAnArchive is how the page declares a design that came out of a
// zip: the store holding the browser mount (goutils' memfs, as goapplib's wasmhost uses) refuses a
// file under a path that is itself a file, so the descriptor is written under OverlayDir and reads
// as if it sat in the archive's folder.
func TestExpandZipsReadsTheOverlayOverAnArchive(t *testing.T) {
	board := zipOf(t, map[string]string{"g/g.edn": "edn", "g/g.kicad_sch": "sch"})
	if _, err := memfs.New(map[string][]byte{"d/g.zip": board, "d/g.zip/g/design.yaml": []byte("x")}); err == nil {
		t.Fatal("memfs accepted a file under a file, so the overlay is no longer needed and this test is stale")
	}
	base, err := memfs.New(map[string][]byte{
		"d/g.zip":                             board,
		OverlayDir + "/d/g.zip/g/design.yaml": []byte("name: g\nentry: g.edn\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	fsys := ExpandZips(base)
	if err := fstest.TestFS(fsys, "d/g.zip/g/g.edn", "d/g.zip/g/g.kicad_sch", "d/g.zip/g/design.yaml"); err != nil {
		t.Fatal(err)
	}
	if b, err := fs.ReadFile(fsys, "d/g.zip/g/design.yaml"); err != nil || string(b) != "name: g\nentry: g.edn\n" {
		t.Errorf("overlay file: %q %v", b, err)
	}
	if des, _ := fs.ReadDir(fsys, "."); len(des) != 1 || des[0].Name() != "d" {
		t.Errorf("the overlay should be hidden from the root: %v", des)
	}
}

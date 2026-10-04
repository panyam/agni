package fshost

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"testing"
	"testing/fstest"
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

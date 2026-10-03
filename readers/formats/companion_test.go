package formats

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// TestCompanion runs one table over a host directory and an in-memory FS holding the same tree, since
// the server reads host paths and the wasm engine reads an FS, and a companion one of them finds and
// the other misses draws one design two ways.
func TestCompanion(t *testing.T) {
	files := map[string]string{
		"d/board.edn":       "",
		"d/board.eds":       "",
		"d/lonely.edn":      "",
		"d/sheet.eds":       "",
		"d/dir.edn":         "",
		"d/dir.eds/x":       "",
		"d/other.kicad_sch": "",
		"d/other.eds":       "",
	}
	host := t.TempDir()
	mem := fstest.MapFS{}
	for name, body := range files {
		p := filepath.Join(host, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		mem[name] = &fstest.MapFile{Data: []byte(body)}
	}
	cases := []struct{ name, want string }{
		{"d/board.edn", "d/board.eds"},
		{"d/lonely.edn", ""},
		{"d/sheet.eds", ""},
		{"d/dir.edn", ""},
		{"d/other.kicad_sch", ""},
	}
	hosts := []struct {
		label  string
		loader *Loader
		path   func(string) string
	}{
		{"host", &Loader{}, func(n string) string { return filepath.Join(host, filepath.FromSlash(n)) }},
		{"fs", &Loader{FS: mem}, func(n string) string { return n }},
	}
	for _, h := range hosts {
		for _, c := range cases {
			want := ""
			if c.want != "" {
				want = h.path(c.want)
			}
			if got := h.loader.Companion(h.path(c.name)); got != want {
				t.Errorf("%s: Companion(%s) = %q, want %q", h.label, c.name, got, want)
			}
		}
	}
}

package wasmengine

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"strings"
)

// SumsFile is the manifest the page writes at a mount's root beside the design files it brings in
// (agni issue 1515). It names each file's sha256 and size as the server listed them.
// testdata/sums_twin.json holds it to the page's writer in web/src/wasm/client.ts.
const SumsFile = ".agni-sums.json"

// listedSum is one manifest row.
type listedSum struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type sumsManifest struct {
	Files map[string]listedSum `json:"files"`
}

// summed wraps a mount so that it answers formats.ContentSummer from the mount's manifest. A stored
// read is then checked by the listed hash rather than by reading the file. So a worker holding a
// placeholder for a board restores that board from the store, where it would otherwise open the
// placeholder and need the board's bytes.
//
// It answers only where the manifest agrees with the file the worker holds. A real file must have the
// listed size. A placeholder must name the listed hash. Anything else is read and summed as before.
// raw is the mount below the withholding layer, so peeking at a placeholder records no read of it.
func summed(fsys, raw fs.FS) fs.FS {
	b, err := fs.ReadFile(raw, SumsFile)
	if err != nil {
		return fsys
	}
	var m sumsManifest
	if json.Unmarshal(b, &m) != nil || len(m.Files) == 0 {
		return fsys
	}
	return summedFS{FS: fsys, raw: raw, sums: m.Files}
}

type summedFS struct {
	fs.FS
	raw  fs.FS
	sums map[string]listedSum
}

func (f summedFS) Stat(name string) (fs.FileInfo, error) { return fs.Stat(f.FS, name) }

func (f summedFS) ReadDir(name string) ([]fs.DirEntry, error) { return fs.ReadDir(f.FS, name) }

// ContentSum implements formats.ContentSummer.
func (f summedFS) ContentSum(name string) (string, int64, bool) {
	want, ok := f.sums[name]
	if !ok || !strings.HasPrefix(want.SHA256, "sha256:") {
		return "", 0, false
	}
	st, err := fs.Stat(f.raw, name)
	if err != nil || st.IsDir() {
		return "", 0, false
	}
	if st.Size() == want.Size {
		return want.SHA256, want.Size, true
	}
	if held, ok := placeholderHash(f.raw, name, st.Size()); ok && held == want.SHA256 {
		return want.SHA256, want.Size, true
	}
	return "", 0, false
}

// placeholderHash reads the hash a placeholder names, or reports false for a file that is not one.
func placeholderHash(raw fs.FS, name string, size int64) (string, bool) {
	if size < int64(len(withheldPrefix)) || size > maxPlaceholder {
		return "", false
	}
	file, err := raw.Open(name)
	if err != nil {
		return "", false
	}
	defer file.Close()
	b, err := io.ReadAll(file)
	if err != nil {
		return "", false
	}
	rest, ok := bytes.CutPrefix(b, []byte(withheldPrefix))
	if !ok {
		return "", false
	}
	return strings.TrimSpace(string(rest)), true
}

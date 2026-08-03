package common

import (
	"embed"
	"io/fs"
	"sort"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// designsFS holds the synthetic sample designs the examples read. They are hand-authored
// fixtures, not real customer boards, so every example runs hermetically (no external files
// to fetch) and this repo stays shareable: no proprietary netlist ever ships here.
//
//go:embed designs
var designsFS embed.FS

// Designs lists the bundled fixture file names (e.g. "two-resistors.edn"), sorted. Use it to
// offer a choice of designs in a walkthrough or to iterate every bundled format.
func Designs() []string {
	entries, err := fs.ReadDir(designsFS, "designs")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// ReadFixture decodes a bundled design by file name into the IR, picking the reader by
// extension exactly as ReadDesign does for on-disk files.
func ReadFixture(name string) (*ir.Design, error) {
	f, err := designsFS.Open("designs/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readByExt(f, name)
}

package common

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
)

// designsFS holds the synthetic sample designs the examples read. They are hand-authored, never a
// real board, so every example runs with no external files and no proprietary netlist ships here.
//
//go:embed designs
var designsFS embed.FS

// Designs lists the bundled fixtures as paths relative to the designs root, sorted:
// "two-resistors.edn" for a loose file, "i2c-sensor/i2c-sensor.edn" for one inside a declared
// design.
//
// It WALKS the tree rather than reading one directory, because a fixture that carries a design.yaml
// lives in its own folder beside the companions it declares (#609).
func Designs() []string {
	var names []string
	err := fs.WalkDir(designsFS, "designs", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && isDesignFile(path) {
			names = append(names, strings.TrimPrefix(path, "designs/"))
		}
		return nil
	})
	if err != nil {
		return nil
	}
	sort.Strings(names)
	return names
}

// isDesignFile reports whether a bundled file is something a reader opens, so a walkthrough never
// offers a design.yaml and a caller iterating every format is never handed a descriptor to parse.
// The set is the registered reader extensions (readers/formats/registry.go) the bundled fixtures
// use, including the .eds that ReadSchematicFixture opens.
func isDesignFile(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".edn", ".eds", ".tel", ".kicad_pcb", ".kicad_sch", ".xml", ".cvg":
		return true
	}
	return false
}

// fixturePath resolves name against the embedded tree, as either the relative path Designs reports
// or a bare base name. Walkthrough defaults and sidecar prose use the base name, so a fixture can
// move into a declared design folder without every example changing.
func fixturePath(name string) (string, error) {
	if _, err := fs.Stat(designsFS, "designs/"+name); err == nil {
		return "designs/" + name, nil
	}
	if !strings.Contains(name, "/") {
		for _, p := range Designs() {
			if path.Base(p) == name {
				return "designs/" + p, nil
			}
		}
	}
	return "", fmt.Errorf("no bundled fixture named %q", name)
}

// ReadFixture decodes a bundled design into the IR through the same formats.Loader an on-disk
// read uses, so a fixture and a file of the same design produce the same IR. name is either the
// path Designs reports or a bare base name.
func ReadFixture(name string) (*ir.Design, error) {
	p, err := fixturePath(name)
	if err != nil {
		return nil, err
	}
	return (&formats.Loader{FS: designsFS}).ReadDesign(p)
}

package common

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// designsFS holds the synthetic sample designs the examples read. They are hand-authored
// fixtures, not real customer boards, so every example runs hermetically (no external files
// to fetch) and this repo stays shareable: no proprietary netlist ever ships here.
//
//go:embed designs
var designsFS embed.FS

// Designs lists the bundled fixtures as paths relative to the designs root, sorted:
// "two-resistors.edn" for a loose file, "i2c-sensor/i2c-sensor.edn" for one inside a declared
// design.
//
// It WALKS the tree rather than reading one directory. A fixture that carries a design.yaml lives in
// its own folder beside the companions it declares, and a listing that skipped directories reported
// every such file as absent: the demo project's netlist and board have been invisible here since they
// were added.
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

// isDesignFile reports whether a bundled file is something a reader opens, which is what this
// listing is FOR: a walkthrough offering a choice must not offer a design.yaml, and a caller
// iterating every bundled format must not be handed a descriptor to parse. The set mirrors
// readByExt plus the .eds schematic geometry ReadSchematicFixture opens.
func isDesignFile(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".edn", ".eds", ".tel", ".kicad_pcb", ".kicad_sch", ".xml", ".cvg":
		return true
	}
	return false
}

// fixturePath resolves name against the embedded tree. It accepts either form a caller has: the
// relative path Designs reports, or a bare base name.
//
// The base-name form is kept working deliberately. It is what a walkthrough's default and a prose
// sentence both reach for, and resolving it here is what let i2c-sensor.edn move into a declared
// design without five examples having to learn where it went.
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

// ReadFixture decodes a bundled design into the IR, picking the reader by extension exactly as
// ReadDesign does for on-disk files. name is either the path Designs reports or a bare base name.
func ReadFixture(name string) (*ir.Design, error) {
	p, err := fixturePath(name)
	if err != nil {
		return nil, err
	}
	f, err := designsFS.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readByExt(f, name)
}

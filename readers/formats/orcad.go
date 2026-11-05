package formats

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/orcad"
)

// registerOrCAD registers .dsn, an OrCAD Capture design. Extension matching ignores case, so the
// one key covers Capture's .DSN. A SPECCTRA design shares the extension and is plain text, so the
// reader refuses it by name.
func registerOrCAD() {
	Register(&Format{Ext: ".dsn", Name: "orcad", Design: readOrCAD, Geometry: readOrCADGeometry})
}

func readOrCADGeometry(l *Loader, p string) (*geom.SchematicGeometry, error) {
	b, err := l.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return orcad.ReadGeometry(bytes.NewReader(b), p, l.orcadOpener())
}

func readOrCAD(l *Loader, p string) (*ir.Design, error) {
	b, err := l.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return orcad.Read(bytes.NewReader(b), p, l.orcadOpener())
}

// orcadOpener resolves a block's design against the design that places it. Capture records the
// path in the form of the machine that saved it (".\MPU9250\MPU9250.DSN"), and a case-insensitive
// file system let a design spell its file in another case, so each path element that is not found
// as spelled is matched against its directory's entries without regard to case.
func (l *Loader) orcadOpener() orcad.Opener {
	return func(from, ref string) (string, []byte, error) {
		rel := strings.ReplaceAll(ref, "\\", "/")
		if strings.HasPrefix(rel, "/") || filepath.VolumeName(rel) != "" || (len(rel) > 1 && rel[1] == ':') {
			return "", nil, fmt.Errorf("block design %q is an absolute path, which agni does not follow", ref)
		}
		name := l.dir(from)
		for _, elem := range strings.Split(rel, "/") {
			switch elem {
			case "", ".":
				continue
			case "..":
				name = l.dir(name)
				continue
			}
			name = l.matchEntry(name, elem)
		}
		b, err := l.ReadFile(name)
		if err != nil {
			return "", nil, err
		}
		return name, b, nil
	}
}

// matchEntry is dir joined with elem, spelled as dir spells it when only the case differs.
func (l *Loader) matchEntry(dir, elem string) string {
	exact := l.join(dir, elem)
	if l != nil && l.Touched != nil {
		l.Touched.note(l.FS, dir) // a file renamed in dir can change which one the block names
	}
	var entries []fs.DirEntry
	var err error
	if l == nil || l.FS == nil {
		entries, err = os.ReadDir(dir)
	} else {
		entries, err = fs.ReadDir(l.FS, dir)
	}
	if err != nil {
		return exact
	}
	for _, e := range entries {
		if e.Name() == elem {
			return exact
		}
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), elem) {
			return l.join(dir, e.Name())
		}
	}
	return exact
}

package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/agni/datasheet/doc"
	docpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/doc"
)

// docSiblingSuffix is the extension of a doc-IR an older layout wrote beside its PDF, so LM1117.pdf
// paired with LM1117.doc.textproto. `make pdf2doc` still writes one, and the loader reads it when the
// store has none.
const docSiblingSuffix = ".doc.textproto"

// docFile is the doc-IR's name inside a datasheet's derived folder. It is textproto, named .txtpb
// rather than .textproto, because param's corpus walk reads every *.textproto under the corpus as a
// PartSpec, and one doc-IR read that way would stop the whole corpus loading.
const docFile = "doc.txtpb"

// osDocLoader is the OS-backed dsservice.DocLoader. It reads a datasheet's doc-IR from the corpus
// store, or from beside the PDF when the store has none, and parses it with doc.Load. A datasheet
// with no doc-IR yet is not an error, and the service reports it as extracted=false.
type osDocLoader struct {
	derived derived
}

// Document reads the doc-IR for the datasheet at uri. mountset.Table.Resolve classifies an unknown mount or
// a path escaping the mount, as service.ErrNotFound or ErrInvalidPath. No doc-IR returns (nil, nil).
// An unparseable one returns a parse error, and the service classifies it as invalid.
func (l *osDocLoader) Document(ctx context.Context, uri mounts.URI) (*docpb.Document, error) {
	if l.derived.corpus != "" {
		dir, err := l.derived.dir(uri)
		if err != nil {
			return nil, err
		}
		if d, err := loadDoc(filepath.Join(dir, docFile)); d != nil || err != nil {
			return d, err
		}
	}
	abs, err := l.derived.legacy(uri, docSibling)
	if err != nil {
		return nil, err
	}
	return loadDoc(abs)
}

// loadDoc parses the doc-IR at abs. A missing file returns (nil, nil).
func loadDoc(abs string) (*docpb.Document, error) {
	f, err := os.Open(abs) // #nosec G304 -- abs is under the corpus store or comes from resolveSibling, which resolves inside the mount
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil // not yet extracted
		}
		return nil, err
	}
	defer f.Close()
	return doc.Load(f)
}

// docSibling maps a datasheet source path to its doc-IR sibling, the same directory and stem with
// docSiblingSuffix.
func docSibling(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + docSiblingSuffix
}

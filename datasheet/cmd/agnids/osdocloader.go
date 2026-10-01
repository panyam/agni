package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/datasheet/doc"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	"github.com/panyam/agni/mounts"
)

// docSiblingSuffix is the extension of a datasheet's doc-IR sibling, so LM1117.pdf pairs with
// LM1117.doc.textproto. The file is produced offline (tools/pdf2doc, docling) and this loader only
// reads it, since no Go PDF->doc-IR producer exists (WS13-006).
const docSiblingSuffix = ".doc.textproto"

// osDocLoader is the OS-backed dsservice.DocLoader. It resolves a datasheet's source path to its
// sibling doc-IR file under the mount and parses it with doc.Load. A datasheet with no sibling yet
// is not an error, and the service reports it as extracted=false.
type osDocLoader struct {
	mounts []mounts.Mount
}

// Document resolves the sibling doc-IR for the datasheet at uri and parses it. An unknown mount or
// a path escaping the mount comes back already classified by mounts.Resolve (service.ErrNotFound or
// ErrInvalidPath). A missing sibling is (nil, nil), and an unparseable one is a parse error the
// service classifies as invalid.
func (l *osDocLoader) Document(ctx context.Context, uri artifact.URI) (*docpb.Document, error) {
	abs, err := resolveSibling(l.mounts, uri, docSibling)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
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

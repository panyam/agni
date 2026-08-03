package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/doc"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	"github.com/panyam/agni/internal/mounts"
)

// docSiblingSuffix is the extension the doc-IR sibling of a datasheet uses: LM1117.pdf pairs
// with LM1117.doc.textproto. Producing that file is offline (tools/pdf2doc, docling); this
// loader only reads it — there is no Go PDF->doc-IR producer (WS13-006).
const docSiblingSuffix = ".doc.textproto"

// osDocLoader is the OS-backed service.DocLoader: it resolves a datasheet's source path to its
// sibling doc-IR file under the mount and parses it (doc.Load). The sibling convention and all
// filesystem access live here at the cmd edge (CONSTRAINTS C1/C13); the service package is
// os-free. A datasheet with no sibling doc-IR yet returns (nil, nil): "not yet extracted" is a
// normal state the service reports as extracted=false.
type osDocLoader struct {
	mounts []mounts.Mount
}

// Document resolves the sibling doc-IR for the datasheet at (mount, path) and parses it. An
// unknown mount or a path escaping the mount is returned already classified by mounts.Resolve
// (service.ErrNotFound / ErrInvalidPath). A missing sibling is (nil, nil); a present-but-
// unparseable one is a parse error the service classifies as invalid.
func (l *osDocLoader) Document(_ context.Context, mountName, path string) (*docpb.Document, error) {
	abs, err := mounts.Resolve(l.mounts, mountName, docSibling(path))
	if err != nil {
		return nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil // not yet extracted — a normal state, not an error
		}
		return nil, err
	}
	defer f.Close()
	return doc.Load(f)
}

// docSibling maps a datasheet source path to its doc-IR sibling path (same directory + stem,
// docSiblingSuffix): foo/LM1117.pdf -> foo/LM1117.doc.textproto.
func docSibling(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + docSiblingSuffix
}

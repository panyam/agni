package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/datasheet/dsservice"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
)

// derivedDir is the corpus folder that holds what the workbench derives from a datasheet.
const derivedDir = "derived"

// derived places what the workbench derives from a datasheet PDF, its doc-IR and its annotations,
// in the corpus store under derived/<sha256 of the PDF>/ (agni issue 1734). The PDF stays in its
// mount, and nothing is written there. So a mount can be read-only, and the same PDF in two mounts
// shares one doc-IR. A doc-IR or annotations written beside the PDF by `make pdf2doc` or an older
// agnids are still read when the store has none.
type derived struct {
	mounts []mounts.Mount
	// corpus is the corpus store's folder. Empty means the server has no store, so nothing derived
	// can be written.
	corpus string
}

// dir returns the store folder for the datasheet at uri. It hashes the PDF, so a PDF the mount
// lacks is service.ErrNotFound. A server with no corpus returns dsservice.ErrNoCorpus.
func (d derived) dir(uri artifact.URI) (string, error) {
	if d.corpus == "" {
		return "", dsservice.ErrNoCorpus
	}
	pdf, err := mounts.Resolve(d.mounts, uri)
	if err != nil {
		return "", err
	}
	f, err := os.Open(pdf) // #nosec G304 -- pdf comes from mounts.Resolve, which keeps it inside the mount
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w: no datasheet at %s", service.ErrNotFound, uri)
		}
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return filepath.Join(d.corpus, derivedDir, hex.EncodeToString(h.Sum(nil))), nil
}

// legacy resolves a file an older layout derived beside the PDF, such as LM1117.doc.textproto
// beside LM1117.pdf. It is only read.
func (d derived) legacy(uri artifact.URI, derive func(string) string) (string, error) {
	return resolveSibling(d.mounts, uri, derive)
}

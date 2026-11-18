package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/datasheet/doc"
	docpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/doc"
	"github.com/panyam/agni/mounts"
)

// osDocExtractor is the OS-backed dsservice.DocExtractor. It shells out to the doc-IR producer named
// by --pdf2doc (pdf2doc/docling) to write a datasheet's doc-IR into the corpus store (see derived),
// and an empty command disables extraction. Docling is external and CI-excluded, so the engine never
// bundles it. It runs the configured argv inside the boundary, with the resolved PDF and output paths
// appended, so the datasheet bytes never leave (C16).
type osDocExtractor struct {
	derived derived
	cmd     []string // producer argv (cmd[0] is the executable); empty = extraction disabled
}

// Available reports whether a producer command is configured, so the service offers the
// "Extract (first pass)" action only on a server started with --pdf2doc.
func (e *osDocExtractor) Available() bool { return len(e.cmd) > 0 }

// Extract runs the producer over the datasheet at uri, writing its doc-IR into the corpus store and
// returning the parsed and validated Document. The source PDF resolves inside the mount through
// mounts.Resolve. A server with no corpus returns dsservice.ErrNoCorpus. A non-zero exit, an
// unreadable output, or an invalid doc-IR is an error, and the service maps it to Internal.
func (e *osDocExtractor) Extract(ctx context.Context, uri artifact.URI) (*docpb.Document, error) {
	pdfAbs, err := mounts.Resolve(e.derived.mounts, uri)
	if err != nil {
		return nil, err
	}
	dir, err := e.derived.dir(uri)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { // #nosec G301 -- the shared corpus, which the engine reads too
		return nil, err
	}
	outAbs := filepath.Join(dir, docFile)
	args := append(append([]string{}, e.cmd[1:]...), pdfAbs, "-o", outAbs)
	if out, err := exec.CommandContext(ctx, e.cmd[0], args...).CombinedOutput(); err != nil { // #nosec G204 -- runs the configured pdf2doc over paths mounts.Resolve admitted
		return nil, fmt.Errorf("pdf2doc %q: %v: %s", e.cmd[0], err, strings.TrimSpace(string(out)))
	}
	f, err := os.Open(outAbs) // #nosec G304 -- outAbs is under the corpus store
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d, err := doc.Load(f)
	if err != nil {
		return nil, err
	}
	if err := doc.Validate(d); err != nil {
		return nil, fmt.Errorf("produced doc-IR failed validation: %w", err)
	}
	return d, nil
}

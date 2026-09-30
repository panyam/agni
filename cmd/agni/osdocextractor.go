package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/datasheet/doc"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	"github.com/panyam/agni/internal/mounts"
)

// osDocExtractor is the OS-backed service.DocExtractor. It shells out to the doc-IR producer named
// by --pdf2doc (pdf2doc/docling) to write a datasheet's <stem>.doc.textproto sibling, and an empty
// command disables extraction. Docling is external and CI-excluded, so the engine never bundles it.
// It runs the configured argv with the resolved PDF and output paths appended, in-boundary, so the
// datasheet bytes never leave (C16).
type osDocExtractor struct {
	mounts []mounts.Mount
	cmd    []string // producer argv (cmd[0] is the executable); empty = extraction disabled
}

// Available reports whether a producer command is configured, so the service offers the
// "Extract (first pass)" action only on a server started with --pdf2doc.
func (e *osDocExtractor) Available() bool { return len(e.cmd) > 0 }

// Extract runs the producer over the datasheet at uri, writing the sibling doc-IR and returning
// the parsed and validated Document. The source PDF and the output sibling both resolve inside the
// mount through mounts.Resolve. A non-zero exit, an unreadable output, or an invalid doc-IR is an
// error the service maps to Internal.
func (e *osDocExtractor) Extract(ctx context.Context, uri artifact.URI) (*docpb.Document, error) {
	pdfAbs, err := mounts.Resolve(e.mounts, uri)
	if err != nil {
		return nil, err
	}
	outAbs, err := resolveSibling(e.mounts, uri, docSibling)
	if err != nil {
		return nil, err
	}
	args := append(append([]string{}, e.cmd[1:]...), pdfAbs, "-o", outAbs)
	if out, err := exec.CommandContext(ctx, e.cmd[0], args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pdf2doc %q: %v: %s", e.cmd[0], err, strings.TrimSpace(string(out)))
	}
	f, err := os.Open(outAbs)
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

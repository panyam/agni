package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/panyam/agni/datasheet/doc"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	"github.com/panyam/agni/internal/mounts"
)

// osDocExtractor is the OS-backed service.DocExtractor: it shells out to the operator-configured
// doc-IR producer (pdf2doc/docling) to derive a datasheet's <stem>.doc.textproto sibling. The
// command is set with --pdf2doc; an empty command means extraction is disabled. All I/O and the
// sibling convention live here at the cmd edge (CONSTRAINTS C1/C13). Docling is external and
// CI-excluded: the engine never bundles it, it runs the configured argv with the resolved PDF and
// output paths appended, and it runs in-boundary (the datasheet bytes never leave, C16).
type osDocExtractor struct {
	mounts []mounts.Mount
	cmd    []string // producer argv (cmd[0] is the executable); empty = extraction disabled
}

// Available reports whether a producer command is configured (so the service can offer the
// "Extract (first pass)" action only when a server was started with --pdf2doc).
func (e *osDocExtractor) Available() bool { return len(e.cmd) > 0 }

// Extract runs the producer over the datasheet at (mount, path), writing the sibling doc-IR and
// returning the parsed + validated Document. Both the source PDF and the output sibling resolve
// inside the mount (containment via mounts.Resolve, the write path). A non-zero exit, an unreadable
// output, or an invalid doc-IR is an error the service maps to Internal.
func (e *osDocExtractor) Extract(ctx context.Context, mountName, path string) (*docpb.Document, error) {
	pdfAbs, err := mounts.Resolve(e.mounts, mountName, path)
	if err != nil {
		return nil, err
	}
	outAbs, err := mounts.Resolve(e.mounts, mountName, docSibling(path))
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

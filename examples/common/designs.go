// Package common is the shared reuse payload for Agni's runnable examples: design
// loading (the same reader dispatch agni's CLI does at its edge), the bundled synthetic
// fixtures every example reads, narration pretty-printers, and the demokit renderer
// wiring. Examples import it so each one stays a thin walkthrough over its own sidecar
// markdown, not a copy of the same plumbing.
//
// This package deliberately lives at the I/O edge. CONSTRAINTS C1 keeps file paths out of
// the engine core (edif/kicad/ipc2581 each take an io.Reader), so the path handling lives
// here and the reading goes through formats.Loader, which is the same entry point cmd/agni
// uses. That is not a stylistic choice: the Loader is where the format-neutral INGESTION
// PASSES run, and dispatching to a reader directly skips them silently.
package common

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
)

// Load reads a design from arg, which an example takes as user input. arg may be a filesystem
// path (absolute, or relative to the working directory, e.g. "../common/designs/foo.edn") or
// the bare name of a bundled fixture. It tries the path on disk first, so an example can point
// at any design including your own; if no such file exists it falls back to the embedded
// fixture whose base name matches, so the examples still run from any directory. A file that
// exists but fails to parse is reported, not masked by the fallback.
func Load(arg string) (*ir.Design, error) {
	d, err := ReadDesign(arg)
	if err == nil {
		return d, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err // exists but failed to parse (or another real error): don't mask it
	}
	if d, ferr := ReadFixture(filepath.Base(arg)); ferr == nil {
		return d, nil
	}
	return nil, fmt.Errorf("no design at path %q, and no bundled fixture named %q", arg, filepath.Base(arg))
}

// ReadDesign reads a design file from disk into the IR through formats.Loader, which is what
// cmd/agni reads through. Reader dispatch by extension, the .kicad_pro project merge, and the
// format-neutral ingestion passes all live there.
//
// Going through the Loader rather than calling a reader directly is the whole point of this
// function. The passes are where ir.Component.mpn is filled (classify.StampMPN) and where
// provenance paths are rewritten, and a read that skips them produces an IR that parses,
// counts correctly, and answers every datasheet-tier question with nothing. Nothing catches a
// pass that is never called, so this stays one call and not a dispatch of its own.
func ReadDesign(path string) (*ir.Design, error) {
	return (&formats.Loader{}).ReadDesign(path)
}

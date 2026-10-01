// Package common is the shared plumbing for Agni's runnable examples. It holds design loading,
// the bundled synthetic fixtures, narration pretty-printers, and the demokit renderer wiring, so
// each example stays a thin walkthrough over its own sidecar markdown.
//
// It lives at the I/O edge, since C1 keeps file paths out of the engine core. Reads go through
// formats.Loader, the same entry point cmd/agni uses, because the Loader is where the
// format-neutral INGESTION PASSES run and a direct reader call skips them silently.
package common

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
)

// Load reads a design from arg, which is either a filesystem path (absolute, or relative to the
// working directory) or the bare name of a bundled fixture. It tries the path on disk first, then
// falls back to the embedded fixture with that base name, so the examples run from any directory.
// A file that exists but fails to parse is reported, not masked by the fallback.
func Load(arg string) (*ir.Design, error) {
	d, err := ReadDesign(arg)
	if err == nil {
		return d, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err // exists but failed to parse, or another real error
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
// Keep this one Loader call rather than a dispatch of its own (agni issue 618). The passes fill
// ir.Component.mpn (classify.StampMPN) and rewrite provenance paths, and a read that skips them
// parses and counts correctly while answering every datasheet-tier question with nothing.
func ReadDesign(path string) (*ir.Design, error) {
	return (&formats.Loader{}).ReadDesign(path)
}

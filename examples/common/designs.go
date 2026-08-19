// Package common is the shared reuse payload for Agni's runnable examples: design
// loading (the same reader dispatch agni's CLI does at its edge), the bundled synthetic
// fixtures every example reads, narration pretty-printers, and the demokit renderer
// wiring. Examples import it so each one stays a thin walkthrough over its own sidecar
// markdown, not a copy of the same plumbing.
//
// This package deliberately lives at the I/O edge. CONSTRAINTS C1 keeps file paths out of
// the engine core (edif/kicad/ipc2581 each take an io.Reader), so the readByExt/ReadDesign
// edge here mirrors cmd/agni's readDesign: examples do their file I/O the same way the CLI
// does, without the core ever learning about paths.
package common

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/readers/edif"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/ipc2581"
	"github.com/panyam/agni/readers/kicad"
	"github.com/panyam/agni/readers/telesis"
)

// readByExt picks a reader by the name's extension and decodes r into the neutral IR. It is
// the format dispatch shared by ReadDesign (on-disk files) and ReadFixture (embedded
// fixtures); name supplies both the extension and the provenance source path.
func readByExt(r io.Reader, name string) (*ir.Design, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".edn":
		return edif.Read(r, name)
	case ".tel":
		return telesis.Read(r, name)
	case ".kicad_pcb":
		return kicad.Read(r, name)
	case ".kicad_sch":
		return kicad.ReadSchematic(r, name)
	case ".xml", ".cvg":
		// .xml is ambiguous, so sniff for the IPC-2581 root before committing to that reader.
		br := bufio.NewReader(r)
		head, _ := br.Peek(1024)
		if !bytes.Contains(head, []byte("IPC-2581")) {
			return nil, fmt.Errorf("%q: not an IPC-2581 file (no IPC-2581 root element)", name)
		}
		return ipc2581.Read(br, name)
	default:
		return nil, fmt.Errorf("no reader for %q (have: .edn, .kicad_pcb, .kicad_sch, .kicad_pro, .xml/.cvg)", filepath.Ext(name))
	}
}

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

// ReadDesign reads a design file from disk into the IR, picking a reader by extension. A
// .kicad_pro is a whole project (its sibling schematic + board merged); the rest are single
// files. It mirrors cmd/agni's readDesign so examples read on-disk files exactly as the CLI
// does. Examples that only touch bundled fixtures use ReadFixture instead.
func ReadDesign(path string) (*ir.Design, error) {
	if strings.ToLower(filepath.Ext(path)) == ".kicad_pro" {
		return readKicadProject(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readByExt(f, path)
}

// readKicadProject merges the sibling .kicad_sch and .kicad_pcb sharing the .kicad_pro stem
// into one IR. Either sibling may be absent; the merge degrades to whichever exists.
func readKicadProject(proPath string) (*ir.Design, error) {
	stem := strings.TrimSuffix(proPath, filepath.Ext(proPath))
	var schR, pcbR io.Reader
	if f, err := os.Open(stem + ".kicad_sch"); err == nil {
		defer f.Close()
		schR = f
	}
	if f, err := os.Open(stem + ".kicad_pcb"); err == nil {
		defer f.Close()
		pcbR = f
	}
	if schR == nil && pcbR == nil {
		return nil, fmt.Errorf("kicad project %q: no sibling .kicad_sch or .kicad_pcb found", proPath)
	}
	return kicad.ReadProject(schR, pcbR, stem+".kicad_sch", stem+".kicad_pcb", nil)
}

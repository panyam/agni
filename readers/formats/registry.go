package formats

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/readers/edif"
	"github.com/panyam/agni/readers/geda"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/ipc2581"
	"github.com/panyam/agni/readers/kicad"
	"github.com/panyam/agni/readers/xschem"
)

func init() {
	// EDIF netlists appear under several extensions in the wild: our fixtures use .edn, but real
	// exports (and the whole EDIF corpus) use .edf/.edif. All three are the same netlist reader;
	// the geometry-only .eds is separate (see below). Extension matching is case-insensitive
	// (lowerExt), so .EDF resolves here too.
	for _, ext := range []string{".edn", ".edf", ".edif"} {
		Register(&Format{Ext: ext, Name: "edif", Design: readEDIF})
	}
	Register(&Format{
		Ext:  ".eds",
		Name: "edif-schematic",
		// An EDIF SCHEMATIC export carries explicit netlist connectivity too (nets joining
		// portRefs), the same grammar the .edn/.edf/.edif netlist reader parses — so a .eds is
		// a dual-capability format (netlist + faithful geometry), like a .kicad_sch. Wiring the
		// netlist reader makes .eds queryable/checkable/diffable (it renders either way).
		Design: readEDIF,
		Geometry: func(_ *Loader, path string) (*geom.SchematicGeometry, error) {
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return edif.ReadSchematic(f, path)
		},
	})
	Register(&Format{
		Ext:  ".kicad_sch",
		Name: "kicad",
		Design: func(l *Loader, path string) (*ir.Design, error) {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			// Walk the sheet tree (WS1-018), matching what the geometry entry below shows:
			// the viewer's sheets and the rules' nets come from the same hierarchy. The
			// completeness flag is deliberately dropped — a bare .kicad_sch may itself be
			// one sheet of a larger design, so its external markings never resolve; only
			// the .kicad_pro read is a completeness witness (WS1-017).
			d, _, err := kicad.ReadSchematicHierarchyNetsWithSymbols(path, content, sheetOpener(path), l.kicadSymOpener(path))
			return d, err
		},
		Geometry: func(l *Loader, path string) (*geom.SchematicGeometry, error) {
			return readKicadHierarchy(l, path)
		},
	})
	Register(&Format{
		Ext:  ".kicad_pcb",
		Name: "kicad",
		Design: func(_ *Loader, path string) (*ir.Design, error) {
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return kicad.Read(f, path)
		},
		Board: func(_ *Loader, path string) (*geom.BoardGeometry, error) {
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return kicad.ReadBoardGeometry(f, path)
		},
	})
	Register(&Format{
		Ext:      ".kicad_pro",
		Name:     "kicad",
		Design:   readKicadProject,
		Geometry: readKicadProjectGeometry,
	})
	Register(&Format{
		Ext:      ".sch",
		Name:     "xschem", // shared by xschem/gEDA/legacy-KiCad; sniffed for real at load
		Design:   readSchDesign,
		Geometry: readSchGeometry,
	})
	Register(&Format{Ext: ".xml", Name: "ipc2581", Design: readIPC2581, Board: readIPC2581Board})
	Register(&Format{Ext: ".cvg", Name: "ipc2581", Design: readIPC2581, Board: readIPC2581Board})
}

// readKicadProject merges the sibling .kicad_sch and .kicad_pcb that share the .kicad_pro's
// stem into one IR (schematic structure + board connectivity). Either sibling may be
// absent; the merge degrades to whichever exists.
func readKicadProject(l *Loader, proPath string) (*ir.Design, error) {
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
	d, err := kicad.ReadProjectWithSymbols(schR, pcbR, stem+".kicad_sch", stem+".kicad_pcb", sheetOpener(stem+".kicad_sch"), l.kicadSymOpener(stem+".kicad_sch"))
	if err != nil {
		return nil, err
	}
	// Net-class membership lives only in the .kicad_pro (net_settings), not the sch/pcb the
	// readers consume, so populate ir.Net.net_class here in the I/O layer (WS1-037, C1).
	if pro, err := os.Open(proPath); err == nil {
		kicad.AnnotateNetClasses(d, kicad.ParseNetClasses(pro))
		pro.Close()
	}
	return d, nil
}

// sheetOpener resolves a schematic's sub-sheet Sheetfile references against its own
// directory — the same contract the geometry walk's opener (readKicadHierarchy) uses, so
// netlist and geometry read the same tree.
func sheetOpener(schPath string) func(relPath string) ([]byte, error) {
	dir := filepath.Dir(schPath)
	return func(relPath string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, relPath)) }
}

// readKicadProjectGeometry reads a project's faithful schematic: its sibling .kicad_sch
// (same stem), read as a hierarchy.
func readKicadProjectGeometry(l *Loader, proPath string) (*geom.SchematicGeometry, error) {
	schPath := strings.TrimSuffix(proPath, filepath.Ext(proPath)) + ".kicad_sch"
	g, err := readKicadHierarchy(l, schPath)
	if err != nil {
		return nil, fmt.Errorf("kicad project %q: no sibling schematic %q: %w", proPath, filepath.Base(schPath), err)
	}
	return g, nil
}

// readKicadHierarchy reads a .kicad_sch and its hierarchical sub-sheets into geometry. It
// reads the root here and hands the kicad package an opener that resolves each child's
// Sheetfile against the root's directory, so the reader owns no file I/O (C1).
func readKicadHierarchy(l *Loader, schPath string) (*geom.SchematicGeometry, error) {
	content, err := os.ReadFile(schPath)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(schPath)
	open := func(relPath string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, relPath)) }
	return kicad.ReadSchematicHierarchyWithSymbols(schPath, content, open, l.kicadSymOpener(schPath))
}

// readSchDesign nets a .sch, which is shared by xschem, gEDA gschem, and legacy KiCad.
// Sniff the header: an xschem file opens with "v {xschem", a gEDA file with "v <version>
// <flags>". Symbol artwork resolves through the Loader's --symbol-path opener.
func readSchDesign(l *Loader, path string) (*ir.Design, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	head, _ := br.Peek(256)
	switch {
	case xschem.IsXschem(head):
		return xschem.ReadWithSymbols(br, path, l.symbolOpener(path))
	case geda.IsGeda(head):
		return geda.ReadWithSymbols(br, path, l.symbolOpener(path))
	default:
		return nil, fmt.Errorf("%q: unrecognized .sch dialect (want xschem or gEDA gschem; legacy KiCad .sch is not supported)", path)
	}
}

// readSchGeometry reads an xschem/gEDA schematic drawing, sniffing the dialect the same way
// readSchDesign does and resolving symbol artwork through the same --symbol-path opener.
func readSchGeometry(l *Loader, path string) (*geom.SchematicGeometry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	head, _ := br.Peek(256)
	switch {
	case xschem.IsXschem(head):
		return xschem.ReadSchematicGeometry(br, path, l.symbolOpener(path))
	case geda.IsGeda(head):
		return geda.ReadSchematicGeometry(br, path, l.symbolOpener(path))
	default:
		return nil, fmt.Errorf("%q: unrecognized .sch dialect (want xschem or gEDA gschem)", path)
	}
}

// readIPC2581 reads an IPC-2581 file. .xml is ambiguous, so sniff for the IPC-2581 root
// before committing to that reader.
// readEDIF reads an EDIF netlist into the IR. Shared by the .edn/.edf/.edif extensions, which
// are all the same format under different conventional suffixes.
func readEDIF(_ *Loader, path string) (*ir.Design, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return edif.Read(f, path)
}

func readIPC2581(_ *Loader, path string) (*ir.Design, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	head, _ := br.Peek(1024)
	if !bytes.Contains(head, []byte("IPC-2581")) {
		return nil, fmt.Errorf("%q: not an IPC-2581 file (no IPC-2581 root element)", path)
	}
	return ipc2581.Read(br, path)
}

// readIPC2581Board reads the board-geometry sidecar from an IPC-2581 file, sniffing the same
// ambiguous .xml root as readIPC2581 before committing to the reader.
func readIPC2581Board(_ *Loader, path string) (*geom.BoardGeometry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	head, _ := br.Peek(1024)
	if !bytes.Contains(head, []byte("IPC-2581")) {
		return nil, fmt.Errorf("%q: not an IPC-2581 file (no IPC-2581 root element)", path)
	}
	return ipc2581.ReadBoardGeometry(br, path)
}

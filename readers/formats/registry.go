package formats

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/readers/edif"
	"github.com/panyam/agni/readers/geda"
	"github.com/panyam/agni/readers/ipc2581"
	"github.com/panyam/agni/readers/kicad"
	"github.com/panyam/agni/readers/telesis"
	"github.com/panyam/agni/readers/xschem"
)

func init() {
	// EDIF netlists appear under several extensions. Our fixtures use .edn, and real exports and the
	// whole EDIF corpus use .edf/.edif. All three share one netlist reader. The schematic export .eds
	// registers separately below. Extension matching is case-insensitive (lowerExt), so .EDF
	// resolves here too.
	for _, ext := range []string{".edn", ".edf", ".edif"} {
		Register(&Format{Ext: ext, Name: "edif", Design: readEDIF})
	}
	Register(&Format{
		Ext:  ".eds",
		Name: "edif-schematic",
		// An EDIF SCHEMATIC export has netlist connectivity too, as nets joining portRefs. It is
		// in the grammar the netlist reader parses. So a .eds is dual-capability like a .kicad_sch,
		// with a netlist and faithful geometry. Wiring the netlist reader makes it queryable,
		// checkable and diffable. Its netlist counts drawn instances and per-sheet segments, so its
		// counts are not comparable with an .edn read.
		Design: readEDIF,
		Geometry: func(l *Loader, path string) (*geom.SchematicGeometry, error) {
			f, err := l.Open(path)
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
			content, err := l.ReadFile(path)
			if err != nil {
				return nil, err
			}
			// Walk the sheet tree (WS1-018), matching the geometry entry below. The viewer's sheets
			// and the rules' nets then come from the same hierarchy. A complete walk resolves the
			// external markings (WS1-017) only when the schematic's own .kicad_pro sits beside it.
			// That project makes the schematic the project's root. Without one, the schematic may
			// be another design's sub-sheet (agni issue 1013).
			d, complete, err := kicad.ReadSchematicHierarchyNetsWithSymbols(path, content, l.sheetOpener(path), l.kicadSymOpener(path))
			if err != nil {
				return nil, err
			}
			if annotateFromProject(l, d, kicadProjectOf(path)) && complete {
				netgraph.ResolveExternal(d)
			}
			return d, nil
		},
		Geometry: func(l *Loader, path string) (*geom.SchematicGeometry, error) {
			return readKicadHierarchy(l, path)
		},
	})
	Register(&Format{
		Ext:  ".kicad_pcb",
		Name: "kicad",
		Design: func(l *Loader, path string) (*ir.Design, error) {
			f, err := l.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			d, err := kicad.Read(f, path)
			if err != nil {
				return nil, err
			}
			annotateFromProject(l, d, kicadProjectOf(path))
			return d, nil
		},
		Board: func(l *Loader, path string) (*geom.BoardGeometry, error) {
			f, err := l.Open(path)
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
	// The flat Telesis netlist the Mentor/Siemens flow emits. The format holds connectivity and
	// properties and no geometry at all. So Geometry and Board are nil, and the design renders
	// through auto-layout like any other netlist-only source.
	Register(&Format{Ext: ".tel", Name: "telesis", Design: readTelesis})
	Register(&Format{Ext: ".xml", Name: "ipc2581", Design: readIPC2581, Board: readIPC2581Board})
	Register(&Format{Ext: ".cvg", Name: "ipc2581", Design: readIPC2581, Board: readIPC2581Board})
	registerODB()
}

// readKicadProject merges the sibling .kicad_sch and .kicad_pcb that share the .kicad_pro's stem
// into one IR, with the schematic's structure and the board's connectivity. Either sibling may be
// absent, and the merge then uses whichever exists.
func readKicadProject(l *Loader, proPath string) (*ir.Design, error) {
	stem := strings.TrimSuffix(proPath, filepath.Ext(proPath))
	var schR, pcbR io.Reader
	if f, err := l.Open(stem + ".kicad_sch"); err == nil {
		defer f.Close()
		schR = f
	}
	if f, err := l.Open(stem + ".kicad_pcb"); err == nil {
		defer f.Close()
		pcbR = f
	}
	if schR == nil && pcbR == nil {
		return nil, fmt.Errorf("kicad project %q: no sibling .kicad_sch or .kicad_pcb found", proPath)
	}
	d, err := kicad.ReadProjectWithSymbols(schR, pcbR, stem+".kicad_sch", stem+".kicad_pcb", l.sheetOpener(stem+".kicad_sch"), l.kicadSymOpener(stem+".kicad_sch"))
	if err != nil {
		return nil, err
	}
	annotateFromProject(l, d, proPath)
	return d, nil
}

// kicadProjectOf returns the .kicad_pro a schematic or board belongs to, the one sharing its stem.
// A sub-sheet has a stem of its own and so no project. That is correct. A sub-sheet is one sheet of
// a larger design, not the design.
func kicadProjectOf(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".kicad_pro"
}

// annotateFromProject applies what only the .kicad_pro declares. It does so whichever of the
// project's files the read began from (agni issue 933). Neither the schematic nor the board holds
// these settings. Reading either without its project used to drop them. A design named by its
// schematic then checked its copper against fixed floors and none of its own rules.
//
// Net-class membership and per-class routing constraints come from net_settings (WS1-037, WS3-111).
// The board-wide minimums come from board.design_settings.rules. Each pass decodes the whole file,
// so each gets its own reader over one buffer. An fs.File need not be an io.Seeker (WS1-049), and
// independent readers also make the passes order-independent. If the project is absent or
// unreadable, the design is left as read, and the result says whether a project was found.
func annotateFromProject(l *Loader, d *ir.Design, proPath string) bool {
	data, err := l.ReadFile(proPath)
	if err != nil {
		return false
	}
	kicad.AnnotateNetClasses(d, kicad.ParseNetClasses(bytes.NewReader(data)))
	kicad.AnnotateNetClassDefs(d, kicad.ParseNetClassDefs(bytes.NewReader(data)))
	kicad.AnnotateBoardRules(d, kicad.ParseBoardRules(bytes.NewReader(data)))
	return true
}

// sheetOpener resolves a schematic's sub-sheet Sheetfile references against its own
// directory. The geometry walk (readKicadHierarchy) uses the same opener, so netlist and
// geometry read the same tree.
func (l *Loader) sheetOpener(schPath string) func(relPath string) ([]byte, error) {
	return func(relPath string) ([]byte, error) { return l.ReadFile(l.Sibling(schPath, relPath)) }
}

// readKicadProjectGeometry reads a project's faithful schematic, which is its sibling
// .kicad_sch (same stem) read as a hierarchy.
func readKicadProjectGeometry(l *Loader, proPath string) (*geom.SchematicGeometry, error) {
	schPath := strings.TrimSuffix(proPath, filepath.Ext(proPath)) + ".kicad_sch"
	g, err := readKicadHierarchy(l, schPath)
	if err != nil {
		return nil, fmt.Errorf("kicad project %q: no sibling schematic %q: %w", proPath, filepath.Base(schPath), err)
	}
	return g, nil
}

// readKicadHierarchy reads a .kicad_sch and its hierarchical sub-sheets into geometry. It reads the
// root here and hands the kicad package an opener. The opener resolves each child's Sheetfile
// against the root's directory, so the reader owns no file I/O (C1).
func readKicadHierarchy(l *Loader, schPath string) (*geom.SchematicGeometry, error) {
	content, err := l.ReadFile(schPath)
	if err != nil {
		return nil, err
	}
	return kicad.ReadSchematicHierarchyWithSymbols(schPath, content, l.sheetOpener(schPath), l.kicadSymOpener(schPath))
}

// readSchDesign nets a .sch, which is shared by xschem, gEDA gschem, and legacy KiCad. It
// sniffs the header, where an xschem file opens with "v {xschem" and a gEDA file with
// "v <version> <flags>". Symbol artwork resolves through the Loader's --symbol-path opener.
func readSchDesign(l *Loader, path string) (*ir.Design, error) {
	f, err := l.Open(path)
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
	f, err := l.Open(path)
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

// readEDIF reads an EDIF netlist into the IR. The .edn/.edf/.edif extensions share it. They are all
// the same format under different conventional suffixes.
func readEDIF(l *Loader, path string) (*ir.Design, error) {
	f, err := l.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return edif.ReadWith(f, path, edif.Options{FlattenHierarchy: l != nil && l.FlattenHierarchy})
}

// readIPC2581 reads an IPC-2581 file. .xml is ambiguous, so sniff for the IPC-2581 root
// before committing to that reader.
func readIPC2581(l *Loader, path string) (*ir.Design, error) {
	f, err := l.Open(path)
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
func readIPC2581Board(l *Loader, path string) (*geom.BoardGeometry, error) {
	f, err := l.Open(path)
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

// readTelesis reads a flat Telesis netlist. The extension is unambiguous, so unlike .sch and .xml
// there is nothing to sniff.
func readTelesis(l *Loader, path string) (*ir.Design, error) {
	f, err := l.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return telesis.Read(f, path)
}

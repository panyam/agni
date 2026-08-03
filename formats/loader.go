package formats

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/classify"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/graph"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/kicad"
)

// Loader reads design files through the registry. It carries the configuration the readers
// need beyond the file itself (today: the --symbol-path search directories); every
// entrypoint (CLI commands, the serve Loader adapter, a future WASM shim) constructs one
// and shares the same dispatch.
type Loader struct {
	// SymbolPaths are the directories searched for .sym symbol files when netlisting or
	// drawing xschem/gEDA schematics (the schematic's own directory is always searched).
	SymbolPaths []string
}

// ReadDesign reads a design file into the netlist IR, picking the reader by extension.
func (l *Loader) ReadDesign(path string) (*ir.Design, error) {
	ext := lowerExt(path)
	f := byExt[ext]
	if f == nil || f.Design == nil {
		return nil, fmt.Errorf("no reader for %q files (have: %s)", ext, strings.Join(NetlistExts(), ", "))
	}
	d, err := f.Design(l, path)
	if err != nil {
		return nil, err
	}
	// Stamp the format-neutral per-instance net id here, once, for every reader (WS9): netgraph-based
	// readers already set it (a no-op), and a direct-IR reader like EDIF gets it from its connections.
	netgraph.StampNetIDs(d)
	// Classify every component into its device_classes set once at ingestion (WS3-071), so check reads
	// a normalized data fact instead of re-deriving the class from vendor strings on every model build.
	// Runs after readers finish, applying the process-level class lexicon (SetActiveClassVocab), so a
	// --conventions class override must be installed before this call.
	classify.Stamp(d)
	// Stamp each net's role SET (rail / ground / feedback) from the active naming lexicon once at
	// ingestion (WS3-072), so the core reads a normalized net.role fact instead of re-running name
	// matching per-net per-rule. Same ordering contract as the class stamp: a --conventions role
	// override (SetActiveRoleVocab) must be installed before this call.
	classify.StampNetRoles(d)
	// Fill POWER_IN on supply pins a reader left under-typed (WS3-072 PR2): EDIF's port grammar carries
	// only INPUT/OUTPUT/INOUT, so a VDD pin reads as plain INPUT; this promotes it so the power-pin rule
	// family works format-neutrally on PinDir == POWER_IN. A no-op for KiCad/gEDA (already typed).
	classify.StampPowerInPins(d)
	return d, nil
}

// BoardGeometry reads a design's board-geometry sidecar (WS1-006), or (nil, nil) when the
// format carries no board layout — absence is a normal state for a netlist-only source,
// distinct from a board-bearing file that fails to parse (an error).
func (l *Loader) BoardGeometry(path string) (*geom.BoardGeometry, error) {
	f := byExt[lowerExt(path)]
	if f == nil || f.Board == nil {
		return nil, nil
	}
	return f.Board(l, path)
}

// FaithfulGeometry reads a design's ingested schematic geometry. An extension with no
// geometry reader returns a format-aware error that points at --layout=grid, distinguishing
// formats with no schematic view at all from ones agni simply does not read geometry from
// yet.
func (l *Loader) FaithfulGeometry(path string) (*geom.SchematicGeometry, error) {
	ext := lowerExt(path)
	f := byExt[ext]
	if f == nil || f.Geometry == nil {
		return nil, faithfulUnavailable(ext)
	}
	return f.Geometry(l, path)
}

// ResolveGeometry produces the geometry to render for the chosen layout. LayoutFaithful
// reads the design's ingested schematic geometry; any other value is an auto-layout
// computed from the netlist IR (the set is graph.Strategies). The two paths read disjoint
// file types, so a mismatch (e.g. faithful on a netlist, or auto-layout on a geometry-only
// .eds) returns a guiding error.
func (l *Loader) ResolveGeometry(path, layout string, reg *graph.Registry, symbols string) (*geom.SchematicGeometry, error) {
	if layout == LayoutFaithful {
		return l.FaithfulGeometry(path)
	}
	d, err := l.ReadDesign(path)
	if err != nil {
		return nil, err
	}
	source, err := l.SymbolSource(path, symbols, reg)
	if err != nil {
		return nil, err
	}
	return graph.LayoutWith(d, layout, graph.WithSymbolSource(source))
}

// SymbolSource builds the auto-layout node symbol source for the chosen symbols mode: the
// classification registry (synthetic glyphs) by default, or a FaithfulSource over the
// design's own geometry when SymbolsFaithful is requested. Faithful needs the design's own
// geometry; a netlist-only format (.edn, IPC-2581) has none, so faithful GRACEFULLY FALLS
// BACK to glyphs rather than erroring — a viewer that still has faithful selected from a
// previous file draws the netlist graph with glyph nodes instead of failing the request.
func (l *Loader) SymbolSource(path, symbols string, reg *graph.Registry) (graph.SymbolSource, error) {
	if reg == nil {
		reg = graph.DefaultRegistry()
	}
	if symbols == SymbolsFaithful {
		if fg, err := l.FaithfulGeometry(path); err == nil {
			return graph.NewFaithfulSource(fg, reg), nil
		}
	}
	return reg, nil
}

// ConversionReport reads the file's netlist and classifies it under the chosen symbol
// source, returning how each component maps to a drawn node. Shared by the CLI (--report)
// and the serve API (GetLayoutReport).
func (l *Loader) ConversionReport(path, symbols string, reg *graph.Registry) (*graph.ConversionReport, error) {
	d, err := l.ReadDesign(path)
	if err != nil {
		return nil, err
	}
	source, err := l.SymbolSource(path, symbols, reg)
	if err != nil {
		return nil, err
	}
	return graph.BuildReport(d, source), nil
}

// symbolOpener builds a resolver that finds a symbol reference (e.g. "res.sym" or
// "devices/res.sym") by searching the schematic's own directory first, then each
// SymbolPaths entry. It tries the reference as written and by basename directly, then falls
// back to a recursive search of each dir's subtree by basename — gEDA/Lepton libraries are
// organized in categorized subdirs (analog/, power/, ...) and reference symbols by bare name,
// so a --symbol-path pointed at a library ROOT resolves them. The subtree index is built once
// per opener (lazily) and reused; earlier dirs and shallower matches win. Passed to the
// xschem/gEDA readers, which own no file I/O themselves (CONSTRAINTS C1).
func (l *Loader) symbolOpener(schPath string) func(string) ([]byte, error) {
	dirs := append([]string{filepath.Dir(schPath)}, l.SymbolPaths...)
	var index map[string]string // basename -> full path; nil until the first recursive miss
	return func(symref string) ([]byte, error) {
		base := filepath.Base(symref)
		for _, d := range dirs {
			for _, cand := range []string{filepath.Join(d, symref), filepath.Join(d, base)} {
				if data, err := os.ReadFile(cand); err == nil {
					return data, nil
				}
			}
		}
		if index == nil {
			index = indexSymFiles(dirs)
		}
		if p, ok := index[base]; ok {
			return os.ReadFile(p)
		}
		return nil, fmt.Errorf("symbol %q not found (searched %d dir(s) and their subtrees; pass --symbol-path)", symref, len(dirs))
	}
}

// indexSymFiles walks each dir's subtree once and maps every .sym file's basename to its path.
// Dirs are indexed in order, first write wins, so precedence follows dir order (schematic dir,
// then each --symbol-path). It only decides among SUBTREE matches: the direct search in
// symbolOpener runs first, so a top-level file in an earlier dir always wins over any subdir
// match. A missing or unreadable dir is skipped.
func indexSymFiles(dirs []string) map[string]string {
	m := map[string]string{}
	for _, d := range dirs {
		filepath.WalkDir(d, func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || !strings.HasSuffix(path, ".sym") {
				return nil
			}
			if b := filepath.Base(path); m[b] == "" {
				m[b] = path
			}
			return nil
		})
	}
	return m
}

// kicadSymOpener builds the external .kicad_sym resolver for a schematic (WS1-016):
// the project's own sym-lib-table (beside the schematic; ${KIPRJMOD} = that directory)
// is consulted first — project truth needs no flag, like the sheet opener — then each
// --symbol-path directory is searched for <Library>.kicad_sym by nickname, which is how
// table entries naming installed-lib env vars (${KICAD9_SYMBOL_DIR}/...) and tableless
// projects resolve. The readers own no file I/O (C1).
func (l *Loader) kicadSymOpener(schPath string) func(lib string) ([]byte, error) {
	dir := filepath.Dir(schPath)
	var table map[string]string
	if data, err := os.ReadFile(filepath.Join(dir, "sym-lib-table")); err == nil {
		table = kicad.ParseSymLibTable(data, dir)
	}
	return func(lib string) ([]byte, error) {
		if uri, ok := table[lib]; ok && !strings.Contains(uri, "${") {
			if data, err := os.ReadFile(uri); err == nil {
				return data, nil
			}
		}
		for _, d := range append([]string{dir}, l.SymbolPaths...) {
			if data, err := os.ReadFile(filepath.Join(d, lib+".kicad_sym")); err == nil {
				return data, nil
			}
		}
		return nil, fmt.Errorf("kicad symbol library %q not found (sym-lib-table + %d dir(s); pass --symbol-path)", lib, 1+len(l.SymbolPaths))
	}
}

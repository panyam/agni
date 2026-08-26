package formats

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/graph"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/geomath"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/readers/kicad"
)

// Loader reads design files through the registry. It carries the configuration the readers
// need beyond the file itself (today: the --symbol-path search directories); every
// entrypoint (CLI commands, the serve Loader adapter, a future WASM shim) constructs one
// and shares the same dispatch.
type Loader struct {
	// SymbolPaths are the directories searched for .sym symbol files when netlisting or
	// drawing xschem/gEDA schematics (the schematic's own directory is always searched).
	SymbolPaths []string
	// Lexicon is the naming vocabulary this loader's reads are stamped with: which net names are
	// rails / grounds / feedback nodes, which pin names are supplies, and the device-class token
	// hints (WS3-106). Nil means the process defaults, so a loader that declares no project
	// convention behaves exactly as before.
	//
	// It lives here, beside SymbolPaths, because it is read configuration: the stamps below turn it
	// into design DATA once, and rules then read the data. Carrying it per-loader rather than in a
	// package global is what lets two designs be read with different project conventions in one
	// process, which a served request needs and a mutable global cannot give.
	Lexicon *classify.Lexicon
	// FS, when non-nil, is what every read in this package resolves against, so a host with no
	// filesystem — a WASM build, an embedder holding designs in memory, a test — reads through the
	// same registry, the same extension dispatch, and the same post-read stamps as an on-disk read
	// (WS1-049). Nil means the host filesystem, so an os-backed caller is unaffected and its paths
	// keep their host form.
	//
	// A non-nil FS makes every path an fs.ValidPath name: slash-separated, unrooted, no "..". That
	// applies to the SymbolPaths entries and to the sibling references the multi-file formats
	// resolve (a KiCad sub-sheet's Sheetfile, a .kicad_sym library, an xschem/gEDA .sym), because
	// those are joined against the design's own directory in this same name space.
	//
	// It carries an fs.FS rather than a bytes entry point because bytes cannot express the
	// multi-file formats at all: a KiCad hierarchy is a root plus its sub-sheets plus its symbol
	// libraries, and the readers reach them through opener closures this loader supplies. One FS
	// covers os, in-memory, embedded, and archive hosts with no second dispatch table.
	FS fs.FS
	// SourceName maps a path this loader opened onto the name provenance should record for it. Nil
	// records the path verbatim, which is what a host reading through FS wants: an fs.ValidPath is
	// already unrooted and portable.
	//
	// It exists for the host that is NOT reading through an FS. The CLI resolves its argument to an
	// absolute path before opening it, so every locator in the read carried the machine's directory
	// layout: `--format json` published it, `--results-out` STORED it, and an archived run therefore
	// embedded the filesystem of whatever produced it. A results document is meant to be mailed and
	// re-read by someone holding neither the design nor this build (see architecture/checks-contract),
	// which a host path quietly makes untrue.
	//
	// It is a function rather than a base-path string because only the caller knows what a path
	// should be CALLED: the CLI holds a mount table and can say which mount contains this file, and
	// nothing in this package can. `CheckReport.source` already promises a mount-relative path, so a
	// locator naming a file inside that design follows the same convention.
	SourceName func(string) string
}

// Open reads one file in this loader's name space: from FS when it carries one, else from the host
// filesystem. A registered reader MUST reach its bytes through this (or ReadFile) rather than
// calling os directly, or it works on a server and fails in every host that has no filesystem —
// that is the whole contract the FS field buys. It is exported for exactly that reason: the
// registry is a public extension point (see Register), so an out-of-module reader in the overlay
// needs the same door the built-in readers use.
//
// *os.File already satisfies fs.File, so the host branch needs no wrapper and a caller that sniffs
// a header keeps the io.Reader it peeks at. The returned file is the caller's to Close.
func (l *Loader) Open(name string) (fs.File, error) {
	if l == nil || l.FS == nil {
		return os.Open(name)
	}
	return l.FS.Open(name)
}

// ReadFile is Open plus a full read, for the readers that want bytes rather than a stream: anything
// parsed twice, and the multi-file walks that hand whole sub-files to a parser.
func (l *Loader) ReadFile(name string) ([]byte, error) {
	if l == nil || l.FS == nil {
		return os.ReadFile(name)
	}
	return fs.ReadFile(l.FS, name)
}

// Sibling resolves a reference made RELATIVE to a design file (a sub-sheet's Sheetfile, a symbol
// library, a companion sidecar) into a name Open and ReadFile accept. Readers must build sibling
// names with this rather than path/filepath directly, because the separator rules differ by host:
// an fs.FS name space is always slash-separated, and the host filesystem uses the platform's. The
// difference is invisible on unix, where the two agree, and breaks every sibling lookup on Windows.
func (l *Loader) Sibling(name, rel string) string {
	return l.join(l.dir(name), rel)
}

// dir and join split a path in this loader's name space. Under an FS that is always slash
// separated (path), and on the host filesystem it is the platform's (filepath) — the same
// distinction fs.FS itself draws, hoisted to one place so no call site has to remember it. Getting
// this wrong is invisible on unix, where the two agree, and breaks every sibling lookup on Windows.
func (l *Loader) dir(name string) string {
	if l == nil || l.FS == nil {
		return filepath.Dir(name)
	}
	return path.Dir(name)
}

func (l *Loader) join(elem ...string) string {
	if l == nil || l.FS == nil {
		return filepath.Join(elem...)
	}
	return path.Join(elem...)
}

// base takes the final element of a path in this loader's name space. A symbol reference is read
// out of a schematic FILE, so it may be spelled with either separator regardless of the host;
// ToSlash normalizes that before the FS name space sees it.
func (l *Loader) base(name string) string {
	if l == nil || l.FS == nil {
		return filepath.Base(name)
	}
	return path.Base(filepath.ToSlash(name))
}

// walkDir walks a directory subtree in this loader's name space. Under an FS a root of "." is the
// whole tree; on the host filesystem it is the given directory. A missing or unreadable root is
// skipped by the caller's walk function, matching filepath.WalkDir's error-in, nil-out convention.
func (l *Loader) walkDir(root string, fn fs.WalkDirFunc) error {
	if l == nil || l.FS == nil {
		return filepath.WalkDir(root, fn)
	}
	return fs.WalkDir(l.FS, root, fn)
}

// lexicon is the naming vocabulary this loader stamps with. A nil *Loader is a supported caller
// (ResolveGeometry is reached through one), and a nil Lexicon means the process defaults, so both
// degrade to the built-in vocabularies rather than panicking.
func (l *Loader) lexicon() *classify.Lexicon {
	if l == nil {
		return nil
	}
	return l.Lexicon
}

// sourceName is SourceName with this package's nil-loader tolerance, matching lexicon() above.
func (l *Loader) sourceName() func(string) string {
	if l == nil {
		return nil
	}
	return l.SourceName
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
	// Every locator recorded during the read is rewritten to the name this loader was told to call
	// its sources, before any stamp reads one and before the design reaches a caller.
	relocateSources(d, l.sourceName())
	netgraph.StampNetIDs(d)
	// Classify every component into its device_classes set once at ingestion (WS3-071), so check reads
	// a normalized data fact instead of re-deriving the class from vendor strings on every model build.
	// Runs after readers finish, against THIS loader's lexicon (WS3-106) — the vocabulary arrives with
	// the read, so there is no install-before-this-call ordering to get wrong.
	lex := l.lexicon()
	lex.Stamp(d)
	// Stamp each net's role SET (rail / ground / feedback) from the same lexicon once at ingestion
	// (WS3-072), so the core reads a normalized net.role fact instead of re-running name matching
	// per-net per-rule.
	lex.StampNetRoles(d)
	// Read each component's VALUE into a machine-comparable Quantity once at ingestion (WS3-118), from
	// whatever attribute its format spelled it in. It runs AFTER Stamp because the bare-number unit
	// convention is keyed on the device class this pass has just filled: a bare "100" means ohms only
	// once the component is known to be a resistor.
	lex.StampValues(d)
	// Fill POWER_IN on supply pins a reader left under-typed (WS3-072 PR2): EDIF's port grammar carries
	// only INPUT/OUTPUT/INOUT, so a VDD pin reads as plain INPUT; this promotes it so the power-pin rule
	// family works format-neutrally on PinDir == POWER_IN. A no-op for KiCad/gEDA (already typed).
	lex.StampPowerInPins(d)
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
	b, err := f.Board(l, path)
	if err != nil {
		return nil, err
	}
	relocateSources(b, l.sourceName())
	return b, nil
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
	g, err := f.Geometry(l, path)
	if err != nil {
		return nil, err
	}
	// What this read could not draw, recorded here because this is where the geometry is produced and
	// where the symbol libraries were (or were not) found. Computing it downstream would mean a second
	// join that can disagree with the renderer's (agni issue 354).
	geomath.MarkUndrawn(g)
	relocateSources(g, l.sourceName())
	return g, nil
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
	dirs := append([]string{l.dir(schPath)}, l.SymbolPaths...)
	var index map[string]string // basename -> full path; nil until the first recursive miss
	return func(symref string) ([]byte, error) {
		base := l.base(symref)
		for _, d := range dirs {
			for _, cand := range []string{l.join(d, symref), l.join(d, base)} {
				if data, err := l.ReadFile(cand); err == nil {
					return data, nil
				}
			}
		}
		if index == nil {
			index = l.indexSymFiles(dirs)
		}
		if p, ok := index[base]; ok {
			return l.ReadFile(p)
		}
		return nil, fmt.Errorf("symbol %q not found (searched %d dir(s) and their subtrees; pass --symbol-path)", symref, len(dirs))
	}
}

// indexSymFiles walks each dir's subtree once and maps every .sym file's basename to its path.
// Dirs are indexed in order, first write wins, so precedence follows dir order (schematic dir,
// then each --symbol-path). It only decides among SUBTREE matches: the direct search in
// symbolOpener runs first, so a top-level file in an earlier dir always wins over any subdir
// match. A missing or unreadable dir is skipped.
func (l *Loader) indexSymFiles(dirs []string) map[string]string {
	m := map[string]string{}
	for _, d := range dirs {
		l.walkDir(d, func(name string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() || !strings.HasSuffix(name, ".sym") {
				return nil
			}
			if b := l.base(name); m[b] == "" {
				m[b] = name
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
	dir := l.dir(schPath)
	var table map[string]string
	if data, err := l.ReadFile(l.join(dir, "sym-lib-table")); err == nil {
		table = kicad.ParseSymLibTable(data, dir)
	}
	return func(lib string) ([]byte, error) {
		if uri, ok := table[lib]; ok && !strings.Contains(uri, "${") {
			if data, err := l.ReadFile(uri); err == nil {
				return data, nil
			}
		}
		for _, d := range append([]string{dir}, l.SymbolPaths...) {
			if data, err := l.ReadFile(l.join(d, lib+".kicad_sym")); err == nil {
				return data, nil
			}
		}
		return nil, fmt.Errorf("kicad symbol library %q not found (sym-lib-table + %d dir(s); pass --symbol-path)", lib, 1+len(l.SymbolPaths))
	}
}

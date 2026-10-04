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

// Loader reads design files through the registry. It carries the read configuration the readers
// need beyond the file itself (symbol paths, naming vocabulary, file system, provenance naming,
// datasheet classes). Every entrypoint (CLI commands, the serve adapter, a future WASM shim)
// constructs one and shares the same dispatch.
type Loader struct {
	// SymbolPaths are the directories searched for .sym symbol files when netlisting or
	// drawing xschem/gEDA schematics (the schematic's own directory is always searched).
	SymbolPaths []string
	// Lexicon is the naming vocabulary this loader's reads are stamped with (WS3-106). Nil means the
	// process defaults. It is per-loader rather than a package global so one process can read two
	// designs under two projects' conventions; see classify.Lexicon.
	Lexicon *classify.Lexicon
	// FS, when non-nil, is what every read in this package resolves against, so a host with no
	// filesystem (a WASM build, an embedder holding designs in memory, a test) gets the same registry,
	// dispatch and post-read stamps as an on-disk read (WS1-049). Nil means the host filesystem, and
	// paths keep their host form.
	//
	// A non-nil FS makes every path an fs.ValidPath name (slash-separated, unrooted, no ".."). That
	// includes the SymbolPaths entries and the sibling references the multi-file formats resolve
	// against the design's own directory. It is an fs.FS rather than a bytes entry point because a
	// multi-file format such as a KiCad root plus its sub-sheets and symbol libraries cannot be one
	// byte slice. See docsite/content/build/format-reader.md.
	FS fs.FS
	// SourceName maps a path this loader opened onto the name provenance should record for it. Nil
	// records the path verbatim, which suits a host reading through FS, since an fs.ValidPath is
	// already unrooted.
	//
	// It is for the host that is NOT reading through an FS. The CLI opens absolute paths, so without
	// it every locator carries the machine's directory layout into `--format json` and into the
	// stored `--results-out` document, which is meant to be re-read elsewhere (#511,
	// docsite/content/architecture/checks-contract.md). A function rather than a base path, because
	// only the caller's mount table knows which mount contains a file, and `CheckReport.source`
	// already promises a mount-relative path.
	SourceName func(string) string
	// Touched, when set, records every name this loader opens, reads, walks or fails to find, so a
	// host can keep the read's result and later check it is still current (agni issue 895). A reader
	// that reaches its bytes around Open and ReadFile escapes it, which is one more reason they must
	// not.
	Touched *Touched
	// DeviceClassFor answers the vendor device_class a datasheet states for an MPN, or "" for a part
	// with no seeded spec. Nil means the read has no datasheet corpus, the ordinary case, and every
	// component is classified by convention alone.
	//
	// It is the DATASHEET evidence tier of the class stamp (agni issue 710), per-loader like Lexicon.
	// A FUNCTION rather than the param provider itself, so this package does not take on the
	// datasheet layer (C17). The pass needs one string per part, and the corpus, its loading and its
	// staleness rules stay with the caller.
	DeviceClassFor func(mpn string) string
}

// Open reads one file in this loader's name space, from FS when it carries one and from the host
// filesystem otherwise. A registered reader MUST reach its bytes through this (or ReadFile) rather
// than calling os directly, or it works on a server and fails in every host with no filesystem.
// Exported because the registry is a public extension point (see Register), so an out-of-module
// reader needs the same door the built-in readers use.
//
// *os.File already satisfies fs.File, so a caller that sniffs a header keeps the io.Reader it peeks
// at. The returned file is the caller's to Close.
func (l *Loader) Open(name string) (fs.File, error) {
	if l == nil {
		return os.Open(name)
	}
	l.Touched.note(l.FS, name)
	if l.FS == nil {
		return os.Open(name)
	}
	return l.FS.Open(name)
}

// ReadFile is Open plus a full read, for the readers that want bytes rather than a stream: anything
// parsed twice, and the multi-file walks that hand whole sub-files to a parser.
func (l *Loader) ReadFile(name string) ([]byte, error) {
	if l == nil {
		return os.ReadFile(name)
	}
	l.Touched.note(l.FS, name)
	if l.FS == nil {
		return os.ReadFile(name)
	}
	return fs.ReadFile(l.FS, name)
}

// Sibling resolves a reference made RELATIVE to a design file (a sub-sheet's Sheetfile, a symbol
// library, a companion sidecar) into a name Open and ReadFile accept. Readers must build sibling
// names with this rather than path/filepath directly, because an fs.FS name space is always
// slash-separated and the host filesystem uses the platform's separator. Getting it wrong is
// invisible on unix, where the two agree, and breaks every sibling lookup on Windows.
func (l *Loader) Sibling(name, rel string) string {
	return l.join(l.dir(name), rel)
}

// dir and join split and join a path in this loader's name space, slash-separated (path) under an
// FS and the platform's (filepath) on the host filesystem. See Sibling.
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
	if l != nil && l.Touched != nil {
		// Every directory the walk visits, the root included even when it is missing, since a
		// symbol found by name in a walked tree changes when a file is added there.
		l.Touched.note(l.FS, root)
		inner := fn
		fn = func(name string, e fs.DirEntry, err error) error {
			if e != nil && e.IsDir() {
				l.Touched.note(l.FS, name)
			}
			return inner(name, e, err)
		}
	}
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

// deviceClassFor is DeviceClassFor with this package's nil-loader tolerance, matching lexicon()
// above. A nil loader means no corpus, which is what a caller reading with the package defaults has.
func (l *Loader) deviceClassFor() func(string) string {
	if l == nil {
		return nil
	}
	return l.DeviceClassFor
}

// sourceName is SourceName with this package's nil-loader tolerance, matching lexicon() above.
func (l *Loader) sourceName() func(string) string {
	if l == nil {
		return nil
	}
	return l.SourceName
}

// ReadDesign reads a design file into the netlist IR, picking the reader by extension, then runs
// the format-neutral ingestion passes in order. Two of the orderings fail silently when wrong. Stamp
// REPLACES the device-class set, so the datasheet class pass must run after it, and that pass joins
// on the MPN StampMPN fills, so it must run after that too. See
// docsite/content/architecture/ingestion-and-ir.md#derived-fields-and-the-tiers-that-fill-them.
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
	// Rename every locator to the name this loader was told to call its sources, before any stamp
	// reads one and before the design reaches a caller.
	relocateSources(d, l.sourceName())
	// The format-neutral per-instance net id (WS9). A no-op for netgraph-based readers, which set it
	// already; a direct-IR reader like EDIF gets it from its connections.
	netgraph.StampNetIDs(d)
	// Device classes (WS3-071), against THIS loader's lexicon (WS3-106), so the vocabulary arrives
	// with the read and there is no install-before-read ordering to get wrong.
	lex := l.lexicon()
	lex.Stamp(d)
	// Each net's role SET from the same lexicon (WS3-072); see Lexicon.StampNetRoles.
	lex.StampNetRoles(d)
	// Component values into comparable Quantities (WS3-118). AFTER Stamp, because the bare-number
	// unit is keyed on the device class, so a bare "100" means ohms only once the component is known
	// to be a resistor.
	lex.StampValues(d)
	// POWER_IN on supply pins a reader left under-typed (WS3-072 PR2); see Lexicon.StampPowerInPins.
	lex.StampPowerInPins(d)
	// Fill ir.Component.mpn from the component's own attributes or its part type (agni issue 519).
	// Not a lexicon pass, since a part number is an identifier the source states and not a name this
	// engine interprets.
	classify.StampMPN(d)
	// The classes only a DATASHEET can establish (agni issue 710, C9's evidence-tier variant). LAST
	// of the class passes, for the two orderings in the doc comment. A read with no corpus skips it
	// and is unchanged by it.
	classify.StampClassesFromSpecs(d, l.deviceClassFor())
	return d, nil
}

// BoardGeometry reads a design's board-geometry sidecar (WS1-006), or (nil, nil) when the
// format carries no board layout. Absence is a normal state for a netlist-only source, distinct
// from a board-bearing file that fails to parse (an error).
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
	// What this read could not draw, recorded where the geometry is produced and the symbol libraries
	// were (or were not) found. Computing it downstream would be a second join that can disagree with
	// the renderer's (agni issue 354).
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
// BACK to glyphs rather than erroring. A viewer that still has faithful selected from a
// previous file then draws the netlist graph with glyph nodes instead of failing the request.
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
// back to a recursive search of each dir's subtree by basename. gEDA/Lepton libraries are
// organized in categorized subdirs (analog/, power/, ...) and reference symbols by bare name,
// so this lets a --symbol-path pointed at a library ROOT resolve them. The subtree index is built once
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
// then each --symbol-path). It only decides among SUBTREE matches, since the direct search in
// symbolOpener runs first and a top-level file in an earlier dir always wins over any subdir
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
// is consulted first and needs no flag, like the sheet opener. Then each --symbol-path
// directory is searched for <Library>.kicad_sym by nickname, which is how table entries
// naming installed-lib env vars (${KICAD9_SYMBOL_DIR}/...) and tableless projects resolve.
// The readers own no file I/O (C1).
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

// SymbolsFor maps the services' faithful-symbols bool onto the --symbols value a geometry read
// takes.
func SymbolsFor(faithful bool) string {
	if faithful {
		return SymbolsFaithful
	}
	return SymbolsGlyph
}

// Companion returns a sibling <stem>.eds schematic for a NETLIST design, or "" when the design
// already carries its own geometry (an .eds or .kicad_sch draws itself) or no sibling exists. It
// checks names only and never parses the sibling. A netlist with one draws that schematic instead of
// an auto-layout, while checks and queries still read the netlist, joined by net name (C21).
//
// It resolves through this loader's name space, so a server reading host paths and the wasm engine
// reading an in-memory FS find the same companion for the same tree.
func (l *Loader) Companion(name string) string {
	if HasFaithful(name) {
		return ""
	}
	sib := l.Sibling(name, strings.TrimSuffix(l.base(name), l.ext(name))+".eds")
	if sib == name {
		return ""
	}
	f, err := l.Open(sib)
	if err != nil {
		return ""
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil || st.IsDir() {
		return ""
	}
	return sib
}

// ext takes the extension of a path in this loader's name space. See Sibling.
func (l *Loader) ext(name string) string {
	if l == nil || l.FS == nil {
		return filepath.Ext(name)
	}
	return path.Ext(name)
}

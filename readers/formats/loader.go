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

// Loader reads design files through the registry. It holds the read configuration the readers need
// beyond the file itself. That covers symbol paths, naming vocabulary, file system, provenance naming
// and datasheet classes. Every entrypoint constructs one and shares the same dispatch. The CLI
// commands do, the serve adapter does, and a future WASM shim would.
type Loader struct {
	// SymbolPaths are the directories searched for .sym symbol files when netlisting or drawing
	// xschem/gEDA schematics. The schematic's own directory is always searched too.
	SymbolPaths []string
	// Lexicon is the naming vocabulary this loader stamps its reads with (WS3-106). Nil means the
	// process defaults. It is per-loader rather than a package global, so one process can read two
	// designs under two projects' conventions. See classify.Lexicon.
	Lexicon *classify.Lexicon
	// FS names the file system every read in this package resolves against. Set it for a host with
	// no filesystem, such as a WASM build, an embedder holding designs in memory, or a test. That host
	// then gets the same registry, dispatch and post-read stamps as an on-disk read (WS1-049). Nil
	// means the host filesystem, and paths keep their host form.
	//
	// A non-nil FS makes every path an fs.ValidPath name (slash-separated, unrooted, no ".."). That
	// includes the SymbolPaths entries. It also includes the sibling references that the multi-file
	// formats resolve against the design's own directory. It is an fs.FS rather than a bytes entry
	// point. A multi-file format such as a KiCad root plus its sub-sheets and symbol libraries cannot
	// be one byte slice. See docsite/content/build/format-reader.md.
	FS fs.FS
	// SourceName maps a path this loader opened onto the name provenance should record for it. Nil
	// records the path verbatim. That suits a host reading through FS, since an fs.ValidPath is
	// already unrooted.
	//
	// It is for the host that is NOT reading through an FS. The CLI opens absolute paths. Without
	// SourceName, every locator would copy the machine's directory layout into `--format json` and
	// into the stored `--results-out` document. That document is meant to be re-read elsewhere (#511,
	// docsite/content/architecture/checks-contract.md). SourceName is a function rather than a base
	// path. Only the caller's mount table knows which mount contains a file, and `CheckReport.source`
	// already promises a mount-relative path.
	SourceName func(string) string
	// Touched, when set, records every name this loader opens, reads, walks or fails to find. A host
	// can then keep the read's result and later check it is still current (agni issue 895). A reader
	// that reaches its bytes around Open and ReadFile escapes the record. That is one more reason
	// readers must not do so.
	Touched *Touched
	// DeviceClassFor answers the vendor device_class a datasheet states for an MPN, or "" for a part
	// with no seeded spec. Nil means the read has no datasheet corpus, the ordinary case, and every
	// component is classified by convention alone.
	//
	// It is the DATASHEET evidence tier of the class stamp (agni issue 710), per-loader like Lexicon.
	// It is a FUNCTION rather than the param provider itself, so this package does not take on the
	// datasheet layer (C17). The pass needs one string per part. The corpus, its loading and its
	// staleness rules belong to the caller.
	DeviceClassFor func(mpn string) string
	// ParseOnly makes ReadDesign stop after ParseDesign. The passes that read configuration are then
	// left to the caller (agni issue 1061). A host sets it through a read option rather than a method
	// of its own, so every host that reads through LoaderIn supports it without a change.
	ParseOnly bool
	// FlattenHierarchy makes a hierarchical netlist read as its whole schematic rather than as its top
	// cell alone (agni issue 718). The read walks into every block cell the top cell places. Only the
	// EDIF reader has blocks to walk today. It changes what the design contains. So a host that keys a
	// parse must include it in the key, as service.CachingLoader does.
	FlattenHierarchy bool
}

// Open reads one file in this loader's name space. It reads from FS when the loader has one, and from
// the host filesystem otherwise. A registered reader MUST reach its bytes through Open or ReadFile
// rather than calling os directly. Otherwise it works on a server and fails in every host with no
// filesystem. Open is exported because the registry is a public extension point (see Register). An
// out-of-module reader needs the same door the built-in readers use.
//
// *os.File already satisfies fs.File, so a caller that sniffs a header keeps the io.Reader it peeks
// at. The caller must Close the returned file.
func (l *Loader) Open(name string) (fs.File, error) {
	if l == nil {
		return os.Open(name) // #nosec G304 -- opens the path its host hands it; a server resolves it through mounts first
	}
	l.Touched.note(l.FS, name)
	if l.FS == nil {
		return os.Open(name) // #nosec G304 -- opens the path its host hands it; a server resolves it through mounts first
	}
	return l.FS.Open(name)
}

// ReadFile is Open plus a full read, for the readers that want bytes rather than a stream. Anything
// parsed twice wants bytes, and so do the multi-file walks that hand whole sub-files to a parser.
func (l *Loader) ReadFile(name string) ([]byte, error) {
	if l == nil {
		return os.ReadFile(name) // #nosec G304 -- opens the path its host hands it; a server resolves it through mounts first
	}
	l.Touched.note(l.FS, name)
	if l.FS == nil {
		return os.ReadFile(name) // #nosec G304 -- opens the path its host hands it; a server resolves it through mounts first
	}
	return fs.ReadFile(l.FS, name)
}

// Sibling resolves a reference made RELATIVE to a design file into a name Open and ReadFile accept.
// Such a reference is a sub-sheet's Sheetfile, a symbol library, or a companion sidecar. Readers must
// build sibling names with Sibling rather than path/filepath directly. An fs.FS name space is always
// slash-separated, and the host filesystem uses the platform's separator. On unix the two agree, so a
// mistake is invisible there. On Windows it breaks every sibling lookup.
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
// out of a schematic FILE, so it may be spelled with either separator regardless of the host.
// ToSlash normalizes it before the FS name space sees it.
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
		// Note every directory the walk visits, and the root even when it is missing. A file added
		// to a walked tree can change which symbol a name finds.
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

// lexicon is the naming vocabulary this loader stamps with. A nil *Loader is a supported caller, and
// ResolveGeometry is reached through one. A nil Lexicon means the process defaults. So both degrade
// to the built-in vocabularies rather than panicking.
func (l *Loader) lexicon() *classify.Lexicon {
	if l == nil {
		return nil
	}
	return l.Lexicon
}

// deviceClassFor is DeviceClassFor with this package's nil-loader tolerance, matching lexicon()
// above. A nil loader means no corpus. A caller reading with the package defaults has no corpus.
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

// ReadDesign reads a design file into the netlist IR, picking the reader by extension. Then it runs
// the format-neutral ingestion passes in order. It is ParseDesign followed by DeriveDesign, or
// ParseDesign alone under ParseOnly. Two of the orderings fail silently when wrong. Stamp REPLACES
// the device-class set, so the datasheet class pass must run after it. That pass joins on the MPN,
// and StampMPN fills the MPN, so the pass must run after StampMPN too. See
// docsite/content/architecture/ingestion-and-ir.md#derived-fields-and-the-tiers-that-fill-them.
func (l *Loader) ReadDesign(path string) (*ir.Design, error) {
	d, err := l.ParseDesign(path)
	if err != nil || (l != nil && l.ParseOnly) {
		return d, err
	}
	DeriveDesign(d, l.lexicon(), l.deviceClassFor())
	return d, nil
}

// ParseDesign is the half of ReadDesign that reads no configuration. It runs the reader, renames the
// locators and stamps the per-instance net ids. Its result depends on the files alone, and on
// SymbolPaths, which decide which files are read. So a host can keep it across projects and
// conventions and derive it again for each (agni issue 1061). A caller that wants a usable design
// calls ReadDesign, or calls DeriveDesign on what this returns.
func (l *Loader) ParseDesign(path string) (*ir.Design, error) {
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
	// The format-neutral per-instance net id (WS9). It is a no-op for netgraph-based readers, which
	// set it already. A direct-IR reader like EDIF gets it from its connections.
	netgraph.StampNetIDs(d)
	return d, nil
}

// DeriveDesign runs the passes of ReadDesign that read configuration, in place over a parsed design.
// From lex it stamps part numbers, device classes, power pins, net roles and values. A nil lex means
// the process defaults. Then it stamps the classes a datasheet establishes, from deviceClassFor. A
// nil deviceClassFor means no corpus. It expects a design straight from ParseDesign. Stamp replaces
// the class set, and a second derive over its own output is not a supported input.
func DeriveDesign(d *ir.Design, lex *classify.Lexicon, deviceClassFor func(mpn string) string) {
	// Fill ir.Component.mpn from the component's own attributes or its part type (agni issue 519),
	// under the lexicon's attribute.mpn keys (agni 1285). The datasheet class pass joins on it, so
	// this runs first.
	lex.StampMPN(d)
	// Device classes (WS3-071), against the read's lexicon (WS3-106). The vocabulary arrives with
	// the read, so no install-before-read ordering can go wrong.
	lex.Stamp(d)
	// POWER_IN on supply pins a reader left under-typed (WS3-072 PR2). See Lexicon.StampPowerInPins.
	// It runs BEFORE the roles. A rail can be inferred from the supply pins it feeds, and that
	// inference reads the filled types.
	lex.StampPowerInPins(d)
	// Each net's role SET from the same lexicon (WS3-072); see Lexicon.StampNetRoles.
	lex.StampNetRoles(d)
	// Component values into comparable Quantities (WS3-118). It runs AFTER Stamp, because the
	// bare-number unit is keyed on the device class. A bare "100" means ohms only once the component
	// is known to be a resistor.
	lex.StampValues(d)
	// The classes only a DATASHEET can establish (agni issue 710, C9's evidence-tier variant). It is
	// the LAST class pass, for the two orderings in ReadDesign's comment. A read with no corpus skips
	// it and is unchanged by it.
	classify.StampClassesFromSpecs(d, deviceClassFor)
}

// BoardGeometry reads a design's board-geometry sidecar (WS1-006). It returns (nil, nil) when the
// format has no board layout. Absence is a normal state for a netlist-only source. A board file
// that fails to parse is different, and returns an error.
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

// FaithfulGeometry reads a design's ingested schematic geometry. For an extension with no geometry
// reader it returns a format-aware error that points at --layout=grid. The error tells a format with
// no schematic view at all apart from one agni simply does not read geometry from yet.
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
	// Record what this read could not draw. The geometry is produced here, and the symbol libraries
	// were found or missed here. Computing it downstream would be a second join, and that join can
	// disagree with the renderer's (agni issue 354).
	geomath.MarkUndrawn(g)
	relocateSources(g, l.sourceName())
	return g, nil
}

// ResolveGeometry produces the geometry to render for the chosen layout. LayoutFaithful reads the
// design's ingested schematic geometry. Any other value is an auto-layout computed from the netlist
// IR, and graph.Strategies lists the set. The two paths read disjoint file types. So a mismatch
// returns a guiding error, such as faithful on a netlist, or auto-layout on a geometry-only .eds.
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

// SymbolSource builds the auto-layout node symbol source for the chosen symbols mode. By default it
// is the classification registry, which draws synthetic glyphs. With SymbolsFaithful it is a
// FaithfulSource over the design's own geometry. A netlist-only format (.edn, IPC-2581) has no
// geometry, so faithful GRACEFULLY FALLS BACK to glyphs rather than erroring. Say a viewer still has
// faithful selected from a previous file. It then draws the netlist graph with glyph nodes instead
// of failing the request.
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

// ConversionReport reads the file's netlist and classifies it under the chosen symbol source. It
// returns how each component maps to a drawn node. The CLI (--report) and the serve API
// (GetLayoutReport) share it.
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

// symbolOpener builds a resolver for a symbol reference such as "res.sym" or "devices/res.sym". It
// searches the schematic's own directory first, then each SymbolPaths entry. In each it tries the
// reference as written and then by basename. If both miss, it falls back to a recursive search of
// each dir's subtree by basename. gEDA/Lepton libraries sort symbols into subdirs (analog/, power/,
// and so on) and reference them by bare name. So a --symbol-path pointed at a library ROOT resolves
// them. Each opener builds the subtree index once, lazily, and reuses it. Earlier dirs and
// shallower matches win. The xschem/gEDA readers get the opener, since they own no file I/O
// themselves (CONSTRAINTS C1).
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

// indexSymFiles walks each dir's subtree once and maps every .sym file's basename to its path. It
// indexes dirs in order and the first write wins. So precedence follows dir order, the schematic dir
// first and then each --symbol-path. It only decides among SUBTREE matches. The direct search in
// symbolOpener runs first, so a top-level file in an earlier dir always wins over any subdir match.
// A missing or unreadable dir is skipped.
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

// kicadSymOpener builds the external .kicad_sym resolver for a schematic (WS1-016). It consults the
// project's own sym-lib-table first, and that needs no flag, like the sheet opener. The table sits
// beside the schematic, and ${KIPRJMOD} is that directory. Then it searches each --symbol-path
// directory for <Library>.kicad_sym by nickname. Table entries that name installed-lib env vars
// (${KICAD9_SYMBOL_DIR}/...) resolve that way, and so do tableless projects. The readers own no file
// I/O (C1).
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

// Companion returns a sibling <stem>.eds schematic for a NETLIST design. It returns "" when no
// sibling exists, or when the design already has its own geometry, as an .eds or .kicad_sch does. It
// checks names only and never parses the sibling. A netlist with a companion draws that schematic
// instead of an auto-layout. Checks and queries still read the netlist, joined by net name (C21).
//
// It resolves through this loader's name space. So a server reading host paths and the wasm engine
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

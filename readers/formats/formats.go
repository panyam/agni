// Package formats is the single registry of design-file formats the engine reads. For each
// extension it holds the UI label, the netlist reader, and the faithful-geometry reader. The CLI's
// extension switch, the geometry switch, the service's faithful/netlist extension sets and the
// file-tree labels all derive from it. So adding a reader is one entry here, plus CONSTRAINTS C10's
// example. File I/O lives in this package's Loader. The Loader is the edge-orchestration layer, and
// the CLI and the serve adapters share it (CONSTRAINTS C13). The readers themselves are still
// io.Reader-pure (C1).
//
// A registered reader reaches its bytes through Loader.Open / Loader.ReadFile / Loader.Sibling,
// never through os directly (WS1-049). So one Loader can serve a host filesystem or an in-memory
// fs.FS. See docsite/content/build/format-reader.md#the-registry-entry.
//
// This package is public rather than under internal/. It is the engine's reader extension point for
// the open-core split (WS12-003). A reader in another module registers itself with Register and gains
// every derived surface with no fork of the engine. The built-in readers use the same Register.
package formats

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// LayoutFaithful is the layout name that renders the design's own ingested geometry (the
// coordinates the designer placed), as opposed to an auto-computed layout. It is not a
// graph.Strategy, since it reads the geometry sidecar rather than computing from the IR.
const LayoutFaithful = "faithful"

// Symbol-source names for an auto-layout. An auto-layout draws nodes with synthetic classified
// glyphs by default. It can instead re-lay-out the design's own symbol artwork (partial-faithful,
// WS7-031).
const (
	SymbolsGlyph    = "glyph"
	SymbolsFaithful = "faithful"
)

// Format describes what the engine can do with one file extension. A nil reader means the
// capability is absent for that extension, as Geometry is for .edn and the board/netlist formats.
type Format struct {
	// Ext is the lowercase extension including the dot.
	Ext string
	// Name is the reader/format label the file-tree UI shows. Ambiguous extensions (.sch,
	// .xml) are labelled optimistically and sniffed for real at load time.
	Name string
	// Design reads the file into the netlist IR; nil when the format has no netlist.
	Design func(l *Loader, path string) (*ir.Design, error)
	// Geometry reads the file's ingested faithful schematic geometry; nil when the format
	// has none (FaithfulGeometry then returns the per-format guiding error).
	Geometry func(l *Loader, path string) (*geom.SchematicGeometry, error)
	// Board reads the file's board-geometry sidecar (WS1-006); nil when the format
	// carries no board layout. Consumers that want a board-aware check Model use
	// Loader.BoardGeometry and tolerate its absence.
	Board func(l *Loader, path string) (*geom.BoardGeometry, error)
	// DrawnNetlist marks a format whose netlist counts what its drawing draws rather than the
	// design's parts and nets. An EDIF schematic export (.eds) is one: it nets per-sheet segments
	// that nothing joins, so one board read 5219 components and 4572 nets off its .eds against 3980
	// and 1617 off its .edn. A design read from such a file reports no counts. A KiCad, xschem or
	// gEDA schematic is its design's netlist and leaves this false (agni issue 1342).
	DrawnNetlist bool
	// DrawnBy names the extensions of the schematic files that draw a design this format holds no
	// drawing of, such as ".eds" for an EDIF netlist. The viewer's note on a computed layout names
	// them (DrawnByExts). Empty when no format agni reads draws it, as for a board format.
	DrawnBy []string
	// Blocks returns the files a design file places as blocks, each as the file records it,
	// relative to the file's own folder and possibly in Windows form (".\MPU9250\MPU9250.DSN").
	// ProposeDesigns groups a dropped folder by it, so a block's file joins the design that places
	// it rather than proposing a design of its own (agni issue 1406). nil when the format keeps a
	// design in one file. An error says the bytes are not this format's design.
	Blocks func(b []byte) ([]string, error)
}

// byExt is the registry. The built-in entries are defined in registry.go; an out-of-module
// consumer (the open-core extension) adds its own via Register.
var byExt = map[string]*Format{}

// Register adds a format to the registry, keyed by its extension. A package in another module
// calls it from its init or from the composing binary's main. Its extension then resolves through
// ByExt, the CLI reader dispatch, the file-tree label and the supported-extensions error text.
//
// Register panics on a malformed entry or a duplicate extension, as image.RegisterFormat and
// sql.Register do. The first registration claims an extension. To override a built-in, a consumer
// composes a binary that does not register the built-in, rather than re-registering the extension.
func Register(f *Format) {
	switch {
	case f == nil:
		panic("formats: Register(nil)")
	case f.Ext == "" || f.Ext[0] != '.' || f.Ext != strings.ToLower(f.Ext):
		panic("formats: Register: Ext must be a lowercase extension including the dot, got " + f.Ext)
	case f.Name == "":
		panic("formats: Register: Name (the UI label) is required for " + f.Ext)
	case f.Design == nil && f.Geometry == nil && f.Board == nil:
		panic("formats: Register: " + f.Ext + " declares no capability (Design, Geometry, and Board all nil)")
	}
	if _, dup := byExt[f.Ext]; dup {
		panic("formats: duplicate registration for " + f.Ext)
	}
	byExt[f.Ext] = f
}

// ByExt returns the Format for a file name or path, or nil when no reader claims its
// extension (the file tree then shows it disabled).
func ByExt(name string) *Format {
	return byExt[lowerExt(name)]
}

// NameForExt returns the UI format label for a file name, or "" for an unclaimed extension.
func NameForExt(name string) string {
	if f := ByExt(name); f != nil {
		return f.Name
	}
	return ""
}

// HasNetlist reports whether the file's extension parses into a netlist IR (so checks,
// diff, and auto-layouts apply).
func HasNetlist(name string) bool {
	f := ByExt(name)
	return f != nil && f.Design != nil
}

// HasFaithful reports whether the file's extension carries ingested schematic geometry the
// viewer can render faithfully.
func HasFaithful(name string) bool {
	f := ByExt(name)
	return f != nil && f.Geometry != nil
}

// DrawnByExts returns the extensions that draw the file's design, from its format's DrawnBy, or nil.
func DrawnByExts(name string) []string {
	if f := ByExt(name); f != nil {
		return f.DrawnBy
	}
	return nil
}

// DrawingExts lists every registered extension with a faithful-geometry reader, sorted. These are
// the files a design.yaml can name as a companion to give a netlist its drawn sheets.
func DrawingExts() []string {
	var out []string
	for ext, f := range byExt {
		if f.Geometry != nil {
			out = append(out, ext)
		}
	}
	sort.Strings(out)
	return out
}

// HasBoard reports whether the file's extension holds board geometry, the copper, layers and
// courtyards that the board-tier rules run against. A design can declare several companion views,
// and only one of them may supply the board tier. A caller asks HasBoard to find that one.
func HasBoard(name string) bool {
	f := ByExt(name)
	return f != nil && f.Board != nil
}

// DrawnNetlist reports whether a file's format nets what its drawing draws (Format.DrawnNetlist),
// so its counts say nothing about the design. An unknown extension reports false.
func DrawnNetlist(name string) bool {
	f := ByExt(name)
	return f != nil && f.DrawnNetlist
}

// NetlistExts returns the sorted extensions with a netlist reader, for error text and help.
func NetlistExts() []string {
	var exts []string
	for ext, f := range byExt {
		// An ODB++ job's key names a matrix file inside a folder, not an extension anyone types,
		// so the list offers the job's archives instead.
		if f.Design != nil && ext != odbJobExt {
			exts = append(exts, ext)
		}
	}
	sort.Strings(exts)
	return exts
}

func lowerExt(name string) string {
	if k := odbKey(name); k != "" {
		return k
	}
	return strings.ToLower(filepath.Ext(name))
}

// faithfulUnavailable explains why a file has no faithful schematic geometry and points at
// the auto-layout fallback. The message differs by format. For a netlist or board file it says the
// format has no schematic view at all.
func faithfulUnavailable(ext string) error {
	switch ext {
	case ".edn":
		return fmt.Errorf(".edn is an EDIF netlist with no schematic geometry; use --layout=grid to draw the netlist graph")
	case ".xml", ".cvg":
		return fmt.Errorf("IPC-2581 (%s) is a board/netlist format with no schematic view; use --layout=grid to draw the netlist graph", ext)
	case ".kicad_pcb":
		return fmt.Errorf(".kicad_pcb is a board layout, not a schematic; the physical board renders as the 'board' sheet (or plain `agni render` on the file); --layout=grid draws the netlist graph")
	default:
		return fmt.Errorf("no faithful schematic geometry for %q (have: .eds); use --layout=grid for an auto-layout", ext)
	}
}

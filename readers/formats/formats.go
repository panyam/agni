// Package formats is the single registry of design-file formats the engine reads: for each
// extension, the UI label, the netlist reader, and the faithful-geometry reader. Every
// consumer that used to keep its own format table (the CLI's extension switch, the
// geometry switch, the service's faithful/netlist extension sets, the file-tree labels)
// derives from this one, so adding a reader is one entry here (plus CONSTRAINTS C10's
// example). File I/O lives in this package's Loader — it is the edge-orchestration layer
// the CLI and the serve adapters share (CONSTRAINTS C13); the readers themselves stay
// io.Reader-pure (C1).
//
// A registered reader reaches its bytes through Loader.Open / Loader.ReadFile / Loader.Sibling,
// never through os directly (WS1-049). That is what lets one Loader serve a host filesystem and an
// in-memory fs.FS with the same registry, the same dispatch, and the same post-read stamps; a
// reader that opens its own files works on a server and fails everywhere else.
//
// This package is public (not internal/) because it is the engine's reader extension point
// for the open-core split (WS12-003): a reader living in another module — a proprietary
// format in the private overlay — registers itself with Register and gains every derived
// surface with no fork of the engine. The built-in readers use the same Register, so the
// registry stays one table with one code path.
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
// graph.Strategy: it reads the geometry sidecar rather than computing from the IR.
const LayoutFaithful = "faithful"

// Symbol-source names for an auto-layout: draw nodes with synthetic classified glyphs
// (default) or with the design's own symbol artwork re-laid-out (partial-faithful, WS7-031).
const (
	SymbolsGlyph    = "glyph"
	SymbolsFaithful = "faithful"
)

// Format describes what the engine can do with one file extension. A nil reader means the
// capability is absent for that extension: .eds is geometry-only (no netlist), .edn and the
// board/netlist formats carry no faithful schematic geometry.
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
}

// byExt is the registry. The built-in entries are defined in registry.go; an out-of-module
// consumer (the open-core overlay) adds its own via Register.
var byExt = map[string]*Format{}

// Register adds a format to the single registry, keyed by its extension. This is the public
// extension point: a package in another module (a proprietary-format reader in the private
// overlay) calls Register — from its init or the composing binary's main — and its extension
// then resolves through every derived surface (ByExt, the CLI reader dispatch, the file-tree
// label, the supported-extensions error text), because they all read this one table. The
// built-in readers register through this same function, so there is exactly one code path.
//
// Register panics on a malformed entry or a duplicate extension, matching the standard
// library's registry convention (image.RegisterFormat, sql.Register): both are programming
// errors surfaced at process start, not runtime conditions a caller recovers from. An
// extension is claimed first-come; to override a built-in, a consumer composes a binary that
// does not register the built-in rather than re-registering the extension.
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

// HasBoard reports whether the file's extension carries board geometry (copper, layers, courtyards)
// the board-tier rules run against. It is the question a caller asks when a design declares several
// companion views and only one of them can supply the board tier.
func HasBoard(name string) bool {
	f := ByExt(name)
	return f != nil && f.Board != nil
}

// NetlistExts returns the sorted extensions with a netlist reader, for error text and help.
func NetlistExts() []string {
	var exts []string
	for ext, f := range byExt {
		if f.Design != nil {
			exts = append(exts, ext)
		}
	}
	sort.Strings(exts)
	return exts
}

func lowerExt(name string) string {
	return strings.ToLower(filepath.Ext(name))
}

// faithfulUnavailable explains why a file has no faithful schematic geometry and points at
// the auto-layout fallback. The message differs by format so the user knows whether the
// schematic is missing (KiCad, not yet extracted) or does not exist for that format at all
// (netlist/board).
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

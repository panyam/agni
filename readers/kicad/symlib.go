package kicad

import (
	"bytes"
	"strings"
)

// This file is the external symbol-library side of the reader (WS1-016): resolving
// `lib_id "Library:Symbol"` references that the schematic does NOT embed in lib_symbols,
// from `.kicad_sym` files — KiCad's own installed libraries or a vendored copy. v6+
// schematics are self-contained, so the embedded index always wins and this path serves
// stripped/minimal files and fixtures that want authentic artwork without pasting it in.
// The reader owns no file I/O (C1): the caller supplies openSym, built by
// formats from the project's sym-lib-table and the --symbol-path dirs, the same
// opener pattern the xschem/gEDA readers use for `.sym` files.

// ParseSymLibTable parses a KiCad sym-lib-table: the project's nickname -> library-file
// mapping. ${KIPRJMOD} expands to projDir (the table's own directory, per KiCad); other
// ${VAR} references are left verbatim — the caller falls back to searching its symbol
// dirs by nickname, which is how installed-lib names (${KICAD9_SYMBOL_DIR}/Device.kicad_sym)
// resolve without environment knowledge. Pure function on bytes; no file I/O.
func ParseSymLibTable(data []byte, projDir string) map[string]string {
	root, err := parse(bytes.NewReader(data))
	if err != nil || root.Head() != "sym_lib_table" {
		return nil
	}
	out := map[string]string{}
	for _, lib := range root.Children("lib") {
		name := atomOf(lib.Child("name").Arg(1))
		uri := atomOf(lib.Child("uri").Arg(1))
		if name == "" || uri == "" {
			continue
		}
		out[name] = strings.ReplaceAll(uri, "${KIPRJMOD}", projDir)
	}
	return out
}

// symLibCache resolves lib_id references against external libraries, one parse per
// library per read. A library that fails to open or parse is remembered as absent, so a
// missing lib costs one lookup, not one per placement. nil receiver (no opener supplied)
// resolves nothing — every call site degrades to today's placeholder behavior.
type symLibCache struct {
	open func(lib string) ([]byte, error)
	libs map[string]map[string]*node
}

func newSymLibCache(open func(lib string) ([]byte, error)) *symLibCache {
	if open == nil {
		return nil
	}
	return &symLibCache{open: open, libs: map[string]map[string]*node{}}
}

// symbol resolves "Library:Name" to its lib-symbol node, with same-library `extends`
// chains flattened (official libs derive symbols heavily: a child that declares no unit
// sub-symbols of its own inherits the parent's pins and artwork; child properties win).
// Cross-library extends is out of scope (OUT_OF_SCOPE.md). Returns nil when unresolved.
func (c *symLibCache) symbol(libID string) *node {
	if c == nil {
		return nil
	}
	lib, name, ok := strings.Cut(libID, ":")
	if !ok {
		return nil
	}
	syms, seen := c.libs[lib]
	if !seen {
		syms = c.load(lib)
		c.libs[lib] = syms
	}
	sym := syms[name]
	if sym == nil {
		return nil
	}
	return flattenExtends(syms, sym, 0)
}

func (c *symLibCache) load(lib string) map[string]*node {
	data, err := c.open(lib)
	if err != nil {
		return nil
	}
	root, err := parse(bytes.NewReader(data))
	if err != nil || root.Head() != "kicad_symbol_lib" {
		return nil
	}
	syms := map[string]*node{}
	for _, s := range root.Children("symbol") {
		if n := atomOf(s.Arg(1)); n != "" {
			syms[n] = s
		}
	}
	return syms
}

// flattenExtends resolves a derived symbol against its same-library parent chain: when
// the child has no unit sub-symbols, a synthetic node is built carrying the child's own
// entries (properties, the name) plus the parent's sub-symbols and pin display flags.
// Depth-capped against cyclic extends.
func flattenExtends(syms map[string]*node, sym *node, depth int) *node {
	ext := sym.Child("extends")
	if ext == nil || depth > 8 {
		return sym
	}
	parent := syms[atomOf(ext.Arg(1))]
	if parent == nil {
		return sym
	}
	parent = flattenExtends(syms, parent, depth+1)
	if len(sym.Children("symbol")) > 0 {
		return sym // the child defines its own units; they win wholesale
	}
	merged := &node{IsList: true, Kids: append([]*node{}, sym.Kids...)}
	for _, k := range parent.Kids {
		switch {
		case k.Head() == "symbol",
			k.Head() == "pin_numbers" && sym.Child("pin_numbers") == nil,
			k.Head() == "pin_names" && sym.Child("pin_names") == nil:
			merged.Kids = append(merged.Kids, k)
		}
	}
	return merged
}

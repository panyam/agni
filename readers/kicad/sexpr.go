// Package kicad reads KiCad s-expression files (.kicad_pcb, .kicad_sch) into the neutral IR
// (agni.v1.ir). Readers take an io.Reader and record the file name only as provenance, never opening
// files themselves (CONSTRAINTS C1).
//
// Parsing goes through internal/sexpr in its KiCad string dialect (backslash escapes, newlines kept).
package kicad

import (
	"io"

	"github.com/panyam/agni/internal/sexpr"
)

// node is a local alias for sexpr.Node, walked via Head/Arg/Child/Children.
type node = sexpr.Node

// parse reads one top-level s-expression from r in the KiCad string dialect (backslash escapes,
// literal newlines kept).
func parse(r io.Reader) (*node, error) {
	return sexpr.Parse(r, sexpr.KiCadStrings)
}

// boardSkips are the board subtrees no reader of a .kicad_pcb reads. A zone's fill is the copper
// KiCad last computed for it, and the board reads keep only the zone's authored outline, so its fill
// points, about a fifth of a large board's atoms, are left out at parse (agni issue 946). The census
// parses without skipping, and classifies filled_polygon as a known drop.
var boardSkips = []string{"filled_polygon"}

// parseBoard parses a .kicad_pcb, leaving out boardSkips.
func parseBoard(r io.Reader) (*node, error) {
	return sexpr.ParseSkipping(r, sexpr.KiCadStrings, boardSkips...)
}

// atomOf returns the text of an atom node, or "" for a list or nil.
func atomOf(n *node) string { return n.Text() }

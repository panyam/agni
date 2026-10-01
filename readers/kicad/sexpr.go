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

// atomOf returns the text of an atom node, or "" for a list or nil.
func atomOf(n *node) string { return n.Text() }

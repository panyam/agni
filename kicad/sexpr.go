// Package kicad reads KiCad s-expression files (.kicad_pcb, .kicad_sch) into the neutral IR
// (agni.v1.ir). It is core, runtime-agnostic Go (CONSTRAINTS C1): readers take io.Reader and
// record only provenance, never opening files themselves.
//
// S-expression parsing is the shared internal/sexpr package (one parser for KiCad + EDIF,
// parameterized on the string dialect); KiCad uses the escape-and-keep-newlines mode. node is a
// local alias so the reader's many `*node` signatures and the `atomOf` accessor read unchanged.
package kicad

import (
	"io"

	"github.com/panyam/agni/internal/sexpr"
)

// node is the shared s-expression node; the reader walks it via Head/Arg/Child/Children.
type node = sexpr.Node

// parse reads one top-level s-expression from r in the KiCad string dialect (backslash escapes,
// literal newlines kept).
func parse(r io.Reader) (*node, error) {
	return sexpr.Parse(r, sexpr.KiCadStrings)
}

// atomOf returns the text of an atom node, or "" for a list or nil.
func atomOf(n *node) string { return n.Text() }

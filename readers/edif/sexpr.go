// Package edif parses EDIF 2.0.0 netlists into the agni IR.
//
// S-expression parsing is the shared internal/sexpr package (one parser for KiCad + EDIF,
// parameterized on the string dialect); EDIF uses the no-escape, drop-CR/LF mode that rejoins a
// column-wrapped token (WS1-026). node is a local alias so reader.go's many `*node` signatures and
// the `atom` accessor read unchanged.
package edif

import (
	"io"

	"github.com/panyam/agni/internal/sexpr"
)

// node is the shared s-expression node; the reader walks it via Head/Arg/Child/Children.
type node = sexpr.Node

// parse reads a full EDIF document in the EDIF string dialect (no escapes; CR/LF dropped).
func parse(r io.Reader) (*node, error) {
	return sexpr.Parse(r, sexpr.EDIFStrings)
}

// collect appends every list in n's subtree whose Head is head.
func collect(n *node, head string, out *[]*node) {
	sexpr.Collect(n, head, out)
}

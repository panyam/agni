// Package edif parses EDIF 2.0.0 netlists into the agni IR.
//
// S-expression parsing is the shared internal/sexpr package (one parser for KiCad + EDIF,
// parameterized on the string dialect); EDIF uses the no-escape, drop-CR/LF mode that rejoins a
// column-wrapped token (WS1-026). node is a local alias so reader.go's many `*node` signatures and
// the `atom` accessor read unchanged.
package edif

import (
	"io"
	"strings"

	"github.com/panyam/agni/internal/sexpr"
)

// node is the shared s-expression node; the reader walks it via Head/Arg/Child/Children.
type node = sexpr.Node

// parse reads a full EDIF document in the EDIF string dialect (no escapes; CR/LF dropped), with
// every keyword the reader looks up spelled the way it looks it up.
func parse(r io.Reader) (*node, error) {
	root, err := sexpr.Parse(r, sexpr.EDIFStrings)
	if err != nil {
		return nil, err
	}
	canonicalize(root)
	return root, nil
}

// collect appends every list in n's subtree whose Head is head.
func collect(n *node, head string, out *[]*node) {
	sexpr.Collect(n, head, out)
}

// keywords is every EDIF keyword the reader looks a list up by, in the spelling it uses.
// TestEveryLookedUpKeywordIsCanonicalized holds this list to the reader's source.
var keywords = []string{
	"annotate", "arc", "array", "boolean", "boundingBox", "cell", "cellRef", "cellType", "circle",
	"commentGraphics", "connectLocation", "contents", "curve", "design", "designator", "direction",
	"display", "dot", "e", "edifVersion", "false", "figure", "figureGroup", "figureGroupOverride",
	"instance", "instanceRef", "integer", "interface", "justify", "keywordDisplay", "library",
	"libraryRef", "member", "name", "net", "openShape", "orientation", "origin", "page", "pageSize",
	"path", "pointList", "port", "portImplementation", "portInstance", "portRef", "property", "pt",
	"rectangle", "rename", "scale", "scaleX", "scaleY", "string", "stringDisplay", "symbol",
	"textHeight", "transform", "true", "unit", "view", "viewRef", "visible",
}

// canonical maps a keyword's lower-case form to the reader's spelling of it.
var canonical = func() map[string]string {
	m := make(map[string]string, len(keywords))
	for _, k := range keywords {
		m[strings.ToLower(k)] = k
	}
	return m
}()

// canonicalize respells each list's keyword the way the reader looks it up (agni issue 941). EDIF
// keywords are case-insensitive, and Altium's export writes `(Net`, `(PortRef` and `(Instance`, so
// an exact match found nothing in its files and they read as empty designs. Every EDIF list opens
// with a keyword, so only heads change and names and strings keep their case. A keyword the reader
// never looks up keeps its spelling, since nothing asks for it.
func canonicalize(n *node) {
	if n == nil || !n.IsList {
		return
	}
	if len(n.Kids) > 0 {
		if h := n.Kids[0]; !h.IsList && !h.Quoted {
			if k, ok := canonical[strings.ToLower(h.Atom)]; ok {
				h.Atom = k
			}
		}
	}
	for _, k := range n.Kids {
		canonicalize(k)
	}
}

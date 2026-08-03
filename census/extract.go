package census

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strings"

	"github.com/panyam/agni/internal/sexpr"
)

// Kind selects the extractor for a format family. The two s-expr kinds differ only in the shared
// parser's string dialect (KiCad escapes vs EDIF CR/LF drop), which the census needs correct so a
// quoted string terminates where the reader would find it.
type Kind int

const (
	// KindKiCad enumerates s-expression head atoms with the KiCad string dialect (.kicad_pcb/.kicad_sch).
	KindKiCad Kind = iota
	// KindEDIF enumerates s-expression head atoms with the EDIF string dialect (.edn/.eds).
	KindEDIF
	// KindXML enumerates XML element local names (IPC-2581).
	KindXML
	// KindLine enumerates line-oriented object type chars + "@key" attribute keys (xschem, gEDA).
	KindLine
)

// NumberToken is the sentinel for a numeric s-expr head (KiCad layer-table rows keyed by number,
// version triples). Collapsing them to one token keeps a manifest from listing every integer.
const NumberToken = "<number>"

// Enumerate returns the distinct source constructs present in data, per the extractor Kind. Tokens
// are the manifest keys: s-expr heads, XML element names, line-type chars, or "@key".
func Enumerate(data []byte, kind Kind) []string {
	switch kind {
	case KindXML:
		return extractXML(data)
	case KindLine:
		return extractLine(data)
	case KindEDIF:
		return extractSExpr(data, sexpr.EDIFStrings)
	default:
		return extractSExpr(data, sexpr.KiCadStrings)
	}
}

// opaqueSExpr are s-expr subtrees the census records as ONE construct without enumerating their
// children: they are editor/plotter configuration blobs (dozens of one-off knob names) that carry
// no reader-relevant vocabulary, so listing every knob would be noise. The block head itself is
// still classified once.
var opaqueSExpr = map[string]bool{"pcbplotparams": true}

// extractSExpr parses data with the shared s-expr parser (so quoted parens, escapes, and the
// EDIF/KiCad string dialects are handled once, correctly) and walks the tree for head atoms. A
// numeric head collapses to NumberToken; an opaqueSExpr block contributes its head but not its
// children. A malformed file yields nothing — the census is coarse, not a validator.
func extractSExpr(data []byte, mode sexpr.StringMode) []string {
	root, err := sexpr.Parse(bytes.NewReader(data), mode)
	if err != nil || root == nil {
		return nil
	}
	set := map[string]bool{}
	var walk func(n *sexpr.Node)
	walk = func(n *sexpr.Node) {
		if n == nil || !n.IsList {
			return
		}
		if head := n.Head(); head != "" {
			if head[0] == '-' || (head[0] >= '0' && head[0] <= '9') {
				set[NumberToken] = true
			} else {
				set[head] = true
				if opaqueSExpr[head] {
					return // classify the block, skip its config children
				}
			}
		}
		for _, k := range n.Kids {
			walk(k)
		}
	}
	walk(root)
	return keys(set)
}

func isHeadStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// extractXML returns the distinct element local names, ignoring attributes and namespaces.
func extractXML(data []byte) []string {
	set := map[string]bool{}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // partial/malformed: return what we have (census is coarse, not a validator)
		}
		if se, ok := tok.(xml.StartElement); ok {
			set[se.Name.Local] = true
		}
	}
	return keys(set)
}

// attrKeyRe matches an attribute key only at a KEY position — start of string or after
// whitespace / '{' / '(' — capturing group 1. Requiring a boundary before the key means base64
// padding inside an image_data value (a '=' preceded by an alnum char) never registers as a key.
var attrKeyRe = regexp.MustCompile(`(?:^|[\s{(])([A-Za-z_][A-Za-z0-9_-]*)=`)

// extractLine enumerates line-oriented formats (xschem, gEDA). An OBJECT line is a single-letter
// type token followed by a coordinate, sign, or brace (`C 16400 …`, `T {text} …`, `v 2 …`) — the
// coordinate/brace check rejects wrapped property/text continuation lines that merely happen to
// start with "X ". Attribute keys become "@key" tokens (a second namespace, so an object `T` and
// an attribute `@type` never collide).
func extractLine(data []byte) []string {
	set := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		s := strings.TrimLeft(line, " \t")
		if len(s) >= 3 && isHeadStart(s[0]) && s[1] == ' ' && isObjectArgStart(s[2]) {
			set[s[:1]] = true
		}
	}
	// Attribute keys are scanned with quoted regions blanked out, so a domain-parameter key inside
	// a quoted value (xschem SPICE model params in template="Bf=100 Is=1e-14 …") is not mistaken
	// for a schematic attribute — those are simulation model data, not a construct the reader drops.
	for _, m := range attrKeyRe.FindAllStringSubmatch(string(blankQuoted(data)), -1) {
		set["@"+m[1]] = true
	}
	return keys(set)
}

// blankQuoted replaces the contents of double-quoted runs (and the quotes) with spaces, preserving
// byte offsets and line structure so the attr-key regex sees only unquoted text.
func blankQuoted(data []byte) []byte {
	out := make([]byte, len(data))
	inQuote := false
	for i, c := range data {
		switch {
		case c == '"':
			inQuote = !inQuote
			out[i] = ' '
		case inQuote && c != '\n':
			out[i] = ' '
		default:
			out[i] = c
		}
	}
	return out
}

// isObjectArgStart reports whether b can begin an object line's argument list: a coordinate
// (digit/sign/dot) or a braced field. Attribute continuation lines start with a letter, so they
// fail this and are not mistaken for objects.
func isObjectArgStart(b byte) bool {
	return (b >= '0' && b <= '9') || b == '-' || b == '.' || b == '{'
}

func keys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

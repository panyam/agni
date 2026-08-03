// Package sexpr is the shared s-expression parser for the format readers (KiCad, EDIF) and the
// coverage census. It is a streaming tokenizer plus a generic AST, parameterized on the ONE point
// where the KiCad and EDIF dialects diverge — how a quoted string's bytes are resolved — so a
// single implementation serves both without changing either's observed behavior. Readers extract
// their format subset from the generic tree (as edif/reader.go and kicad/*.go do today); the
// census walks it for the construct vocabulary.
//
// It streams via bufio (built for EDIF's multi-MB exports); a top-level Parse returns one node.
package sexpr

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Node is a parsed s-expression: an atom (IsList=false, text in Atom) or a list (IsList=true,
// elements in Kids). Quoted marks an atom that came from a "quoted" string, which the EDIF name
// grammar distinguishes from a bare symbol.
type Node struct {
	IsList bool
	Atom   string
	Quoted bool
	Kids   []*Node
}

// Head returns the leading symbol of a list ("net" for (net ...)), or "" for an atom or a list
// whose first element is itself a list.
func (n *Node) Head() string {
	if n != nil && n.IsList && len(n.Kids) > 0 && !n.Kids[0].IsList {
		return n.Kids[0].Atom
	}
	return ""
}

// Arg returns the i-th element of a list (index 0 is the head), or nil if out of range.
func (n *Node) Arg(i int) *Node {
	if n != nil && n.IsList && i >= 0 && i < len(n.Kids) {
		return n.Kids[i]
	}
	return nil
}

// Child returns the first direct list child whose Head is name, or nil.
func (n *Node) Child(name string) *Node {
	if n == nil {
		return nil
	}
	for _, k := range n.Kids {
		if k.IsList && k.Head() == name {
			return k
		}
	}
	return nil
}

// Children returns all direct list children whose Head is name.
func (n *Node) Children(name string) []*Node {
	var out []*Node
	if n == nil {
		return out
	}
	for _, k := range n.Kids {
		if k.IsList && k.Head() == name {
			out = append(out, k)
		}
	}
	return out
}

// Text returns an atom's text, or "" for a list or nil.
func (n *Node) Text() string {
	if n != nil && !n.IsList {
		return n.Atom
	}
	return ""
}

// Collect appends every list in n's subtree (including n) whose Head is head.
func Collect(n *Node, head string, out *[]*Node) {
	if n == nil || !n.IsList {
		return
	}
	if n.Head() == head {
		*out = append(*out, n)
	}
	for _, k := range n.Kids {
		Collect(k, head, out)
	}
}

// StringMode selects how a quoted string's bytes are resolved — the only point where the KiCad and
// EDIF dialects diverge.
type StringMode int

const (
	// KiCadStrings decodes backslash escapes (\n -> newline, \t -> tab, \<c> -> <c>) and keeps
	// literal newlines. The backslash also escapes a quote, so \" does not terminate the string.
	KiCadStrings StringMode = iota
	// EDIFStrings does NO escape processing and DROPS CR/LF inside a string: machine-generated EDIF
	// is column-wrapped, splitting a token across a newline, and dropping the newline rejoins it
	// losslessly (WS1-026). A backslash is an ordinary byte; any '"' terminates the string.
	EDIFStrings
)

// Parse reads one top-level s-expression from r, resolving quoted strings per mode.
func Parse(r io.Reader, mode StringMode) (*Node, error) {
	t := &tokenizer{r: bufio.NewReaderSize(r, 1<<20), mode: mode}
	tok, err := t.scan()
	if err != nil {
		return nil, err
	}
	if tok.kind != tokLParen {
		return nil, fmt.Errorf("sexpr: expected '(' at start, got %q", tok.text)
	}
	return t.parseList()
}

type tokKind int

const (
	tokLParen tokKind = iota
	tokRParen
	tokAtom
	tokString
	tokEOF
)

type token struct {
	kind tokKind
	text string
}

type tokenizer struct {
	r    *bufio.Reader
	mode StringMode
}

func (t *tokenizer) scan() (token, error) {
	for {
		b, err := t.r.ReadByte()
		if err == io.EOF {
			return token{kind: tokEOF}, nil
		}
		if err != nil {
			return token{}, err
		}
		switch {
		case b == ' ' || b == '\t' || b == '\r' || b == '\n':
			continue
		case b == '(':
			return token{kind: tokLParen, text: "("}, nil
		case b == ')':
			return token{kind: tokRParen, text: ")"}, nil
		case b == '"':
			return t.scanString()
		default:
			return t.scanAtom(b)
		}
	}
}

// scanString reads the body of a "..." string; the opening quote is already consumed. The two
// dialects (see StringMode) differ only here.
func (t *tokenizer) scanString() (token, error) {
	var buf []byte
	for {
		b, err := t.r.ReadByte()
		if err == io.EOF {
			return token{}, fmt.Errorf("sexpr: unterminated string")
		}
		if err != nil {
			return token{}, err
		}
		if t.mode == KiCadStrings && b == '\\' {
			n, err := t.r.ReadByte()
			if err != nil {
				buf = append(buf, '\\')
				if err == io.EOF {
					return token{}, fmt.Errorf("sexpr: unterminated string")
				}
				return token{}, err
			}
			switch n {
			case 'n':
				buf = append(buf, '\n')
			case 't':
				buf = append(buf, '\t')
			default:
				buf = append(buf, n)
			}
			continue
		}
		if b == '"' {
			return token{kind: tokString, text: string(buf)}, nil
		}
		if t.mode == EDIFStrings && b == '%' {
			// EDIF escapes characters not directly representable as %<decimal code(s)>%:
			// %10% -> newline, %72 73% -> "HI". A '%' that does not form a valid escape is
			// kept literally (and its consumed bytes restored), so ordinary text with a
			// stray percent is lossless.
			raw, closed := t.readEDIFPercent()
			if dec, ok := decodeEDIFCodes(raw); ok && closed {
				buf = append(buf, dec...)
			} else {
				buf = append(buf, '%')
				buf = append(buf, raw...)
				if closed {
					buf = append(buf, '%')
				}
			}
			continue
		}
		if t.mode == EDIFStrings && (b == '\n' || b == '\r') {
			continue // rejoin a column-wrapped token
		}
		buf = append(buf, b)
	}
}

// readEDIFPercent reads the body of an EDIF %...% escape after the leading '%'. It returns the
// bytes between the percents and whether a closing '%' was found. CR/LF are dropped (a
// column-wrap may split the escape). A byte that cannot be part of a decimal code list ends the
// read and is unread, so scanString reprocesses it (it may be the closing quote); the caller then
// treats the sequence as a literal '%'.
func (t *tokenizer) readEDIFPercent() (raw []byte, closed bool) {
	for {
		b, err := t.r.ReadByte()
		if err != nil {
			return raw, false // EOF: scanString's next read reports the unterminated string
		}
		switch {
		case b == '%':
			return raw, true
		case b == '\n' || b == '\r':
			// drop a column-wrap that split the escape
		case b >= '0' && b <= '9' || b == ' ' || b == '\t':
			raw = append(raw, b)
		default:
			_ = t.r.UnreadByte()
			return raw, false
		}
	}
}

// decodeEDIFCodes turns the body of a %...% escape (whitespace-separated decimal character codes)
// into the runes it names. It reports false when the body is empty or any token is not a valid
// code, so the caller can keep the text literal.
func decodeEDIFCodes(raw []byte) ([]byte, bool) {
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return nil, false
	}
	var out []byte
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 || n > utf8.MaxRune {
			return nil, false
		}
		out = utf8.AppendRune(out, rune(n))
	}
	return out, true
}

// scanAtom reads a bare atom (symbol, number, &ref) beginning with first, stopping at whitespace,
// a paren, or a quote.
func (t *tokenizer) scanAtom(first byte) (token, error) {
	buf := []byte{first}
	for {
		b, err := t.r.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return token{}, err
		}
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '(' || b == ')' || b == '"' {
			_ = t.r.UnreadByte()
			break
		}
		buf = append(buf, b)
	}
	return token{kind: tokAtom, text: string(buf)}, nil
}

// parseList reads elements until the matching ')'. The opening '(' is already consumed.
func (t *tokenizer) parseList() (*Node, error) {
	n := &Node{IsList: true}
	for {
		tok, err := t.scan()
		if err != nil {
			return nil, err
		}
		switch tok.kind {
		case tokEOF:
			return nil, fmt.Errorf("sexpr: unexpected EOF inside list")
		case tokRParen:
			return n, nil
		case tokLParen:
			child, err := t.parseList()
			if err != nil {
				return nil, err
			}
			n.Kids = append(n.Kids, child)
		case tokAtom:
			n.Kids = append(n.Kids, &Node{Atom: tok.text})
		case tokString:
			n.Kids = append(n.Kids, &Node{Atom: tok.text, Quoted: true})
		}
	}
}

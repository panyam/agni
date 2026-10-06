// Package sexpr is the shared s-expression parser for the format readers (KiCad, EDIF) and the
// coverage census. It is a tokenizer plus a generic AST, parameterized on the ONE point where the
// KiCad and EDIF dialects diverge, how a quoted string's bytes are resolved (StringMode). Readers
// extract their format subset from the generic tree (readers/edif, readers/kicad), and the census
// walks it for the construct vocabulary.
//
// Parse reads the whole input into one string and builds the tree over it compactly, because an
// 80 MB board is about 8M atoms and a heap object per atom held it at 11 times its size (agni issue
// 945). Lists come from slabs and each list's Kids is sized exactly. Atoms are shared, so every atom
// with the same text and Quoted flag is ONE Node, which turns 7.9M atoms into 684K on that board.
// Their texts are copies rather than substrings of the input, so the input is garbage once Parse
// returns, and a reader that keeps a net name does not keep the whole file alive with it.
package sexpr

import (
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Node is a parsed s-expression: an atom (IsList=false, text in Atom) or a list (IsList=true,
// elements in Kids). Quoted marks an atom that came from a "quoted" string, which the EDIF name
// grammar distinguishes from a bare symbol.
//
// A parsed atom is shared by every atom spelled the same way, so rewriting one means putting a new
// Node in its parent's Kids, never assigning its Atom, which would respell the word everywhere in the
// file. Lists are never shared. A parsed list's Kids has no spare capacity, so appending to it copies
// rather than writing over the next list's children. The two flags sit last so a Node is 48 bytes
// rather than 56.
type Node struct {
	Atom   string
	Kids   []*Node
	IsList bool
	Quoted bool
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

// StringMode selects how a quoted string's bytes are resolved, the only point where the KiCad and
// EDIF dialects diverge.
type StringMode int

const (
	// KiCadStrings decodes backslash escapes (\n -> newline, \t -> tab, \<c> -> <c>) and keeps
	// literal newlines. The backslash also escapes a quote, so \" does not terminate the string.
	KiCadStrings StringMode = iota
	// EDIFStrings decodes only %<decimal codes>% escapes and DROPS CR/LF inside a string, because
	// machine-generated EDIF is column-wrapped and dropping the newline rejoins a split token
	// (WS1-026). A backslash is an ordinary byte, and any '"' terminates the string.
	EDIFStrings
)

// Parse reads the top-level s-expression from r, resolving quoted strings per mode.
//
// The whole input must be that one expression. Leftover input is an error, because it means an
// unbalanced ')' closed the tree early and what was built is a truncated PREFIX that reads like a
// small clean board. A KiCad demo with 349 surplus parens read as 2 of its 71 footprints (agni issue
// 562).
func Parse(r io.Reader, mode StringMode) (*Node, error) {
	src, err := readAll(r)
	if err != nil {
		return nil, err
	}
	t := &tokenizer{src: src, mode: mode}
	tok, err := t.scan()
	if err != nil {
		return nil, err
	}
	if tok.kind != tokLParen {
		return nil, fmt.Errorf("sexpr: expected '(' at start, got %q", tok.text)
	}
	n, err := t.parseList()
	if err != nil {
		return nil, err
	}
	switch tok, err := t.scan(); {
	case err != nil:
		return nil, err
	case tok.kind == tokEOF:
		return n, nil
	default:
		return nil, fmt.Errorf("sexpr: %q at line %d is outside the top-level expression, which an "+
			"unbalanced ')' closed early; the rest of the input is unread", tok.text, t.line())
	}
}

// readAll reads r into one string, sized up front when r says how long it is, so an 80 MB board
// costs one 80 MB allocation rather than a doubling series of them. strings.Builder hands its
// buffer over as the string without a copy.
func readAll(r io.Reader) (string, error) {
	var b strings.Builder
	if n := sizeHint(r); n > 0 {
		b.Grow(n)
	}
	if _, err := io.Copy(&b, r); err != nil {
		return "", err
	}
	return b.String(), nil
}

// sizeHint is how many bytes r holds when it can say (a bytes or strings reader, or a file), else 0.
func sizeHint(r io.Reader) int {
	switch v := r.(type) {
	case interface{ Len() int }:
		return v.Len()
	case interface{ Stat() (fs.FileInfo, error) }:
		if fi, err := v.Stat(); err == nil && fi.Mode().IsRegular() {
			return int(fi.Size())
		}
	}
	return 0
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

// Slab sizes, in elements. A slab starts small so a short file allocates little, and doubles to a
// cap so a large one does not hold a huge chunk alive for one retained node.
const (
	firstSlab   = 64
	maxNodeSlab = 1 << 14
	maxKidSlab  = 1 << 16
)

type tokenizer struct {
	src  string
	pos  int // offset of the next unread byte of src
	mode StringMode

	stack []*Node // children gathered so far by every list still open, innermost last
	nodes []Node  // unused tail of the current node slab
	kids  []*Node // unused tail of the current Kids slab
	nSlab int     // size of the next node slab
	kSlab int     // size of the next Kids slab

	atoms  map[string]*Node // the shared bare atoms, by text
	quoted map[string]*Node // the shared quoted atoms, by text
}

// line is the 1-based line of the next unread byte, which is where Parse's own errors point. It is
// counted only when an error needs it.
func (t *tokenizer) line() int {
	return 1 + strings.Count(t.src[:t.pos], "\n")
}

// next reads one byte, reporting false at the end of the input.
func (t *tokenizer) next() (byte, bool) {
	if t.pos >= len(t.src) {
		return 0, false
	}
	b := t.src[t.pos]
	t.pos++
	return b, true
}

func (t *tokenizer) scan() (token, error) {
	for {
		b, ok := t.next()
		if !ok {
			return token{kind: tokEOF}, nil
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
			return t.scanAtom(), nil
		}
	}
}

// scanString reads the body of a "..." string; the opening quote is already consumed. A string
// holding nothing its dialect rewrites is a substring of the input. One that does is decoded by
// decodeString, where the two dialects (see StringMode) differ.
func (t *tokenizer) scanString() (token, error) {
	start := t.pos
	rest := t.src[start:]
	// Every string ends at a '"' in both dialects, so a string with none after it is unterminated
	// however its escapes resolve.
	q := strings.IndexByte(rest, '"')
	if q < 0 {
		t.pos = len(t.src)
		return token{}, fmt.Errorf("sexpr: unterminated string")
	}
	var special int
	if t.mode == KiCadStrings {
		special = strings.IndexByte(rest[:q], '\\')
	} else {
		special = strings.IndexAny(rest[:q], "%\r\n")
	}
	if special < 0 {
		t.pos = start + q + 1
		return token{kind: tokString, text: rest[:q]}, nil
	}
	t.pos = start + special
	return t.decodeString([]byte(rest[:special]))
}

// decodeString finishes a string whose bytes so far are buf, from the first byte its dialect
// rewrites.
func (t *tokenizer) decodeString(buf []byte) (token, error) {
	for {
		b, ok := t.next()
		if !ok {
			return token{}, fmt.Errorf("sexpr: unterminated string")
		}
		if t.mode == KiCadStrings && b == '\\' {
			n, ok := t.next()
			if !ok {
				return token{}, fmt.Errorf("sexpr: unterminated string")
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
			// EDIF escapes characters as %<decimal code(s)>%, e.g. %10% -> newline,
			// %72 73% -> "HI". A '%' that does not form a valid escape is kept literally, with
			// its consumed bytes restored.
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
// read and is left unread, so decodeString reprocesses it (it may be the closing quote); the
// caller then treats the sequence as a literal '%'.
func (t *tokenizer) readEDIFPercent() (raw []byte, closed bool) {
	for {
		b, ok := t.next()
		if !ok {
			return raw, false // end of input: decodeString's next read reports the unterminated string
		}
		switch {
		case b == '%':
			return raw, true
		case b == '\n' || b == '\r':
			// drop a column-wrap that split the escape
		case b >= '0' && b <= '9' || b == ' ' || b == '\t':
			raw = append(raw, b)
		default:
			t.pos--
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

// scanAtom reads a bare atom (symbol, number, &ref) whose first byte is already consumed, stopping
// at whitespace, a paren, or a quote, which is left unread. The atom is a substring of the input.
func (t *tokenizer) scanAtom() token {
	start := t.pos - 1
	for t.pos < len(t.src) {
		switch t.src[t.pos] {
		case ' ', '\t', '\r', '\n', '(', ')', '"':
			return token{kind: tokAtom, text: t.src[start:t.pos]}
		}
		t.pos++
	}
	return token{kind: tokAtom, text: t.src[start:]}
}

// parseList reads elements until the matching ')'. The opening '(' is already consumed. Children
// gather on t.stack and move into an exactly sized Kids once the list closes.
func (t *tokenizer) parseList() (*Node, error) {
	n := t.node()
	n.IsList = true
	base := len(t.stack)
	for {
		tok, err := t.scan()
		if err != nil {
			return nil, err
		}
		switch tok.kind {
		case tokEOF:
			return nil, fmt.Errorf("sexpr: unexpected EOF inside list")
		case tokRParen:
			n.Kids = t.closeList(base)
			return n, nil
		case tokLParen:
			child, err := t.parseList()
			if err != nil {
				return nil, err
			}
			t.stack = append(t.stack, child)
		case tokAtom, tokString:
			t.stack = append(t.stack, t.atom(tok.text, tok.kind == tokString))
		}
	}
}

// node returns a zeroed Node from the current slab, starting a new slab when it runs out.
func (t *tokenizer) node() *Node {
	if len(t.nodes) == 0 {
		t.nSlab = nextSlab(t.nSlab, maxNodeSlab)
		t.nodes = make([]Node, t.nSlab)
	}
	n := &t.nodes[0]
	t.nodes = t.nodes[1:]
	return n
}

// closeList moves the children gathered above base off the stack into a Kids slice with no spare
// capacity, cut from the current Kids slab. A list too long for a slab gets its own slice. An empty
// list keeps a nil Kids.
func (t *tokenizer) closeList(base int) []*Node {
	k := len(t.stack) - base
	if k == 0 {
		return nil
	}
	var kids []*Node
	switch {
	case k <= len(t.kids):
	case k > maxKidSlab/4:
		kids = make([]*Node, k)
	default:
		t.kSlab = nextSlab(t.kSlab, maxKidSlab)
		t.kids = make([]*Node, max(t.kSlab, k))
	}
	if kids == nil {
		kids = t.kids[:k:k]
		t.kids = t.kids[k:]
	}
	copy(kids, t.stack[base:])
	t.stack = t.stack[:base]
	return kids
}

// nextSlab doubles a slab size from firstSlab up to limit.
func nextSlab(cur, limit int) int {
	if cur == 0 {
		return firstSlab
	}
	return min(cur*2, limit)
}

// atom returns the one Node this parse holds for an atom with text and quoted, making it on first
// sight with a copy of text. A board repeats its atoms heavily (684K distinct among 7.9M on an 80 MB
// one, mostly keywords and numbers), so sharing costs a map lookup per atom and saves a Node.
func (t *tokenizer) atom(text string, quoted bool) *Node {
	m := &t.atoms
	if quoted {
		m = &t.quoted
	}
	if n, ok := (*m)[text]; ok {
		return n
	}
	if *m == nil {
		*m = make(map[string]*Node)
	}
	n := t.node()
	n.Atom = strings.Clone(text)
	n.Quoted = quoted
	(*m)[n.Atom] = n
	return n
}

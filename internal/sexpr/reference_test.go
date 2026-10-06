package sexpr

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reference below is the parser as it stood before agni issue 945 compacted the tree: a bufio
// tokenizer, a heap Node per element and Kids grown by append. It stays here as the definition of
// what Parse must build, so the compact parser is held to it over every fixture in the repo rather
// than to a handful of hand-written cases.

func refParse(r io.Reader, mode StringMode) (*Node, error) {
	t := &refTokenizer{r: bufio.NewReaderSize(r, 1<<20), mode: mode, line: 1}
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
			"unbalanced ')' closed early; the rest of the input is unread", tok.text, t.line)
	}
}

type refTokenizer struct {
	r    *bufio.Reader
	mode StringMode
	line int
}

func (t *refTokenizer) readByte() (byte, error) {
	b, err := t.r.ReadByte()
	if err == nil && b == '\n' {
		t.line++
	}
	return b, err
}

func (t *refTokenizer) unread(b byte) {
	if t.r.UnreadByte() == nil && b == '\n' {
		t.line--
	}
}

func (t *refTokenizer) scan() (token, error) {
	for {
		b, err := t.readByte()
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

func (t *refTokenizer) scanString() (token, error) {
	var buf []byte
	for {
		b, err := t.readByte()
		if err == io.EOF {
			return token{}, fmt.Errorf("sexpr: unterminated string")
		}
		if err != nil {
			return token{}, err
		}
		if t.mode == KiCadStrings && b == '\\' {
			n, err := t.readByte()
			if err != nil {
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
			continue
		}
		buf = append(buf, b)
	}
}

func (t *refTokenizer) readEDIFPercent() (raw []byte, closed bool) {
	for {
		b, err := t.readByte()
		if err != nil {
			return raw, false
		}
		switch {
		case b == '%':
			return raw, true
		case b == '\n' || b == '\r':
		case b >= '0' && b <= '9' || b == ' ' || b == '\t':
			raw = append(raw, b)
		default:
			t.unread(b)
			return raw, false
		}
	}
}

func (t *refTokenizer) scanAtom(first byte) (token, error) {
	buf := []byte{first}
	for {
		b, err := t.readByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return token{}, err
		}
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '(' || b == ')' || b == '"' {
			t.unread(b)
			break
		}
		buf = append(buf, b)
	}
	return token{kind: tokAtom, text: string(buf)}, nil
}

func (t *refTokenizer) parseList() (*Node, error) {
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

// sameTree reports the first place got differs from want, or "" when they match. It also requires
// every Kids in got to have no spare capacity, which is what keeps an append from writing into a
// neighbour's children.
func sameTree(got, want *Node, path string) string {
	switch {
	case got.IsList != want.IsList:
		return fmt.Sprintf("%s: IsList %v, want %v", path, got.IsList, want.IsList)
	case got.Atom != want.Atom:
		return fmt.Sprintf("%s: Atom %q, want %q", path, got.Atom, want.Atom)
	case got.Quoted != want.Quoted:
		return fmt.Sprintf("%s: Quoted %v, want %v", path, got.Quoted, want.Quoted)
	case len(got.Kids) != len(want.Kids):
		return fmt.Sprintf("%s: %d kids, want %d", path, len(got.Kids), len(want.Kids))
	case (got.Kids == nil) != (want.Kids == nil):
		return fmt.Sprintf("%s: Kids nil is %v, want %v", path, got.Kids == nil, want.Kids == nil)
	case cap(got.Kids) != len(got.Kids):
		return fmt.Sprintf("%s: Kids has cap %d for %d kids", path, cap(got.Kids), len(got.Kids))
	}
	for i := range got.Kids {
		if d := sameTree(got.Kids[i], want.Kids[i], fmt.Sprintf("%s/%d%s", path, i, headOf(want.Kids[i]))); d != "" {
			return d
		}
	}
	return ""
}

func headOf(n *Node) string {
	if h := n.Head(); h != "" {
		return "(" + h + ")"
	}
	return ""
}

// sameParse parses src with both parsers and reports how they disagree, comparing an error by its
// message, since a test elsewhere may check one.
func sameParse(src []byte, mode StringMode) string {
	got, gotErr := Parse(bytes.NewReader(src), mode)
	want, wantErr := refParse(bytes.NewReader(src), mode)
	switch {
	case (gotErr == nil) != (wantErr == nil):
		return fmt.Sprintf("error %v, want %v", gotErr, wantErr)
	case gotErr != nil:
		if gotErr.Error() != wantErr.Error() {
			return fmt.Sprintf("error %q, want %q", gotErr, wantErr)
		}
		return ""
	}
	return sameTree(got, want, "")
}

// sexprExts maps every extension the KiCad and EDIF readers parse to its dialect.
var sexprExts = map[string]StringMode{
	".kicad_pcb": KiCadStrings,
	".kicad_sch": KiCadStrings,
	".kicad_sym": KiCadStrings,
	".kicad_mod": KiCadStrings,
	".edn":       EDIFStrings,
	".eds":       EDIFStrings,
	".edf":       EDIFStrings,
	".edif":      EDIFStrings,
	".edo":       EDIFStrings,
}

// maxFixture bounds the files the fixture sweep reads, so the gate does not parse an 80 MB board
// twice per dialect. AGNI_SEXPR_ALL=1 lifts it.
const maxFixture = 16 << 20

// TestParseMatchesReferenceOnEveryFixture parses every KiCad and EDIF file in the repo, and in the
// fetched samples when they are present, with both parsers in both dialects, and requires the same
// tree or the same error. Parsing in the other dialect too is cheap coverage of the string paths a
// file's own dialect never takes.
func TestParseMatchesReferenceOnEveryFixture(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	all := os.Getenv("AGNI_SEXPR_ALL") == "1"
	counts := map[StringMode]int{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "site" || name == "bin") {
				return filepath.SkipDir
			}
			return nil
		}
		own, ok := sexprExts[strings.ToLower(filepath.Ext(path))]
		if !ok || strings.HasPrefix(d.Name(), "._") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxFixture && !all {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for _, mode := range []StringMode{KiCadStrings, EDIFStrings} {
			if diff := sameParse(src, mode); diff != "" {
				t.Errorf("%s (mode %d): %s", rel, mode, diff)
			}
		}
		counts[own]++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// The repo itself carries about 90 KiCad and 78 EDIF fixtures, so a sweep finding far fewer
	// walked the wrong tree.
	if counts[KiCadStrings] < 50 || counts[EDIFStrings] < 40 {
		t.Fatalf("compared %d KiCad and %d EDIF files; want at least 50 and 40", counts[KiCadStrings], counts[EDIFStrings])
	}
	t.Logf("compared %d KiCad and %d EDIF files", counts[KiCadStrings], counts[EDIFStrings])
}

// edgeInputs are the shapes where a substring fast path and the byte-at-a-time decoder could
// disagree: escapes at the ends of a string, an escape split by a column wrap, input ending
// mid-token, and every way a file can be malformed.
var edgeInputs = []string{
	``, ` `, `x`, `)`, `(`, `()`, `(())`, `(a)`, `(a`, `(a b`, `("a`, `(a "b`, `(a "b\`, `(a "b\"`,
	`(a "")`, `(a "\\")`, `(a "\"")`, `(a "x\n\ty")`, `(a "\q")`, `(a "\`, "(a \"line1\nline2\")",
	"(a \"SCH\r\nEMATIC\")", `(a "50%")`, `(a "%")`, `(a "%%")`, `(a "%72 73%")`, `(a "%7` + "\n" + `2%")`,
	`(a "%abc%")`, `(a "%9")`, `(a "100%")`, `(a "% %")`, `(a "%1114112%")`, `(a "%-1%")`,
	`(a b)(c d)`, "(a)\n\n", "(a)\n)", "(a\n(b\n\"c\nd\")\n)\n)\n", `(a"b"c)`, `(a(b)c)`,
	`(kicad_pcb (version 20240108) (net 0 "") (net 1 "GND") (footprint "R" (at 1 2 90)))`,
	"(edif x (status (written (timeStamp 2024 1 1 0 0 0))) (Net N1 (Joined (PortRef A))))",
}

func TestParseMatchesReferenceOnEdgeInputs(t *testing.T) {
	for _, src := range edgeInputs {
		for _, mode := range []StringMode{KiCadStrings, EDIFStrings} {
			if diff := sameParse([]byte(src), mode); diff != "" {
				t.Errorf("%q (mode %d): %s", src, mode, diff)
			}
		}
	}
}

// FuzzParseMatchesReference searches for an input the two parsers read differently.
func FuzzParseMatchesReference(f *testing.F) {
	for _, src := range edgeInputs {
		f.Add([]byte(src), false)
		f.Add([]byte(src), true)
	}
	f.Fuzz(func(t *testing.T, src []byte, edif bool) {
		mode := KiCadStrings
		if edif {
			mode = EDIFStrings
		}
		if diff := sameParse(src, mode); diff != "" {
			t.Errorf("%q (mode %d): %s", src, mode, diff)
		}
	})
}

// TestParsedAtomsAreSharedAndListsAreNot pins the aliasing Parse promises. Atoms with the same text
// and Quoted flag are one Node, which is where the memory goes back, while a bare atom and a quoted
// one of the same text stay apart, since the EDIF name grammar tells them apart. Lists are never
// shared, and appending to one list's Kids leaves its neighbour's children alone.
func TestParsedAtomsAreSharedAndListsAreNot(t *testing.T) {
	n := parse(t, `(top (a 1) (b 2) (a 1) (a "1"))`, KiCadStrings)
	first, second, third, fourth := n.Arg(1), n.Arg(2), n.Arg(3), n.Arg(4)
	if first.Arg(0) != third.Arg(0) || first.Arg(1) != third.Arg(1) {
		t.Errorf("the two (a 1) lists hold different atom nodes; want them shared")
	}
	if first.Arg(1) == fourth.Arg(1) {
		t.Errorf("bare 1 and quoted \"1\" are one node; want them apart")
	}
	if first == third {
		t.Errorf("two equal lists are one node; lists must never be shared")
	}
	first.Kids = append(first.Kids, &Node{Atom: "extra"})
	if got := second.Arg(0).Text(); got != "b" || len(second.Kids) != 2 {
		t.Errorf("appending to (a 1) changed its neighbour to %q with %d kids", got, len(second.Kids))
	}
}

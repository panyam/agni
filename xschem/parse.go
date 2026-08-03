package xschem

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// An xschem .sch/.sym file is a flat stream of objects, one per logical line. The first
// character of the line is the object type; the rest are its fields. Fields are either
// bare whitespace-delimited words (coordinates, layer numbers, rotation/flip flags) or
// brace-delimited attribute blocks ("{...}"). A brace block may contain newlines, so an
// object can span several physical lines; everything else is single-line.
//
// Object types we care about:
//
//	v {xschem version=...}          file header
//	G {} K {} V {} S {} E {}        global property blocks
//	N x1 y1 x2 y2 {lab=NetName}     a wire segment (net); lab= is the net name it carries
//	C {symref} x y rot flip {props} a component/symbol instance
//	T {text} x y rot flip w h {props} free text
//	L/B/A/P layer ... {props}       drawing primitives (lines/boxes/arcs/polys); in a .sym
//	                                a B box with {name=.. pinnumber=..} is a pin.
//
// The parser is deliberately lossless at the object level: it keeps every field verbatim so
// both the structural extractor and the geometric netlister can read what they need without
// re-scanning. Interpretation (which object is a component, which brace is a symref) lives in
// read.go, not here.

// token is one field of an object: either a brace block (its inner text, braces stripped) or
// a bare word.
type token struct {
	brace bool
	text  string
}

// object is one parsed line: its type char and its ordered tokens.
type object struct {
	typ    byte
	tokens []token
}

// braceAt returns the inner text of the i-th brace-block token, or "" if the i-th token is
// not a brace block or is absent.
func (o object) braceAt(i int) string {
	if i < 0 || i >= len(o.tokens) || !o.tokens[i].brace {
		return ""
	}
	return o.tokens[i].text
}

// word returns the i-th token's text if it is a bare word, else "".
func (o object) word(i int) string {
	if i < 0 || i >= len(o.tokens) || o.tokens[i].brace {
		return ""
	}
	return o.tokens[i].text
}

// parse reads an xschem object stream. It splits the input into logical lines (newlines
// inside a brace block do not end a line), then tokenizes each line into its type char and
// fields. Blank lines and pure-comment lines (a bare "*", xschem's license banner) are
// skipped.
func parse(r io.Reader) ([]object, error) {
	lines, err := logicalLines(r)
	if err != nil {
		return nil, err
	}
	var objs []object
	for _, ln := range lines {
		ln = strings.TrimLeft(ln, " \t")
		if ln == "" || ln[0] == '*' {
			continue // blank or banner comment
		}
		typ := ln[0]
		toks := tokenize(ln[1:])
		objs = append(objs, object{typ: typ, tokens: toks})
	}
	return objs, nil
}

// logicalLines splits input on newlines that sit at brace depth zero. A newline inside a
// "{...}" block is kept, so a component whose {props} span several physical lines stays one
// logical line. Backslash escapes and double-quoted strings are honoured so an escaped or
// quoted brace does not shift the depth.
func logicalLines(r io.Reader) ([]string, error) {
	br := bufio.NewReader(r)
	var lines []string
	var cur strings.Builder
	depth := 0
	inQuote := false
	esc := false
	for {
		c, _, err := br.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch {
		case esc:
			esc = false
		case c == '\\':
			esc = true
		case inQuote:
			if c == '"' {
				inQuote = false
			}
		case c == '"':
			inQuote = true
		case c == '{':
			depth++
		case c == '}':
			if depth > 0 {
				depth--
			}
		case c == '\n' && depth == 0:
			lines = append(lines, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(c)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines, nil
}

// tokenize splits one logical line's fields into brace blocks and bare words. A "{...}"
// becomes a single brace token holding the inner text; runs of non-space become word tokens.
// Quotes are not stripped here (a bare word may legitimately contain a quoted value that the
// prop parser handles later).
func tokenize(s string) []token {
	var toks []token
	i, n := 0, len(s)
	for i < n {
		for i < n && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n') {
			i++
		}
		if i >= n {
			break
		}
		if s[i] == '{' {
			inner, next := readBrace(s, i)
			toks = append(toks, token{brace: true, text: inner})
			i = next
			continue
		}
		start := i
		for i < n && s[i] != ' ' && s[i] != '\t' && s[i] != '\r' && s[i] != '\n' {
			i++
		}
		toks = append(toks, token{text: s[start:i]})
	}
	return toks
}

// readBrace consumes a "{...}" block starting at s[open]=='{' and returns its inner text
// (braces stripped) and the index just past the closing brace. Nested braces are balanced;
// backslash escapes and double-quoted strings are respected so a "\}" or a '}' inside a
// quoted value does not close the block early. An unterminated block runs to end of string.
func readBrace(s string, open int) (string, int) {
	depth := 0
	inQuote := false
	esc := false
	var b strings.Builder
	for i := open; i < len(s); i++ {
		c := s[i]
		switch {
		case esc:
			esc = false
			b.WriteByte(c)
			continue
		case c == '\\':
			esc = true
			b.WriteByte(c)
			continue
		case inQuote:
			if c == '"' {
				inQuote = false
			}
		case c == '"':
			inQuote = true
		case c == '{':
			depth++
			if depth == 1 {
				continue // drop the outermost opening brace
			}
		case c == '}':
			depth--
			if depth == 0 {
				return b.String(), i + 1 // drop the outermost closing brace
			}
		}
		b.WriteByte(c)
	}
	return b.String(), len(s)
}

// props parses an xschem attribute block ("name=R5 value=10 device=\"ceramic capacitor\"")
// into key/value pairs. Values may be double-quoted (quotes stripped) or bare (up to the
// next whitespace). A bare flag with no '=' is recorded with an empty value. Order is not
// preserved; xschem attributes are addressed by name.
func props(block string) map[string]string {
	m := map[string]string{}
	i, n := 0, len(block)
	for i < n {
		for i < n && isSpace(block[i]) {
			i++
		}
		if i >= n {
			break
		}
		ks := i
		for i < n && block[i] != '=' && !isSpace(block[i]) {
			i++
		}
		key := block[ks:i]
		if key == "" {
			i++
			continue
		}
		if i >= n || block[i] != '=' {
			m[key] = "" // bare flag
			continue
		}
		i++ // skip '='
		var val string
		if i < n && block[i] == '"' {
			i++
			vs := i
			for i < n && block[i] != '"' {
				i++
			}
			val = block[vs:i]
			if i < n {
				i++ // skip closing quote
			}
		} else {
			vs := i
			for i < n && !isSpace(block[i]) {
				i++
			}
			val = block[vs:i]
		}
		m[key] = val
	}
	return m
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }

// atoi parses an xschem coordinate. Coordinates are integers in the samples but the format
// permits floats, so parse as float and round. A bad value yields (0,false); unlike the
// fmt.Sscanf it replaces, trailing garbage is rejected rather than silently truncated.
func atoi(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

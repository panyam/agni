// Package wrl reads a VRML97 (.wrl) model's triangles and colours (agni issue 1652).
//
// Say a KiCad footprint names `${KICAD9_3DMODEL_DIR}/Capacitor_SMD.3dshapes/C_0402_1005Metric.wrl`.
// KiCad's own 3D viewer drew that file, so the board's maker saw that mesh. KiCad's library
// writes each model as a flat list of Shapes, each an IndexedFaceSet with a Material, and shares
// materials by DEF and USE. Read returns one part per Shape, with its faces split into triangles.
//
// Fidelity: lossy-bounded. Read keeps each IndexedFaceSet's positions and faces, each Material's
// diffuseColor and transparency, and the transforms of Transform and Group nodes above it. It drops
// normals, texture coordinates, per-vertex colour, lights, viewpoints, and every other geometry
// node, such as Box or IndexedLineSet, which KiCad's library does not write. A node it does not know
// is skipped whole. Positions are in the file's own units. KiCad's library is in tenths of an inch,
// and the caller scales by 2.54 for millimetres.
package wrl

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/panyam/agni/core/mesh"
)

// ErrSyntax reports a file Read cannot parse, such as an unclosed brace or a USE of a name no DEF
// gave.
var ErrSyntax = errors.New("wrl: syntax")

// Read parses a VRML97 file into a mesh in the file's units.
func Read(data []byte) (*mesh.Mesh, error) {
	p := &parser{lex: lexer{src: data}, defs: map[string]*node{}}
	var roots []*node
	for {
		t := p.lex.peek()
		if t.kind == tokEOF {
			break
		}
		n, err := p.node()
		if err != nil {
			return nil, err
		}
		roots = append(roots, n)
	}
	m := &mesh.Mesh{}
	for _, n := range roots {
		collect(n, identity, m, 0)
	}
	return m, nil
}

// node is a parsed VRML node: its type and its fields, each a list of values. A value is a number,
// a word, or a child node.
type node struct {
	typ    string
	fields map[string][]value
}

type value struct {
	num  float64
	word string
	node *node
	isN  bool
}

func (n *node) nums(field string) []float64 {
	var out []float64
	for _, v := range n.fields[field] {
		if v.isN {
			out = append(out, v.num)
		}
	}
	return out
}

func (n *node) child(field string) *node {
	for _, v := range n.fields[field] {
		if v.node != nil {
			return v.node
		}
	}
	return nil
}

func (n *node) children(field string) []*node {
	var out []*node
	for _, v := range n.fields[field] {
		if v.node != nil {
			out = append(out, v.node)
		}
	}
	return out
}

// affine is a row-major 3x4 transform.
type affine [12]float64

var identity = affine{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0}

func (a affine) mul(b affine) affine {
	var o affine
	for r := 0; r < 3; r++ {
		for c := 0; c < 4; c++ {
			v := a[r*4]*b[c] + a[r*4+1]*b[4+c] + a[r*4+2]*b[8+c]
			if c == 3 {
				v += a[r*4+3]
			}
			o[r*4+c] = v
		}
	}
	return o
}

func (a affine) apply(p [3]float64) [3]float64 {
	return [3]float64{
		a[0]*p[0] + a[1]*p[1] + a[2]*p[2] + a[3],
		a[4]*p[0] + a[5]*p[1] + a[6]*p[2] + a[7],
		a[8]*p[0] + a[9]*p[1] + a[10]*p[2] + a[11],
	}
}

// rotation is the axis-angle rotation VRML's rotation field gives.
func rotation(x, y, z, angle float64) affine {
	l := math.Sqrt(x*x + y*y + z*z)
	if l == 0 {
		return identity
	}
	x, y, z = x/l, y/l, z/l
	c, s := math.Cos(angle), math.Sin(angle)
	t := 1 - c
	return affine{
		t*x*x + c, t*x*y - s*z, t*x*z + s*y, 0,
		t*x*y + s*z, t*y*y + c, t*y*z - s*x, 0,
		t*x*z - s*y, t*y*z + s*x, t*z*z + c, 0,
	}
}

func translation(x, y, z float64) affine { return affine{1, 0, 0, x, 0, 1, 0, y, 0, 0, 1, z} }
func scaling(x, y, z float64) affine     { return affine{x, 0, 0, 0, 0, y, 0, 0, 0, 0, z, 0} }

// transformOf composes a Transform node's fields as VRML97 says: T × C × R × SR × S × -SR × -C.
func transformOf(n *node) affine {
	vec := func(f string, def [3]float64) [3]float64 {
		if v := n.nums(f); len(v) >= 3 {
			return [3]float64{v[0], v[1], v[2]}
		}
		return def
	}
	rot := func(f string) affine {
		if v := n.nums(f); len(v) >= 4 {
			return rotation(v[0], v[1], v[2], v[3])
		}
		return identity
	}
	t, c, s := vec("translation", [3]float64{}), vec("center", [3]float64{}), vec("scale", [3]float64{1, 1, 1})
	r, sr := rot("rotation"), rot("scaleOrientation")
	srInv := sr
	srInv[1], srInv[4] = sr[4], sr[1]
	srInv[2], srInv[8] = sr[8], sr[2]
	srInv[6], srInv[9] = sr[9], sr[6]
	return translation(t[0], t[1], t[2]).mul(translation(c[0], c[1], c[2])).mul(r).mul(sr).
		mul(scaling(s[0], s[1], s[2])).mul(srInv).mul(translation(-c[0], -c[1], -c[2]))
}

// collect appends n's triangles to m. depth stops a USE cycle.
func collect(n *node, at affine, m *mesh.Mesh, depth int) {
	if n == nil || depth > 64 {
		return
	}
	switch n.typ {
	case "Transform":
		at = at.mul(transformOf(n))
		fallthrough
	case "Group", "Anchor", "Collision", "Billboard":
		for _, c := range n.children("children") {
			collect(c, at, m, depth+1)
		}
	case "Shape":
		if p, ok := shapePart(n, at); ok {
			m.Parts = append(m.Parts, p)
		}
	}
}

// shapePart reads a Shape's IndexedFaceSet, splitting each face into a fan of triangles.
func shapePart(n *node, at affine) (mesh.Part, bool) {
	g := n.child("geometry")
	if g == nil || g.typ != "IndexedFaceSet" {
		return mesh.Part{}, false
	}
	coord := g.child("coord")
	if coord == nil {
		return mesh.Part{}, false
	}
	xyz := coord.nums("point")
	p := mesh.Part{Color: [4]float64{0.8, 0.8, 0.8, 1}}
	for i := 0; i+2 < len(xyz); i += 3 {
		p.Positions = append(p.Positions, at.apply([3]float64{xyz[i], xyz[i+1], xyz[i+2]}))
	}
	var face []uint32
	flush := func() {
		for k := 1; k+1 < len(face); k++ {
			p.Indices = append(p.Indices, face[0], face[k], face[k+1])
		}
		face = face[:0]
	}
	for _, f := range g.nums("coordIndex") {
		i := int(f)
		if i < 0 {
			flush()
			continue
		}
		if i >= len(p.Positions) {
			// A face naming a point the set does not have is dropped, as KiCad drops it.
			face = face[:0]
			continue
		}
		face = append(face, uint32(i)) // #nosec G115 -- 0 <= i < len(p.Positions)
	}
	flush()
	if len(p.Indices) == 0 {
		return mesh.Part{}, false
	}
	if a := n.child("appearance"); a != nil {
		if mat := a.child("material"); mat != nil {
			if c := mat.nums("diffuseColor"); len(c) >= 3 {
				p.Color[0], p.Color[1], p.Color[2] = c[0], c[1], c[2]
			}
			if t := mat.nums("transparency"); len(t) >= 1 {
				p.Color[3] = 1 - t[0]
			}
		}
	}
	return p, true
}

type parser struct {
	lex  lexer
	defs map[string]*node
}

// node reads `[DEF name] Type { fields }` or `USE name`.
func (p *parser) node() (*node, error) {
	t := p.lex.next()
	if t.kind != tokWord {
		return nil, p.errorf(t, "want a node, got %q", t.text)
	}
	switch t.text {
	case "USE":
		name := p.lex.next()
		n, ok := p.defs[name.text]
		if !ok {
			return nil, p.errorf(name, "USE %s names no DEF", name.text)
		}
		return n, nil
	case "DEF":
		name := p.lex.next()
		n, err := p.node()
		if err != nil {
			return nil, err
		}
		p.defs[name.text] = n
		return n, nil
	case "PROTO", "EXTERNPROTO", "ROUTE":
		return nil, p.errorf(t, "%s is not supported", t.text)
	}
	n := &node{typ: t.text, fields: map[string][]value{}}
	if o := p.lex.next(); o.kind != tokOpen {
		return nil, p.errorf(o, "want { after %s", t.text)
	}
	for {
		f := p.lex.next()
		switch f.kind {
		case tokClose:
			return n, nil
		case tokWord:
		default:
			return nil, p.errorf(f, "want a field name in %s, got %q", n.typ, f.text)
		}
		vals, err := p.values()
		if err != nil {
			return nil, err
		}
		n.fields[f.text] = vals
	}
}

// values reads one field's value: a bracketed list, a node, or a run of numbers and words up to the
// next field name. A field name is a word that a non-number follows, so `diffuseColor 1 0 0` ends
// at the next word.
func (p *parser) values() ([]value, error) {
	t := p.lex.peek()
	switch {
	case t.kind == tokOpenList:
		p.lex.next()
		var out []value
		for {
			t := p.lex.peek()
			switch t.kind {
			case tokCloseList:
				p.lex.next()
				return out, nil
			case tokEOF:
				return nil, p.errorf(t, "unclosed [")
			case tokNum:
				p.lex.next()
				out = append(out, value{num: t.num, isN: true})
			case tokWord:
				if isScalarWord(t.text) {
					p.lex.next()
					out = append(out, value{word: t.text})
					continue
				}
				n, err := p.node()
				if err != nil {
					return nil, err
				}
				out = append(out, value{node: n})
			default:
				return nil, p.errorf(t, "unexpected %q in a list", t.text)
			}
		}
	case t.kind == tokWord && !isScalarWord(t.text):
		n, err := p.node()
		if err != nil {
			return nil, err
		}
		return []value{{node: n}}, nil
	}
	var out []value
	for {
		t := p.lex.peek()
		switch {
		case t.kind == tokNum:
			p.lex.next()
			out = append(out, value{num: t.num, isN: true})
		case t.kind == tokString || (t.kind == tokWord && isScalarWord(t.text)):
			p.lex.next()
			out = append(out, value{word: t.text})
			return out, nil
		default:
			if len(out) == 0 {
				return nil, p.errorf(t, "want a value, got %q", t.text)
			}
			return out, nil
		}
	}
}

func isScalarWord(s string) bool { return s == "TRUE" || s == "FALSE" || s == "NULL" }

func (p *parser) errorf(t token, format string, args ...any) error {
	return fmt.Errorf("%w: line %d: %s", ErrSyntax, t.line, fmt.Sprintf(format, args...))
}

type tokKind int

const (
	tokEOF tokKind = iota
	tokWord
	tokNum
	tokString
	tokOpen
	tokClose
	tokOpenList
	tokCloseList
)

type token struct {
	kind tokKind
	text string
	num  float64
	line int
}

type lexer struct {
	src    []byte
	pos    int
	line   int
	peeked *token
}

func (l *lexer) peek() token {
	if l.peeked == nil {
		t := l.scan()
		l.peeked = &t
	}
	return *l.peeked
}

func (l *lexer) next() token {
	t := l.peek()
	l.peeked = nil
	return t
}

// scan reads one token. Commas are white space in VRML, and # starts a comment to the line's end.
func (l *lexer) scan() token {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			l.line++
			l.pos++
		case c == ' ' || c == '\t' || c == '\r' || c == ',':
			l.pos++
		case c == '#':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		default:
			goto token
		}
	}
	return token{kind: tokEOF, line: l.line + 1}
token:
	line := l.line + 1
	c := l.src[l.pos]
	switch c {
	case '{':
		l.pos++
		return token{kind: tokOpen, text: "{", line: line}
	case '}':
		l.pos++
		return token{kind: tokClose, text: "}", line: line}
	case '[':
		l.pos++
		return token{kind: tokOpenList, text: "[", line: line}
	case ']':
		l.pos++
		return token{kind: tokCloseList, text: "]", line: line}
	case '"':
		start := l.pos + 1
		l.pos++
		for l.pos < len(l.src) && l.src[l.pos] != '"' {
			if l.src[l.pos] == '\\' {
				l.pos++
			}
			l.pos++
		}
		s := string(l.src[start:min(l.pos, len(l.src))])
		l.pos++
		return token{kind: tokString, text: s, line: line}
	}
	start := l.pos
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',' || c == '{' || c == '}' || c == '[' || c == ']' || c == '#' || c == '"' {
			break
		}
		l.pos++
	}
	text := string(l.src[start:l.pos])
	if c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9') {
		if f, err := strconv.ParseFloat(text, 64); err == nil {
			return token{kind: tokNum, text: text, num: f, line: line}
		}
	}
	return token{kind: tokWord, text: text, line: line}
}

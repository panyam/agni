package orcad

import "encoding/binary"

// primKind is a symbol primitive's type byte.
type primKind byte

const (
	primRect     primKind = 40
	primLine     primKind = 41
	primArc      primKind = 42
	primEllipse  primKind = 43
	primPolygon  primKind = 44
	primPolyline primKind = 45
	primText     primKind = 46
	primBezier   primKind = 87
)

// prim is one drawn primitive of a symbol, in the symbol's own frame (Capture's units, Y down).
//
//   - rect and ellipse: pts are the box's two corners.
//   - line: pts are its two ends.
//   - arc: pts are the ellipse's box corners, then the start and the end, drawn anticlockwise on
//     the page from start to end.
//   - polygon, polyline and bezier: pts are the points in order.
//   - text: pts[0] is the text's origin, and text and font are set.
type prim struct {
	kind   primKind
	pts    []point
	filled bool
	text   string
	font   int
}

// drawing is a symbol's primitives and its box. Primitives of a kind the reader does not decode
// are left out, and unknown counts them, so a symbol that drew nothing it knows is visible as one.
type drawing struct {
	prims   []prim
	box     [2]point // the box's corners in the symbol's frame
	unknown int
}

// width and height are the box's, which a placement turns the symbol inside.
func (d drawing) width() int32  { return d.box[1].x - d.box[0].x }
func (d drawing) height() int32 { return d.box[1].y - d.box[0].y }

// readDrawing reads a symbol's primitives from the start of its body, and its box from the eight
// bytes before pinStop, where its pin list starts. A primitive record is two equal type bytes and a
// u32 length that counts itself, a four-byte pad and the body, so the record is two bytes longer
// than its length. Comment text keeps eight bytes of padding past that. Between two primitives
// there can be an eight-byte zero filler or a preamble (FF E4 5C 39, a u32 length, that many
// bytes), and both are skipped. A record that does not frame ends the walk, with what was read
// kept.
func readDrawing(b []byte, f frame, pinStop int) drawing {
	var d drawing
	end := f.end()
	if pinStop >= f.body+8 && pinStop <= end {
		bx := &cursor{b: b[:pinStop], p: pinStop - 8}
		d.box = [2]point{{int32(bx.i16()), int32(bx.i16())}, {int32(bx.i16()), int32(bx.i16())}}
		end = pinStop - 8
	}
	c := &cursor{b: b[:end], p: f.body}
	c.lzt() // name
	c.lzt() // source library
	c.u32() // colour
	n := int(c.u16())
	for i := 0; i < n && c.err == nil; i++ {
		for c.err == nil {
			if c.p+8 <= len(c.b) && allZero(c.b[c.p:c.p+8]) {
				c.p += 8
				continue
			}
			if c.p+8 <= len(c.b) && string(c.b[c.p:c.p+4]) == string(preamble) {
				c.p += 8 + int(binary.LittleEndian.Uint32(c.b[c.p+4:]))
				continue
			}
			break
		}
		if !c.need(10) {
			break
		}
		start := c.p
		kind := primKind(c.b[c.p])
		if c.b[c.p+1] != byte(kind) {
			break
		}
		length := int(binary.LittleEndian.Uint32(c.b[c.p+2:]))
		next := start + 2 + length
		if kind == primText {
			next += 8
		}
		if length < 8 || next > len(c.b) {
			break
		}
		body := &cursor{b: c.b[:next], p: start + 10}
		if p, ok := readPrim(body, kind); ok && body.err == nil {
			d.prims = append(d.prims, p)
		} else {
			d.unknown++
		}
		c.p = next
	}
	return d
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

func readPrim(c *cursor, kind primKind) (prim, bool) {
	p := prim{kind: kind}
	box := func() {
		p.pts = append(p.pts, point{c.i32(), c.i32()}, point{c.i32(), c.i32()})
	}
	switch kind {
	case primLine:
		box()
	case primRect, primEllipse:
		box()
		c.u32() // line style
		c.u32() // line width
		p.filled = c.u32() != 1
	case primArc:
		box()
		p.pts = append(p.pts, point{c.i32(), c.i32()}, point{c.i32(), c.i32()})
	case primPolygon, primPolyline, primBezier:
		c.u32() // line style
		c.u32() // line width
		if kind == primPolygon {
			p.filled = c.u32() != 1
			c.u32() // hatch style
		}
		for n := int(c.u16()); n > 0 && c.err == nil; n-- {
			y := c.i16() // points are stored Y first
			x := c.i16()
			p.pts = append(p.pts, point{int32(x), int32(y)})
		}
	case primText:
		box()
		p.pts = []point{{c.i32(), c.i32()}}
		p.font = int(c.u16())
		c.skip(2)
		p.text = c.lzt()
	default:
		return p, false
	}
	return p, true
}

// Package svg builds SVG documents ergonomically, replacing hand-formatted fmt.Fprintf
// calls. It is a zero-dependency element writer: attribute values carry their own
// formatting (F/I) and text content is XML-escaped (Text), so callers avoid quote-soup and
// manual escaping. It is a serialization helper, not a renderer: it holds no geometry model
// and makes no drawing decisions (those stay in the render package).
package svg

import (
	"encoding/xml"
	"strconv"
	"strings"
)

// Attr is one element attribute. Construct with A, F, or I so numeric formatting lives in
// one place rather than at each call site. The value is written verbatim inside quotes, so
// it must not contain a double quote; the constructors only ever produce numbers, and A is
// for caller-controlled tokens (colors, anchors, point lists) that never do.
type Attr struct{ key, val string }

// A is a raw attribute: the value is written verbatim (colors, text-anchor, point lists,
// and other tokens with no quote characters).
//
// NEVER pass data read from a design file to this. Attribute values are written unescaped, so a net
// or component name containing a quote would close the attribute and inject markup into a document
// the viewer mounts with innerHTML. Text CONTENT has always been escaped (see Text); attributes were
// safe only because nothing but colors and numbers went into them, which stopped being true when
// entity keys arrived. Use AEsc for anything a reader's file supplies.
func A(key, val string) Attr { return Attr{key, val} }

// AEsc is an attribute whose value comes from the design: net names, designators, pin numbers. The
// value is escaped, so a name carrying a quote or an angle bracket lands as text rather than as
// markup.
func AEsc(key, val string) Attr { return Attr{key, escapeAttr(val)} }

// escapeAttr escapes a value for an attribute in double quotes. xml.EscapeText covers the five XML
// entities plus the control whitespace, which is what an attribute needs.
func escapeAttr(v string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(v))
	return b.String()
}

// F is a float attribute formatted to one decimal place.
func F(key string, v float64) Attr { return Attr{key, f1(v)} }

// I is an integer attribute.
func I(key string, v int64) Attr { return Attr{key, strconv.FormatInt(v, 10)} }

// Canvas accumulates an SVG document. Create it with Open and finish with String.
type Canvas struct{ b strings.Builder }

// Open starts an <svg> element sized pxW by pxH with a matching viewBox (0 0 pxW pxH).
// Extra attrs (e.g. font-family) are added to the root element.
func Open(pxW, pxH float64, attrs ...Attr) *Canvas {
	c := &Canvas{}
	c.b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"`)
	writeAttrs(&c.b, F("width", pxW), F("height", pxH), A("viewBox", "0 0 "+f1(pxW)+" "+f1(pxH)))
	writeAttrs(&c.b, attrs...)
	c.b.WriteByte('>')
	return c
}

// El writes a self-closing element on its own line: <tag k="v" .../>.
func (c *Canvas) El(tag string, attrs ...Attr) {
	c.b.WriteString("\n<")
	c.b.WriteString(tag)
	writeAttrs(&c.b, attrs...)
	c.b.WriteString("/>")
}

// Group opens a container element (<g k="v" ...>) that stays open until GroupEnd. Used to
// stratify a document into toggleable layers (the board renderer's class-per-layer groups);
// nesting is the caller's responsibility.
func (c *Canvas) Group(attrs ...Attr) {
	c.b.WriteString("\n<g")
	writeAttrs(&c.b, attrs...)
	c.b.WriteByte('>')
}

// GroupEnd closes the most recently opened Group.
func (c *Canvas) GroupEnd() {
	c.b.WriteString("\n</g>")
}

// Text writes a <text> element whose content is XML-escaped: <text ...>content</text>.
func (c *Canvas) Text(content string, attrs ...Attr) {
	c.b.WriteString("\n<text")
	writeAttrs(&c.b, attrs...)
	c.b.WriteByte('>')
	_ = xml.EscapeText(&c.b, []byte(content))
	c.b.WriteString("</text>")
}

// String closes the document and returns the SVG source.
func (c *Canvas) String() string { return c.b.String() + "\n</svg>\n" }

func writeAttrs(b *strings.Builder, attrs ...Attr) {
	for _, a := range attrs {
		b.WriteByte(' ')
		b.WriteString(a.key)
		b.WriteString(`="`)
		b.WriteString(a.val)
		b.WriteByte('"')
	}
}

func f1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

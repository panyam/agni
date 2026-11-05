package orcad

import (
	"fmt"
	"strings"
)

// The format family a Library stream's major version selects. 3 and up is the modern family, and
// Clay R6's Capture 16.6 wrote 3.2. The legacy family frames its streams differently and is refused.
const minVersionMajor = 3

// library is the Library stream: the format version and the string table that every property
// index in every other stream resolves against.
type library struct {
	major, minor uint16
	strs         []string
	root         string // the root schematic folder under Views
}

// str is string i of the table. An index past the table is "", as Capture writes 0 for none.
func (l *library) str(i uint32) string {
	if int(i) < len(l.strs) {
		return l.strs[i]
	}
	return ""
}

// props resolves a structure's property pairs into names and values.
func (l *library) props(pairs [][2]uint32) map[string]string {
	out := map[string]string{}
	for _, p := range pairs {
		if k := l.str(p[0]); k != "" {
			out[k] = l.str(p[1])
		}
	}
	return out
}

const (
	logfontSize      = 60
	pageSettingsSize = 156
)

func parseLibrary(b []byte) (*library, error) {
	c := &cursor{b: b}
	if !c.need(32) {
		return nil, fmt.Errorf("the Library stream is %d bytes, shorter than its header", len(b))
	}
	intro := strings.TrimRight(string(b[:32]), "\x00")
	if i := strings.IndexByte(intro, 0); i >= 0 {
		intro = intro[:i]
	}
	if !strings.HasPrefix(intro, "OrCAD Windows Design") {
		return nil, fmt.Errorf("not an OrCAD Capture design (its Library stream opens %q)", strings.TrimSpace(intro))
	}
	c.p = 32
	l := &library{major: c.u16(), minor: c.u16()}
	if c.err == nil && l.major < minVersionMajor {
		return nil, fmt.Errorf("an OrCAD Capture design in the legacy format %d.%d, which agni does not read", l.major, l.minor)
	}
	c.skip(12) // created, modified, four zero bytes
	fonts := int(c.u16())
	if fonts < 1 {
		c.fail("font count %d", fonts)
	}
	c.skip(logfontSize * (fonts - 1))
	c.skip(2 * int(c.u16())) // the design template's font assignments
	c.skip(8)
	for i := 0; i < 8; i++ { // the eight named part fields
		c.lzt()
	}
	c.skip(pageSettingsSize)
	n := int(c.u32())
	if c.err == nil && n > len(b) {
		c.fail("string table of %d entries in a %d-byte stream", n, len(b))
	}
	for i := 0; i < n && c.err == nil; i++ {
		l.strs = append(l.strs, c.lzt())
	}
	if c.err != nil {
		return nil, fmt.Errorf("the Library stream: %w", c.err)
	}
	// The aliases and the root folder's name follow. A design that names no root uses its first
	// schematic folder, so a short tail is not an error.
	for na := int(c.u16()); na > 0 && c.err == nil; na-- {
		c.lzt()
		c.lzt()
	}
	c.skip(8)
	if root := c.lzt(); c.err == nil {
		l.root = root
	}
	return l, nil
}

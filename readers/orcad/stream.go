package orcad

import (
	"encoding/binary"
	"fmt"

	"github.com/panyam/agni/internal/latin1"
)

// cursor reads one stream. Every read is bounds-checked, and the first failure sticks: later reads
// return zero values, and the caller checks err once at a point where a record is complete.
type cursor struct {
	b   []byte
	p   int
	err error
}

func (c *cursor) fail(format string, args ...any) {
	if c.err == nil {
		c.err = fmt.Errorf(format+" (at 0x%X)", append(args, c.p)...)
	}
}

func (c *cursor) need(n int) bool {
	if c.err != nil {
		return false
	}
	if n < 0 || c.p+n > len(c.b) {
		c.fail("record runs past the end of its stream")
		return false
	}
	return true
}

func (c *cursor) u8() byte {
	if !c.need(1) {
		return 0
	}
	c.p++
	return c.b[c.p-1]
}

func (c *cursor) u16() uint16 {
	if !c.need(2) {
		return 0
	}
	c.p += 2
	return binary.LittleEndian.Uint16(c.b[c.p-2:])
}

func (c *cursor) i16() int16 { return int16(c.u16()) } // #nosec G115 -- a signed field stored as two bytes

func (c *cursor) u32() uint32 {
	if !c.need(4) {
		return 0
	}
	c.p += 4
	return binary.LittleEndian.Uint32(c.b[c.p-4:])
}

func (c *cursor) i32() int32 { return int32(c.u32()) } // #nosec G115 -- a signed field stored as four bytes

func (c *cursor) skip(n int) {
	if c.need(n) {
		c.p += n
	}
}

// lzt reads a u16 length, that many bytes and a NUL the length does not count. A non-zero
// terminator means the record was framed wrongly, so it fails rather than reading on.
func (c *cursor) lzt() string {
	n := int(c.u16())
	if !c.need(n + 1) {
		return ""
	}
	s := latin1.Decode(c.b[c.p : c.p+n])
	c.p += n
	if c.b[c.p] != 0 {
		c.fail("string of %d bytes has no terminator", n)
		return ""
	}
	c.p++
	return s
}

// Structure types the reader reads fields of. Every other type is skipped by its frame.
const (
	typeLibraryRef    = 9
	typePage          = 10
	typeBlock         = 12
	typePart          = 13
	typePinScalar     = 16
	typeWireScalar    = 20
	typeWireBus       = 21
	typePort          = 23
	typeLibraryPart   = 24
	typeSymPinScalar  = 26
	typeSymPinBus     = 27
	typePackage       = 31
	typeDevice        = 32
	typeGlobal        = 37
	typeOffPage       = 38
	typeOccurrence    = 0x42
	typeOccurrenceNet = 0x43
)

// longPrefixes is how many long prefixes open each structure type. A structure is N long prefixes
// (type, u32 length, u32 zero), one short prefix (type, i16 property count, the pairs), a preamble
// (FF E4 5C 39, u32 length, that many bytes), then its body. The table is fixed rather than probed,
// because a shorter chain also frames at the inner offsets of a longer one.
var longPrefixes = map[byte]int{
	2: 3, 6: 3, 9: 1, 10: 2, 12: 3, 13: 4, 16: 2, 17: 2, 20: 2, 21: 2, 23: 3, 24: 5, 26: 2, 27: 2,
	29: 2, 31: 3, 32: 2, 33: 4, 34: 4, 35: 4, 37: 2, 38: 2, 39: 2, 48: 2, 49: 2, 55: 2, 56: 2, 57: 2,
	58: 2, 59: 2, 60: 2, 61: 2, 62: 2, 64: 4, 65: 3, 75: 4, 76: 4, 77: 3, 88: 2, 89: 2,
	0x42: 1, 0x43: 1, 0x44: 1, 0x45: 1, 0x52: 1, 0x5B: 1,
}

var preamble = []byte{0xFF, 0xE4, 0x5C, 0x39}

// frame is one structure's prefixes. stops[i] is where long prefix i says the structure ends from
// its own offset, and stops[0] is the end of the whole structure.
type frame struct {
	typ   byte
	props [][2]uint32 // string-table indices: name, value
	body  int
	stops []int
}

func (f frame) end() int { return f.stops[0] }

// frame reads the prefixes of the structure at the cursor and leaves the cursor at its body.
func (c *cursor) frame() frame {
	start := c.p
	if !c.need(1) {
		return frame{}
	}
	t := c.b[c.p]
	n, ok := longPrefixes[t]
	if !ok {
		c.fail("structure type %d is not one the format defines", t)
		return frame{}
	}
	f := frame{typ: t}
	for i := 0; i < n; i++ {
		if c.u8() != t {
			c.fail("long prefix %d of a type-%d structure has another type", i, t)
			return frame{}
		}
		l := int(c.u32())
		if c.u32() != 0 {
			c.fail("long prefix %d of a type-%d structure has no zero pad", i, t)
			return frame{}
		}
		f.stops = append(f.stops, start+9*i+9+l)
	}
	if c.u8() != t {
		c.fail("short prefix of a type-%d structure has another type", t)
		return frame{}
	}
	for np := int(c.i16()); np > 0 && c.err == nil; np-- {
		f.props = append(f.props, [2]uint32{c.u32(), c.u32()})
	}
	if !c.need(4) {
		return frame{}
	}
	if string(c.b[c.p:c.p+4]) != string(preamble) {
		c.fail("type-%d structure has no preamble", t)
		return frame{}
	}
	c.p += 4
	c.skip(int(c.u32()))
	f.body = c.p
	for _, s := range f.stops {
		if s < f.body || s > len(c.b) {
			c.fail("type-%d structure ends outside its stream", t)
			return frame{}
		}
	}
	return f
}

// skipFrame reads a structure's prefixes and moves past the whole structure.
func (c *cursor) skipFrame() frame {
	f := c.frame()
	if c.err == nil {
		c.p = f.end()
	}
	return f
}

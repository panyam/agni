package allegro

import (
	"encoding/binary"
	"fmt"
	"github.com/panyam/agni/internal/latin1"
	"math"
	"strings"
)

// Allegro writes the format version into the file's first word, with the low byte free. 17.2 and
// 16.6 are read, each through its own layout. The other versions differ in block layouts too (agni
// issue 1355).
const (
	magicMask   = 0xFFFFFF00
	magicV172   = 0x00140400
	magicV166   = 0x00131500
	stringTable = 0x1200
)

// knownVersions names the magic words of other Allegro releases, so a refusal says which release
// wrote the file rather than only that agni does not read it.
var knownVersions = map[uint32]string{
	0x00130000: "16.0", 0x00130200: "16.1", 0x00130400: "16.2", 0x00130500: "16.2",
	0x00130C00: "16.4", 0x00131000: "16.5",
	0x00140500: "17.2 (variant)", 0x00140600: "17.2 (variant)", 0x00140700: "17.2 (variant)",
	0x00140900: "17.4", 0x00141500: "17.5", 0x00150000: "18.0",
}

// Header offsets, the same in 16.6 and 17.2. A linked-list descriptor is two words, the tail sentinel first
// and the head key second. Lists 0 to 17 sit at 0x03C and lists 18 to 21 at 0x0D4.
const (
	hdrVersionString = 0x0F8
	hdrUnits         = 0x180
	hdr0x27End       = 0x18C
	hdrStringCount   = 0x194
	hdrDivisor       = 0x26C
	hdrLayerMap      = 0x428
	layerMapEntries  = 25
)

// The header's linked lists the reader walks.
const (
	listShapes   = 3  // zone boundaries (0x28)
	listGraphics = 4  // board-level graphics (0x14)
	listNets     = 5  // nets (0x1B)
	listRects    = 7  // rectangles and shapes, the board outline among them
	listFootDefs = 9  // footprint definitions (0x2B)
	listText     = 10 // fields and board text (0x03, 0x30)
	listTables   = 18 // definition tables (0x36), the font table among them
)

// Block tags the reader reads fields of. The walk knows every tag's size whether or not it is read.
const (
	tagArc        = 0x01
	tagField      = 0x03
	tagNetAssign  = 0x04
	tagTrack      = 0x05
	tagCompDef    = 0x06
	tagCompInst   = 0x07
	tagPad        = 0x0D
	tagGraphic    = 0x14
	tagLineH      = 0x15
	tagLineD      = 0x16
	tagLineV      = 0x17
	tagNet        = 0x1B
	tagPadstack   = 0x1C
	tagShape      = 0x28
	tagRect       = 0x24
	tagLayerList  = 0x2A
	tagFootDef    = 0x2B
	tagTable      = 0x2C
	tagFootInst   = 0x2D
	tagText       = 0x30
	tagStrGraphic = 0x31
	tagPlacedPad  = 0x32
	tagVoid       = 0x34
	tagVia        = 0x33
	tagDefTable   = 0x36
	tagPtrArray   = 0x37
)

// board is a parsed .brd file: the bytes, the string table, and every block's offset by key.
type board struct {
	b       []byte
	strs    map[uint32]string
	keys    map[uint32]int // block key -> offset of its tag
	nblocks int
	units   byte
	divisor uint32
	version string
	l       *layout
}

// parse checks the version, reads the string table and walks every block to the end of the object
// stream. A block records no length of its own, so a tag whose size is unknown stops the walk with
// an error rather than reading on from a wrong place.
func parse(b []byte) (*board, error) {
	if len(b) < stringTable {
		return nil, fmt.Errorf("allegro: %d bytes is shorter than an Allegro header", len(b))
	}
	if string(b[0xF8:0xFB]) != "all" {
		return nil, fmt.Errorf("allegro: not an Allegro board (no \"all\" at 0xF8)")
	}
	magic := le32(b, 0) & magicMask
	l, ok := layouts[magic]
	if !ok {
		if rel, ok := knownVersions[magic]; ok {
			return nil, fmt.Errorf("allegro: written by Allegro %s, and only 16.6 and 17.2 boards are read (agni issue 1355)", rel)
		}
		return nil, fmt.Errorf("allegro: unknown format version 0x%08X, and only 16.6 and 17.2 boards are read", magic)
	}
	p := &board{
		l:       l,
		b:       b,
		strs:    map[uint32]string{},
		keys:    map[uint32]int{},
		units:   b[hdrUnits],
		divisor: le32(b, hdrDivisor),
		version: cstring(b[hdrVersionString : hdrVersionString+60]),
	}
	if p.divisor == 0 {
		return nil, fmt.Errorf("allegro: units divisor is zero")
	}
	if _, ok := unitNm[p.units]; !ok {
		return nil, fmt.Errorf("allegro: unknown board unit %d", p.units)
	}
	off, err := p.readStrings(int(le32(b, hdrStringCount)))
	if err != nil {
		return nil, err
	}
	if err := p.walk(off); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *board) readStrings(n int) (int, error) {
	off := stringTable
	for i := 0; i < n; i++ {
		if off+4 > len(p.b) {
			return 0, fmt.Errorf("allegro: string table runs past the end of the file")
		}
		id := le32(p.b, off)
		end := off + 4
		for end < len(p.b) && p.b[end] != 0 {
			end++
		}
		if end >= len(p.b) {
			return 0, fmt.Errorf("allegro: unterminated string %d", id)
		}
		p.strs[id] = latin1.Decode(p.b[off+4 : end])
		off = align4(end + 1)
	}
	return off, nil
}

// walk records every block's offset by its key, from the first block after the string table to a
// zero tag or the end of the file.
func (p *board) walk(off int) error {
	for off < len(p.b) && p.b[off] != 0 {
		size, err := p.blockSize(off)
		if err != nil {
			return err
		}
		if size < 4 || off+size > len(p.b) {
			return fmt.Errorf("allegro: block 0x%02X at 0x%X runs past the end of the file", p.b[off], off)
		}
		if k, ok := p.keyOf(off, size); ok {
			p.keys[k] = off
		}
		p.nblocks++
		off += size
	}
	return nil
}

// variableSize names the block types whose size is read from the block itself.
var variableSize = map[byte]bool{
	0x03: true, 0x1C: true, 0x1D: true, 0x1E: true, 0x1F: true, 0x21: true, 0x27: true,
	0x2A: true, 0x31: true, 0x36: true, 0x3B: true, 0x3C: true,
}

// blockSize is the size of the block whose tag is at off. Every size is a multiple of four.
func (p *board) blockSize(off int) (int, error) {
	tag := p.b[off]
	if n, ok := p.l.fixedSize[tag]; ok {
		return n, nil
	}
	if !variableSize[tag] {
		return 0, fmt.Errorf("allegro: unknown block type 0x%02X at 0x%X", tag, off)
	}
	if off+32 > len(p.b) && tag != 0x27 {
		return 0, fmt.Errorf("allegro: block 0x%02X at 0x%X is cut short", tag, off)
	}
	switch tag {
	case 0x03:
		return p.fieldSize(off)
	case 0x1C: // padstack: fixed components and some per copper layer, plus N trailing records
		l := p.l
		comps := l.psFixed + l.psPerLayer*int(le16(p.b, off+l.psCount))
		return l.psHeader + l.psComp*comps - l.psLastShort + l.psTrailer*int(p.b[off+2]), nil
	case 0x1D: // constraint set: B per-layer records of 56 bytes, then A name records of 256
		return p.l.conSetHeader + 56*int(le16(p.b, off+22)) + 256*int(le16(p.b, off+20)), nil
	case 0x1E:
		return p.l.siModelBase + align4(int(le32(p.b, off+20))), nil
	case 0x1F:
		return p.l.dimBase + 280*int(le16(p.b, off+26)), nil
	case 0x21:
		n := int(le32(p.b, off+4))
		if n < 12 {
			return 0, fmt.Errorf("allegro: block 0x21 at 0x%X has size %d", off, n)
		}
		return n, nil
	case 0x27: // runs to the offset the header records, less one
		end := int(le32(p.b, hdr0x27End)) - 1
		if end <= off {
			return 0, fmt.Errorf("allegro: block 0x27 at 0x%X ends before it starts", off)
		}
		return end - off, nil
	case 0x2A:
		return 8 + 12*int(le16(p.b, off+2)), nil
	case 0x31:
		return 24 + align4(int(le16(p.b, off+22))), nil
	case 0x36:
		code := le16(p.b, off+2)
		item, ok := p.l.defItemSize[code]
		if !ok {
			return 0, fmt.Errorf("allegro: definition table 0x36 at 0x%X has unknown code 0x%X", off, code)
		}
		return p.l.defItems + item*int(le32(p.b, off+p.l.defCount)), nil
	case 0x3B:
		return p.l.propBase + align4(int(le32(p.b, off+4))), nil
	case 0x3C:
		return 12 + 4*int(le32(p.b, off+8)), nil
	}
	return 0, fmt.Errorf("allegro: unknown block type 0x%02X at 0x%X", tag, off)
}

// fieldSize sizes a 0x03 field block from its subtype. The payload starts at fieldBody.
func (p *board) fieldSize(off int) (int, error) {
	b, base := p.b, p.l.fieldBody
	sub := b[off+p.l.fieldSub]
	size := int(le16(b, off+p.l.fieldLen))
	switch sub {
	case 0x65:
		return base, nil
	case 0x64, 0x66, 0x67, 0x6A:
		return base + 4, nil
	case 0x69:
		return base + 8, nil
	case 0x68, 0x6B, 0x6D, 0x6E, 0x6F, 0x71, 0x73, 0x78:
		return base + align4(size), nil
	case 0x6C:
		return base + 4 + 4*int(le32(b, off+base)), nil
	case 0x70, 0x74:
		x0, x1 := int(le16(b, off+base)), int(le16(b, off+base+2))
		return base + 4 + x1 + 4*x0, nil
	case 0xF6:
		return base + 80, nil
	}
	if size == 4 || size == 8 {
		return base + size, nil
	}
	return 0, fmt.Errorf("allegro: field at 0x%X has subtype 0x%X and size %d", off, sub, size)
}

// keyOf is the block's key. Most blocks carry it at +4. A layer list carries it after its entries,
// a 0x21 at +8, and a few blocks carry none.
func (p *board) keyOf(off, size int) (uint32, bool) {
	switch p.b[off] {
	case 0x27, 0x35, 0x3B:
		return 0, false
	case 0x21:
		return le32(p.b, off+8), true
	case tagLayerList:
		return le32(p.b, off+size-4), true
	}
	return le32(p.b, off+4), true
}

// at returns the offset of the block a key names, when that block has one of the given tags.
func (p *board) at(key uint32, tags ...byte) (int, bool) {
	off, ok := p.keys[key]
	if !ok || key == 0 {
		return 0, false
	}
	if len(tags) == 0 {
		return off, true
	}
	for _, t := range tags {
		if p.b[off] == t {
			return off, true
		}
	}
	return 0, false
}

// chain walks a singly linked list from head, reading each block's next key at nextAt, until a key
// names no block or equals stop. The step limit guards against a cycle in a damaged file.
func (p *board) chain(head, stop uint32, nextAt func(tag byte) int, visit func(off int)) {
	key := head
	for steps := 0; key != 0 && key != stop && steps <= p.nblocks; steps++ {
		off, ok := p.keys[key]
		if !ok {
			return
		}
		visit(off)
		key = le32(p.b, off+nextAt(p.b[off]))
	}
}

// list walks one of the header's linked lists. Each block's next key is at +8.
func (p *board) list(i int, visit func(off int)) {
	at := 0x03C + 8*i
	if i >= 18 {
		at = 0x0D4 + 8*(i-18)
	}
	tail, head := le32(p.b, at), le32(p.b, at+4)
	p.chain(head, tail, func(tag byte) int {
		if tag == tagFootDef {
			return 32
		}
		return 8
	}, visit)
}

func (p *board) u32(off, field int) uint32 { return le32(p.b, off+field) }
func (p *board) i32(off, field int) int32  { return int32(le32(p.b, off+field)) } // #nosec G115 -- a coordinate is a signed word on disk
func (p *board) str(id uint32) string      { return p.strs[id] }

// float reads an Allegro double, whose high word is stored first.
func (p *board) float(off, field int) float64 {
	hi, lo := uint64(le32(p.b, off+field)), uint64(le32(p.b, off+field+4))
	return math.Float64frombits(hi<<32 | lo)
}

// unitNm is nanometres per board unit, before the divisor.
var unitNm = map[byte]float64{1: 25400, 2: 25400000, 3: 1000000, 4: 10000000, 5: 1000}

// nm converts a file coordinate to nanometres.
func (p *board) nm(v float64) int64 {
	return int64(math.Round(v * unitNm[p.units] / float64(p.divisor)))
}

func le32(b []byte, off int) uint32 {
	if off < 0 || off+4 > len(b) {
		return 0
	}
	return binary.LittleEndian.Uint32(b[off:])
}

func le16(b []byte, off int) uint16 {
	if off < 0 || off+2 > len(b) {
		return 0
	}
	return binary.LittleEndian.Uint16(b[off:])
}

func align4(n int) int { return (n + 3) &^ 3 }

// cstring is the bytes up to the first NUL, as Latin-1.
func cstring(b []byte) string {
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		b = b[:i]
	}
	return latin1.Decode(b)
}

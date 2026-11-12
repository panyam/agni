package allegro

// A layout is where one Allegro release puts each field the reader reads. 17.2 added words to many
// block types, so a 16.6 block is shorter and its later fields sit earlier. Every offset is from the
// block's tag byte.
//
// The 16.6 layout is from KiCad's Allegro format notes (pcbnew/pcb_io/allegro), which mark each
// field that 17.2 added or removed, and is checked against two 16.6 boards and their second views
// (agni issue 1388).
type layout struct {
	release string

	fixedSize   map[byte]int
	defItemSize map[uint16]int

	// A 0x03 field's subtype, size and payload offsets.
	fieldSub, fieldLen, fieldBody int

	// The padstack (0x1C): its header length, one component's length, the fixed technical-layer
	// slots before the per-layer ones, the slots per copper layer, and where the layer count, drill
	// and slot sit. A component's width and height start at psW. In 16.6 the last component is
	// four bytes short.
	psHeader, psComp, psFixed, psPerLayer int
	psCount, psDrill, psSlotX, psSlotY    int
	psW                                   int
	psLastShort                           int
	psTrailer                             int // bytes per trailing record, times the count at +2

	conSetHeader int // a 0x1D's bytes outside its records
	siModelBase  int // a 0x1E's bytes outside its string
	dimBase      int // a 0x1F's bytes outside its 280-byte records
	propBase     int // a 0x3B's bytes outside its value

	// A component instance (0x07): its ref-des string and its definition. In 16.6 an instance
	// names no definition, so compDef is 0 and the definition's own instance chain is followed
	// instead.
	compRef, compDef int

	// A placed footprint (0x2D): rotation, position, its 0x07, and the heads of its graphics, its
	// placed pads and its text.
	fiRot, fiX, fiInstRef, fiGraphics, fiPads, fiText int

	// A placed pad (0x32): the next pad in its footprint, its footprint, and its footprint pad.
	ppNextInFp, ppParentFp, ppPad int

	padRot int // a footprint pad's (0x0D) rotation

	netAssign int // a net's (0x1B) first assignment

	trackSegs  int // a track's (0x05) first segment
	segWidth   int // a line's or arc's width; start, end, centre and radius follow from it
	graphSegs  int // a graphic's (0x14) first segment
	shapeSegs  int // a shape's (0x28) first segment
	shapeTable int // a shape's table (0x2C)
	tablePtr1  int // a table's (0x2C) pointer array

	// Text (0x30): its string graphic, the group it belongs to, position and rotation.
	textStr, textGroup, textX, textRot int

	rectCoords int // a rectangle's (0x24) corners, four words

	viaX, viaPadstack int

	defCount, defItems int // a 0x36's item count and the offset of its first item
}

// arc reads an arc's (0x01) fields from its width: start and end points, then the centre and
// radius as doubles.
func (l *layout) arcStart() int  { return l.segWidth + 4 }
func (l *layout) arcEnd() int    { return l.segWidth + 12 }
func (l *layout) arcCentre() int { return l.segWidth + 20 }

var layout172 = &layout{
	release: "17.2",
	fixedSize: map[byte]int{
		0x01: 84, 0x04: 20, 0x05: 68, 0x06: 40, 0x07: 48, 0x08: 32, 0x09: 48, 0x0A: 72, 0x0C: 64,
		0x0D: 44, 0x0E: 68, 0x0F: 60, 0x10: 36, 0x11: 24, 0x12: 28, 0x14: 36, 0x15: 44, 0x16: 44,
		0x17: 44, 0x1A: 88, 0x1B: 60, 0x20: 40, 0x22: 44, 0x23: 84, 0x24: 56, 0x26: 24, 0x28: 76,
		0x29: 56, 0x2B: 76, 0x2C: 44, 0x2D: 72, 0x2E: 40, 0x2F: 32, 0x30: 56, 0x32: 84, 0x33: 80,
		0x34: 36, 0x35: 124, 0x37: 428, 0x38: 52, 0x39: 60, 0x3A: 16, 0x3E: 44,
	},
	defItemSize: map[uint16]int{
		0x02: 108, 0x03: 64, 0x05: 28, 0x06: 8, 0x08: 64, 0x0B: 1016, 0x0C: 232, 0x0D: 200,
		0x0F: 20, 0x10: 108, 0x12: 1052,
	},
	fieldSub: 16, fieldLen: 18, fieldBody: 24,
	psHeader: 192, psComp: 36, psFixed: 21, psPerLayer: 4,
	psCount: 44, psDrill: 64, psSlotX: 76, psSlotY: 80, psW: 8, psTrailer: 40,
	conSetHeader: 28, siModelBase: 28, dimBase: 36, propBase: 180,
	compRef: 28, compDef: 12,
	fiRot: 28, fiX: 32, fiInstRef: 40, fiGraphics: 44, fiPads: 48, fiText: 52,
	ppNextInFp: 24, ppParentFp: 28, ppPad: 36,
	padRot:    40,
	netAssign: 28,
	trackSegs: 56, segWidth: 24, graphSegs: 24, shapeSegs: 40, shapeTable: 52, tablePtr1: 28,
	textStr: 28, textGroup: 32, textX: 40, textRot: 52,
	rectCoords: 24,
	viaX:       32, viaPadstack: 44,
	defCount: 16, defItems: 32,
}

// layout166 drops the words 17.2 added. In three blocks 16.6 also holds a word that 17.2 dropped or
// moved. A 0x07 has one more word after its footprint. A 0x2D names its 0x07 at +12 rather than +40,
// and a 0x2C has its last unknown word at +16.
var layout166 = &layout{
	release: "16.6",
	fixedSize: map[byte]int{
		0x01: 80, 0x04: 20, 0x05: 60, 0x06: 36, 0x07: 40, 0x08: 24, 0x09: 44, 0x0A: 68, 0x0C: 56,
		0x0D: 40, 0x0E: 60, 0x0F: 56, 0x10: 32, 0x11: 24, 0x12: 28, 0x14: 32, 0x15: 40, 0x16: 40,
		0x17: 40, 0x1A: 88, 0x1B: 56, 0x20: 40, 0x22: 40, 0x23: 84, 0x24: 52, 0x26: 20, 0x28: 68,
		0x29: 56, 0x2B: 72, 0x2C: 36, 0x2D: 64, 0x2E: 36, 0x2F: 32, 0x30: 44, 0x32: 76, 0x33: 72,
		0x34: 32, 0x35: 124, 0x37: 428, 0x38: 52, 0x39: 60, 0x3A: 16, 0x3E: 44,
	},
	defItemSize: map[uint16]int{
		0x02: 100, 0x03: 32, 0x05: 28, 0x06: 208, 0x08: 32, 0x0B: 1016, 0x0C: 232, 0x0D: 200,
		0x0F: 20, 0x10: 108, 0x12: 1052,
	},
	fieldSub: 12, fieldLen: 14, fieldBody: 16,
	psHeader: 88, psComp: 28, psFixed: 11, psPerLayer: 3,
	psCount: 50, psDrill: 16, psSlotX: 72, psSlotY: 76, psW: 4, psLastShort: 4, psTrailer: 32,
	conSetHeader: 24, siModelBase: 24, dimBase: 32, propBase: 176,
	compRef: 20, compDef: 0,
	fiRot: 24, fiX: 28, fiInstRef: 12, fiGraphics: 36, fiPads: 40, fiText: 44,
	ppNextInFp: 20, ppParentFp: 24, ppPad: 32,
	padRot:    36,
	netAssign: 24,
	trackSegs: 48, segWidth: 20, graphSegs: 20, shapeSegs: 32, shapeTable: 48, tablePtr1: 20,
	textStr: 12, textGroup: 40, textX: 24, textRot: 36,
	rectCoords: 20,
	viaX:       24, viaPadstack: 36,
	defCount: 12, defItems: 28,
}

// layouts maps the masked magic word of each release read to its layout.
var layouts = map[uint32]*layout{
	magicV172: layout172,
	magicV166: layout166,
}

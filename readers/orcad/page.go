package orcad

import "fmt"

// point is a page position in Capture's units of 10 mil. A placed pin records its own absolute
// position, so connectivity needs no symbol transform.
type point struct{ x, y int32 }

// wire is a wire on a page. net keys the page's net table. A bus wire is drawn and joins no net.
type wire struct {
	id   uint32 // the wire's own id; Capture numbers an unnamed net after the lowest on it
	net  uint32
	a, b point
	bus  bool
}

// alias is a net name drawn on a wire (type 49).
type alias struct {
	text  string
	at    point
	turns int
	font  int
}

// titleBlock is a placed title block (type 65): its symbol, where it sits, and its fields.
type titleBlock struct {
	symbol string
	at     point
	props  map[string]string
}

// placedPin is one pin of a placed part. index is the 1-based symbol slot, and a negative index
// marks the pin as explicitly not connected. net is the page net the pin names itself, or 0. Most
// pins leave it 0 and join their net through a wire; a pin on no wire can name one this way.
type placedPin struct {
	index int16
	at    point
	net   uint32
}

// shown is one property a placement displays (type 39), at a position relative to its placement.
// font is 1-based into the Library's font table, and turns is how many quarter turns the text is
// rotated.
type shown struct {
	name  string
	at    point
	font  int
	turns int
}

// placedPart is a part on a page (type 13). at is the corner its symbol's box is placed at, and
// orient holds the quarter turns in bits 0 and 1 and a mirror in bit 2.
type placedPart struct {
	at       point
	orient   byte
	shown    []shown
	db       uint32 // matched by an occurrence's target
	cacheKey string // the LibraryPart, such as "RESISTOR.Normal"
	ref      string // the placement's own designator, often the "R?" template
	value    string
	props    map[string]string
	pins     []placedPin
	pkg      string // the Package
	unit     int    // the device within the package
}

// block is a hierarchical block on a page (type 12). Its pins sit at absolute positions, in the
// order of its interface's pin names.
type block struct {
	db       uint32
	at       point   // the block rectangle's corner
	iface    drawing // the interface's drawing, whose box is the rectangle's size
	ref      string
	file     string // the block's design, relative to this page's design
	folder   string // the schematic folder within that design
	pinNames []string
	pins     []placedPin
}

// symbolKind says what a named page symbol joins.
type symbolKind int

const (
	symPort    symbolKind = iota // a hierarchical port: joins the block pin of the same name
	symGlobal                    // a power symbol: joins every net of its name in the design
	symOffPage                   // an off-page connector: joins its name within the schematic
)

// pageSymbol is a port, power symbol or off-page connector, by the net name it carries. lo and hi
// are the corners of its stored bounding box, in absolute page units. The symbol's connection point
// lies on that box, at the middle of an edge, so a wire end inside the box is the wire it connects
// to. A corner is never the connection point, and a wire passing beside the symbol can end on one.
type pageSymbol struct {
	kind   symbolKind
	name   string
	lo, hi point
	symbol string // its cached symbol, such as "GND" or "PORTRIGHT-R"
	at     point  // its anchor; the symbol is placed from lo, the box's corner, not from here
	orient byte
}

func (s pageSymbol) covers(p point) bool {
	if p.x < s.lo.x || p.x > s.hi.x || p.y < s.lo.y || p.y > s.hi.y {
		return false
	}
	corner := (p.x == s.lo.x || p.x == s.hi.x) && (p.y == s.lo.y || p.y == s.hi.y)
	return !corner
}

// page is one schematic page (type 10).
type page struct {
	name    string
	size    point // the page's width and height, in Capture's units
	aliases []alias
	titles  []titleBlock
	netName map[uint32][]string // net table: a wire's net id to the names recorded for it
	wires   []wire
	parts   []placedPart
	blocks  []block
	symbols []pageSymbol
}

func parsePage(b []byte, lib *library) (*page, error) {
	c := &cursor{b: b}
	f := c.frame()
	if c.err == nil && f.typ != typePage {
		return nil, fmt.Errorf("page stream opens with a type-%d structure", f.typ)
	}
	pg := &page{name: c.lzt(), netName: map[uint32][]string{}}
	c.lzt() // page size
	pg.size = pageSize(c)
	for n := int(c.u16()); n > 0 && c.err == nil; n-- {
		tf := c.frame()
		if c.err != nil {
			break
		}
		tb := titleBlock{props: lib.props(tf.props)}
		c.u32()
		c.u32() // source library
		tb.symbol = c.lzt()
		c.u32() // db id
		y, x := c.i16(), c.i16()
		tb.at = point{int32(x), int32(y)}
		if c.err == nil {
			pg.titles = append(pg.titles, tb)
			c.p = tf.end()
		}
	}
	for n := int(c.u16()); n > 0 && c.err == nil; n-- { // net records: style, not connectivity
		c.skip(13)
		c.lzt()
		c.skip(16)
	}
	for n := int(c.u16()); n > 0 && c.err == nil; n-- { // net groups: a bus's members
		c.skip(13)
		c.lzt()
		c.skip(16)
		c.skip(4 * int(c.u16()))
	}
	for n := int(c.u16()); n > 0 && c.err == nil; n-- {
		name := c.lzt()
		id := c.u32()
		pg.netName[id] = append(pg.netName[id], name)
	}
	for n := int(c.u16()); n > 0 && c.err == nil; n-- {
		wf := c.frame()
		w := wire{id: c.u32()}
		w.net, w.bus = c.u32(), wf.typ == typeWireBus
		c.u32() // colour
		w.a = point{c.i32(), c.i32()}
		w.b = point{c.i32(), c.i32()}
		c.u8()
		for na := int(c.u16()); na > 0 && c.err == nil; na-- {
			af := c.frame()
			a := alias{at: point{c.i32(), c.i32()}}
			c.u32() // colour
			a.turns = int(c.u32() & 3)
			a.font = int(c.u32())
			a.text = c.lzt()
			if c.err == nil {
				pg.aliases = append(pg.aliases, a)
				c.p = af.end()
			}
		}
		if c.err == nil && (wf.typ == typeWireScalar || wf.typ == typeWireBus) {
			pg.wires = append(pg.wires, w)
		}
		if c.err == nil {
			c.p = wf.end()
		}
	}
	for n := int(c.u16()); n > 0 && c.err == nil; n-- {
		pf := c.frame()
		switch {
		case c.err != nil:
		case pf.typ == typePart:
			if p := readPart(c, pf, lib); c.err == nil {
				pg.parts = append(pg.parts, p)
			}
		case pf.typ == typeBlock:
			if bl := readBlock(c, pf, lib); c.err == nil {
				pg.blocks = append(pg.blocks, bl)
			}
		default:
			c.fail("placed list holds a type-%d structure", pf.typ)
		}
		if c.err == nil {
			c.p = pf.end()
		}
	}
	for _, list := range []struct {
		kind    symbolKind
		trailer int
	}{{symPort, 0}, {symGlobal, 5}, {symOffPage, 5}} {
		for n := int(c.u16()); n > 0 && c.err == nil; n-- {
			sf := c.frame()
			sym := pageSymbol{kind: list.kind, name: lib.str(c.u32())}
			c.u32() // source library
			sym.symbol = c.lzt()
			c.u32() // db id
			ay, ax := c.i16(), c.i16()
			sym.at = point{int32(ax), int32(ay)}
			y2, x2, x1, y1 := c.i16(), c.i16(), c.i16(), c.i16()
			sym.lo = point{int32(min(x1, x2)), int32(min(y1, y2))}
			sym.hi = point{int32(max(x1, x2)), int32(max(y1, y2))}
			c.u8() // colour
			sym.orient = c.u8()
			if c.err == nil {
				pg.symbols = append(pg.symbols, sym)
				c.p = sf.end() + list.trailer
			}
		}
	}
	if c.err != nil {
		return nil, c.err
	}
	return pg, nil
}

// pageSize reads a page's settings (156 bytes) and returns its width and height in Capture's
// units of 10 mil. The settings store mils, or micrometres when the page is metric.
func pageSize(c *cursor) point {
	start := c.p
	c.skip(24)
	w, h := int64(c.u32()), int64(c.u32())
	c.p = start + 124
	metric := c.u32() != 0
	c.p = start + pageSettingsSize
	if metric {
		return point{int32(w * 100 / 25400), int32(h * 100 / 25400)} // #nosec G115 -- a page's size
	}
	return point{int32(w / 10), int32(h / 10)} // #nosec G115 -- a page's size
}

func readPart(c *cursor, f frame, lib *library) placedPart {
	c.u32()
	c.u32() // source library
	p := placedPart{cacheKey: c.lzt(), db: c.u32(), props: lib.props(f.props)}
	c.skip(8) // bounding box, Y first
	p.at = point{int32(c.i16()), int32(c.i16())}
	c.u8() // colour
	p.orient = c.u8()
	c.skip(2) // part index and byte
	for n := int(c.u16()); n > 0 && c.err == nil; n-- {
		if s, ok := readShown(c, lib); ok {
			p.shown = append(p.shown, s)
		}
	}
	c.skip(1)
	p.ref = c.lzt()
	p.value = lib.str(c.u32())
	c.skip(10)
	for n := int(c.u16()); n > 0 && c.err == nil; n-- {
		pf := c.frame()
		pin := placedPin{index: c.i16(), at: point{int32(c.i16()), int32(c.i16())}}
		c.u32() // a wire the pin touches
		pin.net = c.u32()
		if c.err == nil {
			p.pins = append(p.pins, pin)
			c.p = pf.end()
		}
	}
	p.pkg = c.lzt()
	p.unit = int(c.u16())
	return p
}

// readShown reads one displayed property (type 39) and leaves the cursor past it.
func readShown(c *cursor, lib *library) (shown, bool) {
	f := c.frame()
	if c.err != nil {
		return shown{}, false
	}
	s := shown{name: lib.str(c.u32()), at: point{int32(c.i16()), int32(c.i16())}}
	rf := c.u16()
	s.font, s.turns = int(rf&0x3FFF), int(rf>>14)
	if c.err == nil {
		c.p = f.end()
	}
	return s, c.err == nil && f.typ == 39
}

// readBlock reads a hierarchical block. Its interface is an inline LibraryPart whose pins name the
// block's pins. At the second-to-last prefix stop come the designator, the block's schematic folder
// and its design's path (string indices, the path relative to this design and "" for a folder of
// this design), and then the pin positions.
func readBlock(c *cursor, f frame, lib *library) block {
	c.u32() // name
	c.u32() // source library
	c.lzt()
	bl := block{db: c.u32()}
	c.skip(8) // anchor and bounding box, Y first
	bl.at = point{int32(c.i16()), int32(c.i16())}
	c.skip(4) // colour, orientation, structure id
	for n := int(c.u16()); n > 0 && c.err == nil; n-- {
		c.skipFrame()
	}
	if c.u8() != typeLibraryPart && c.err == nil {
		c.fail("block has no inline interface")
	}
	iface := c.frame()
	if c.err == nil {
		pins, stop := symbolPins(c.b, iface)
		bl.iface = readDrawing(c.b, iface, stop)
		for _, p := range pins {
			bl.pinNames = append(bl.pinNames, p.name)
		}
	}
	if len(f.stops) < 2 {
		c.fail("block has %d prefix stops", len(f.stops))
		return bl
	}
	c.p = f.stops[len(f.stops)-2]
	bl.ref = c.lzt()
	bl.folder = lib.str(c.u32())
	bl.file = lib.str(c.u32())
	c.skip(6)
	for n := int(c.u16()); n > 0 && c.err == nil; n-- {
		pf := c.frame()
		pin := placedPin{index: c.i16(), at: point{int32(c.i16()), int32(c.i16())}}
		if c.err == nil {
			bl.pins = append(bl.pins, pin)
			c.p = pf.end()
		}
	}
	return bl
}

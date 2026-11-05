package orcad

import (
	"fmt"
	"sort"
	"strings"
)

// symPin is one pin of a cached symbol. slot is the pin's index among the symbol's pin slots,
// counting skipped ones, since a package device's pin numbers are indexed by it. leg is the free
// end of the pin's line and hot its connection point, both in the symbol's own frame.
type symPin struct {
	slot     int
	name     string
	portType uint32
	leg, hot point
}

// libPart is a cached LibraryPart (type 24): a symbol's drawing and pins, plus the part properties
// every placement inherits.
type libPart struct {
	pins  []symPin
	draw  drawing
	props map[string]string
	// showNames and showNumbers are the part's last two bytes: bit 0 shows the pins' names and
	// bit 2 hides their numbers.
	showNames, showNumbers bool
}

// symbol is a cached symbol a page places that is not a part: a power symbol, a port, an off-page
// connector or a title block.
type symbol struct {
	pins []symPin
	draw drawing
}

// pkgDevice is one unit of a package. pins[slot] is the pin number of symbol slot slot, or "" for
// an empty slot.
type pkgDevice struct {
	unit string
	pins []string
}

// pkg is a cached Package (type 31): the physical part's footprint and its units.
type pkg struct {
	footprint string
	devices   []pkgDevice
}

// cache is a design's Cache stream: four counted sections of loose symbols, LibraryParts,
// PartCells and Packages. A name may be stored more than once, as revisions, and the first is the
// one placements use.
type cache struct {
	parts    map[string]*libPart // by cache name, such as "RESISTOR.Normal"
	packages map[string]*pkg     // by package name, such as "RESISTOR"
	symbols  map[string]*symbol  // by cache name, such as "GND" or "PORTRIGHT-R"
}

const (
	sectionSymbols      = 0
	sectionLibraryParts = 1
	sectionPackages     = 3
)

func parseCache(b []byte, lib *library) (*cache, error) {
	out := &cache{parts: map[string]*libPart{}, packages: map[string]*pkg{}, symbols: map[string]*symbol{}}
	c := &cursor{b: b}
	if c.u16() != 0 {
		return nil, fmt.Errorf("the Cache stream has no leading marker")
	}
	for sec := 0; sec < 4 && c.err == nil; sec++ {
		for groups := int(c.u16()); groups > 0 && c.err == nil; groups-- {
			name := c.lzt()
			for variants := int(c.u16()); variants > 0 && c.err == nil; variants-- {
				c.lzt()   // source library
				c.skip(8) // created, modified
				c.skip(2) // entry type, pad
				f := c.frame()
				if c.err != nil {
					break
				}
				switch {
				case sec == sectionLibraryParts && f.typ == typeLibraryPart:
					if _, seen := out.parts[name]; !seen {
						pins, stop := symbolPins(b, f)
						lp := &libPart{pins: pins, draw: readDrawing(b, f, stop), props: lib.props(f.props)}
						if e := f.end(); e >= f.body+2 {
							flags := uint16(b[e-2]) | uint16(b[e-1])<<8
							lp.showNames, lp.showNumbers = flags&1 != 0, flags&4 == 0
						}
						out.parts[name] = lp
					}
				case sec == sectionSymbols:
					if _, seen := out.symbols[name]; !seen {
						pins, stop := symbolPins(b, f)
						out.symbols[name] = &symbol{pins: pins, draw: readDrawing(b, f, stop)}
					}
				case sec == sectionPackages && f.typ == typePackage:
					if _, seen := out.packages[name]; !seen {
						if p, err := parsePackage(b, f); err == nil {
							out.packages[name] = p
						}
					}
				}
				c.p = f.end()
			}
		}
	}
	if c.err != nil {
		return nil, fmt.Errorf("the Cache stream: %w", c.err)
	}
	return out, nil
}

// symbolPins reads a symbol's pin list and returns the stop it starts at. The pins follow the
// symbol's drawing and its box, and the list starts at one of the structure's prefix stops, so
// each stop past the body is tried in order, and the first at which the whole list and the
// property count after it read cleanly is taken. Finding it by its stop holds even where a
// primitive the drawing reader does not know sits before it. A symbol whose pins read at no stop
// has none, and stop is then 0.
func symbolPins(b []byte, f frame) ([]symPin, int) {
	for _, s := range sortedStops(f) {
		if s <= f.body {
			continue
		}
		c := &cursor{b: b[:f.end()], p: s}
		var pins []symPin
		n := int(c.u16())
		for slot := 0; slot < n && c.err == nil; slot++ {
			if c.need(1) && b[c.p] == 0 { // a skipped slot keeps its index
				c.p++
				continue
			}
			pf := c.frame()
			if c.err == nil && pf.typ != typeSymPinScalar && pf.typ != typeSymPinBus {
				c.fail("pin slot %d is a type-%d structure", slot, pf.typ)
			}
			sp := symPin{slot: slot, name: c.lzt()}
			sp.leg = point{c.i32(), c.i32()}
			sp.hot = point{c.i32(), c.i32()}
			c.skip(4) // shape bits and two uninitialised bytes
			sp.portType = c.u32()
			if c.err == nil {
				pins = append(pins, sp)
				c.p = pf.end()
			}
		}
		c.u16() // the property count that follows the pins
		if c.err == nil {
			return pins, s
		}
	}
	return nil, 0
}

func sortedStops(f frame) []int {
	out := append([]int(nil), f.stops...)
	sort.Ints(out)
	return out
}

func parsePackage(b []byte, f frame) (*pkg, error) {
	c := &cursor{b: b[:f.end()], p: f.body}
	c.lzt() // name
	c.lzt() // source library
	c.lzt() // ref-des prefix
	c.lzt()
	p := &pkg{footprint: c.lzt()}
	for n := int(c.u16()); n > 0 && c.err == nil; n-- {
		df := c.frame()
		if c.err == nil && df.typ != typeDevice {
			c.fail("package device is a type-%d structure", df.typ)
		}
		d := pkgDevice{unit: c.lzt()}
		c.lzt() // ref-des
		for slots := int(c.u16()); slots > 0 && c.err == nil; slots-- {
			if c.need(2) && b[c.p] == 0xFF && b[c.p+1] == 0xFF {
				c.p += 2
				d.pins = append(d.pins, "")
				continue
			}
			d.pins = append(d.pins, c.lzt())
			c.u8() // bit 7 marks the pin ignored
		}
		if c.err == nil {
			p.devices = append(p.devices, d)
			c.p = df.end()
		}
	}
	return p, c.err
}

// packShorts reads a PACK_SHORT property, the pins a package shorts together, as groups of pin
// names: "(VDD,VDD1,VDD2)(GND,GND1,GND2)".
func packShorts(v string) [][]string {
	var out [][]string
	for _, g := range strings.Split(v, ")") {
		g = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(g), "("))
		if g == "" {
			continue
		}
		var names []string
		for _, n := range strings.Split(g, ",") {
			if n = strings.TrimSpace(n); n != "" {
				names = append(names, n)
			}
		}
		if len(names) > 1 {
			out = append(out, names)
		}
	}
	return out
}

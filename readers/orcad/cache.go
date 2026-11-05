package orcad

import (
	"fmt"
	"sort"
	"strings"
)

// symPin is one pin of a cached part's symbol. slot is the pin's index among the symbol's pin
// slots, counting skipped ones, since a package device's pin numbers are indexed by it.
type symPin struct {
	slot     int
	name     string
	portType uint32
}

// libPart is a cached LibraryPart (type 24): a symbol's pins, plus the part properties every
// placement inherits.
type libPart struct {
	pins  []symPin
	props map[string]string
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
}

const (
	sectionLibraryParts = 1
	sectionPackages     = 3
)

func parseCache(b []byte, lib *library) (*cache, error) {
	out := &cache{parts: map[string]*libPart{}, packages: map[string]*pkg{}}
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
						out.parts[name] = &libPart{pins: symbolPins(b, f), props: lib.props(f.props)}
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

// symbolPins reads a LibraryPart's pin list. The pins follow the symbol's drawing, whose
// primitives this reader does not decode. The list starts at one of the structure's prefix stops,
// so each stop past the body is tried in order, and the first at which the whole list and the
// property count after it read cleanly is taken. A part whose pins read at no stop has none, and
// its pins then read as unnamed.
func symbolPins(b []byte, f frame) []symPin {
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
			name := c.lzt()
			c.skip(16) // the leg's free end and its connection point, in symbol space
			c.skip(4)  // shape bits and two uninitialised bytes
			pt := c.u32()
			if c.err == nil {
				pins = append(pins, symPin{slot: slot, name: name, portType: pt})
				c.p = pf.end()
			}
		}
		c.u16() // the property count that follows the pins
		if c.err == nil {
			return pins
		}
	}
	return nil
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

package allegro

import (
	"encoding/binary"
	"math"
	"sort"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// Allegro's cross-section, the board's physical stack (agni 1601). Each row is one layer: a surface
// of air above and below, and copper and dielectric between. Each release stores the rows in its own
// way, and both were read off boards whose stack another file states.
//
// 16.6 writes one 0x21 block of sub-type 0x0518 per row, 180 bytes, with its values as fixed-width
// text: the material at +20, the thickness in metres at +40 (or with its unit, as "8 MIL"), the dielectric constant at +100 and the
// type (CONDUCTOR, PLANE, DIELECTRIC, SURFACE) at +120. The copper index is the u16 at +12, 253 for
// a row that is no copper, and the row's place in the stack is the u32 at +16. parasdr's IPC-2581
// export from Allegro states the same stack.
//
// 17.2 writes one 0x36 definition table of code 0x10, with a 108-byte item per row and empty items
// after. The item's word at +4 holds the copper index (0xFF for none), the type (0 surface,
// 1 conductor, 2 dielectric, 3 plane) and the place in the stack, a byte each. The material is a
// string id at +40. The thickness in metres, the dielectric constant and the loss tangent are
// doubles at +60, +84 and +92, each stored high word first. The BeagleBone AI's extract states the
// same total and layer count.

const (
	xsectSub     = 0x0518 // a 0x21 block's sub-type for a 16.6 cross-section row
	xsectRowSize = 180
	xsectCode    = 0x10 // a 0x36 table's code for the 17.2 cross-section
	noCopper166  = 253
)

// xsectRow is one cross-section row as either release stores it.
type xsectRow struct {
	seq       int
	copper    int // index into the copper layers, or -1
	kind      string
	material  string
	thickness float64 // metres
	er, tanD  float64 // 0 when the file states none
}

// stackup reads the board's cross-section into a Stackup, top to bottom. The air surfaces are no
// layer of the board and are left out. Allegro states no board thickness apart from the rows, so the
// stack states none. It returns nil for a board with no cross-section.
func (p *board) stackup(layers []copperLayer) *geom.Stackup {
	rows := p.xsectRows()
	if len(rows) == 0 {
		return nil
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].seq < rows[j].seq })
	s := &geom.Stackup{}
	dielectrics := 0
	for _, r := range rows {
		t := int64(math.Round(r.thickness * 1e9))
		row := &geom.StackLayer{Kind: r.kind, Material: r.material, Thickness: &t}
		switch r.kind {
		case "copper":
			if r.copper < 0 || r.copper >= len(layers) {
				continue
			}
			row.Name, row.Layer = layers[r.copper].source, layers[r.copper].name
		case "dielectric":
			dielectrics++
			row.Name = "Dielectric " + strconv.Itoa(dielectrics)
			if r.er > 0 {
				er := r.er
				row.EpsilonR = &er
			}
			if r.tanD > 0 {
				tanD := r.tanD
				row.LossTangent = &tanD
			}
		default:
			continue
		}
		s.Layers = append(s.Layers, row)
	}
	if len(s.Layers) == 0 {
		return nil
	}
	return s
}

// xsectRows reads the cross-section rows in whichever form the board's release stores them.
func (p *board) xsectRows() []xsectRow {
	var rows []xsectRow
	for _, off := range p.keys {
		switch p.b[off] {
		case 0x21:
			if le16(p.b, off+2) == xsectSub && le32(p.b, off+4) == xsectRowSize {
				rows = append(rows, p.xsectRow166(off))
			}
		case tagDefTable:
			if le16(p.b, off+2) == xsectCode {
				rows = append(rows, p.xsectRows172(off)...)
			}
		}
	}
	return rows
}

func (p *board) xsectRow166(off int) xsectRow {
	text := func(at int) string { return cstring(p.b[off+at : off+at+20]) }
	num := func(at int) float64 {
		v, _ := strconv.ParseFloat(strings.TrimSpace(text(at)), 64)
		return v
	}
	r := xsectRow{seq: int(le32(p.b, off+16)), copper: -1, material: text(20), thickness: metres(text(40)), er: num(100)}
	if c := int(le16(p.b, off+12)); c != noCopper166 {
		r.copper = c
	}
	switch text(120) {
	case "CONDUCTOR", "PLANE":
		r.kind = "copper"
	case "DIELECTRIC":
		r.kind = "dielectric"
	default:
		r.kind = "surface"
	}
	return r
}

func (p *board) xsectRows172(off int) []xsectRow {
	n := int(le32(p.b, off+p.l.defCount))
	size := p.l.defItemSize[xsectCode]
	var rows []xsectRow
	for i := 0; i < n; i++ {
		at := off + p.l.defItems + i*size
		if at+size > len(p.b) {
			break
		}
		w := le32(p.b, at+4)
		if w == 0 {
			continue
		}
		r := xsectRow{
			seq: int(w>>16) & 0xFF, copper: -1, material: p.strs[le32(p.b, at+40)],
			thickness: swappedDouble(p.b, at+60), er: swappedDouble(p.b, at+84), tanD: swappedDouble(p.b, at+92),
		}
		if c := int(w & 0xFF); c != 0xFF {
			r.copper = c
		}
		switch (w >> 8) & 0xFF {
		case 1, 3:
			r.kind = "copper"
		case 2:
			r.kind = "dielectric"
		default:
			r.kind = "surface"
		}
		rows = append(rows, r)
	}
	return rows
}

// metres reads a 16.6 row's thickness. Allegro writes most in metres as a bare number, and some as
// the length the user typed with its unit, such as "8 MIL" on Clay R6.
func metres(s string) float64 {
	f := strings.Fields(strings.ToUpper(s))
	if len(f) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0
	}
	if len(f) == 1 {
		return v
	}
	switch f[1] {
	case "MIL", "MILS":
		return v * 25.4e-6
	case "IN", "INCH", "INCHES":
		return v * 25.4e-3
	case "MM":
		return v * 1e-3
	case "UM", "MICRON", "MICRONS":
		return v * 1e-6
	case "M":
		return v
	}
	return 0
}

// swappedDouble reads a double that Allegro stores high word first.
func swappedDouble(b []byte, off int) float64 {
	hi, lo := binary.LittleEndian.Uint32(b[off:]), binary.LittleEndian.Uint32(b[off+4:])
	return math.Float64frombits(uint64(hi)<<32 | uint64(lo))
}

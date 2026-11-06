package allegro

import (
	"sort"
	"strconv"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// constraintKindNetClass is the ir.Constraint kind a net-class definition carries, as the KiCad
// project reader writes it, so the netclass rules read either format's classes.
const constraintKindNetClass = "netclass"

// netFieldConstraintSet is the code of a net's field that names its physical constraint set.
const netFieldConstraintSet = 0x1a0

// conSet is one physical constraint set (0x1D): its name, the string key nets name it by, and its
// minimum line width on each copper layer in nanometres.
type conSet struct {
	name   string
	key    uint32
	widths []int64
}

// constraintSets reads every physical constraint set that has a name, in file order. A set's name is
// its string key's text, or else the last `\NAME\` of the schematic reference its 0x03 field holds.
// A set with neither is left out rather than given an invented name.
//
// Each set holds one 56-byte record per copper layer. The minimum line width is the record's second
// word in 17.2 and its first in 16.6 (KiCad's format notes). The other words are not read yet; the
// clearance and differential-pair gap are tracked in agni issue 1359's follow-up.
func (p *board) constraintSets() []conSet {
	var offs []int
	for _, off := range p.keys {
		if p.b[off] == 0x1D {
			offs = append(offs, off)
		}
	}
	sort.Ints(offs)
	widthWord := 4
	if p.l.release == "16.6" {
		widthWord = 0
	}
	var out []conSet
	for _, off := range offs {
		key := p.u32(off, 12)
		name := p.str(key)
		if name == "" {
			name = p.constraintSetFieldName(p.u32(off, 16))
		}
		if name == "" {
			continue
		}
		s := conSet{name: name, key: key}
		for i := 0; i < int(le16(p.b, off+22)); i++ {
			s.widths = append(s.widths, p.nm(float64(p.i32(off+24+56*i, widthWord))))
		}
		out = append(out, s)
	}
	return out
}

// constraintSetFieldName reads a set's name from the schematic reference its 0x03 field holds, such
// as `@lib.glbl(sch_1):\DEFAULT\`, or "".
func (p *board) constraintSetFieldName(field uint32) string {
	off, ok := p.at(field, tagField)
	if !ok || p.b[off+p.l.fieldSub] != 0x68 {
		return ""
	}
	size := int(le16(p.b, off+p.l.fieldLen))
	body := off + p.l.fieldBody
	if body+size > len(p.b) {
		return ""
	}
	text := strings.TrimRight(cstring(p.b[body:body+size]), "\\")
	if i := strings.LastIndex(text, "\\"); i >= 0 {
		return text[i+1:]
	}
	return ""
}

// netConstraintSet is the string key of the constraint set a net takes, and whether it takes one. A
// net's own field names it. A net with none takes the set its match group names, which is where
// Allegro puts a differential pair's set: on the BeagleBone AI, 30 of the 158 fields naming a set sit
// on match-group tables rather than on nets.
func (p *board) netConstraintSet(net int) (uint32, bool) {
	if key, ok := p.constraintSetField(p.u32(net, p.l.netAssign+8), p.u32(net, 4)); ok {
		return key, true
	}
	table, ok := p.matchGroupTable(p.u32(net, p.l.netAssign+12))
	if !ok {
		return 0, false
	}
	return p.constraintSetField(p.u32(table, p.l.tableFields), p.u32(table, 4))
}

// matchGroupTable is the table (0x2C) a net's match group pointer reaches. A 17.2 net points at a
// 0x26, which names the table or a second 0x26 that does. A 16.6 net points at the table.
func (p *board) matchGroupTable(ptr uint32) (int, bool) {
	for range 3 {
		off, ok := p.at(ptr, tagMatchGroup, tagTable)
		if !ok {
			return 0, false
		}
		if p.b[off] == tagTable {
			return off, true
		}
		ptr = p.u32(off, p.l.mgGroup)
	}
	return 0, false
}

// constraintSetField is the string key that the constraint-set field on a chain of fields names. A
// board stores the key as a word (subtype 0x6A or 0x67).
func (p *board) constraintSetField(head, owner uint32) (uint32, bool) {
	var key uint32
	found := false
	p.chain(head, owner, next8, func(off int) {
		if found || p.b[off] != tagField || le16(p.b, off+2) != netFieldConstraintSet {
			return
		}
		if sub := p.b[off+p.l.fieldSub]; sub == 0x6A || sub == 0x67 {
			key, found = p.u32(off, p.l.fieldBody), true
		}
	})
	return key, found
}

// netClassDefs turns the constraint sets into net-class definitions (agni issue 1359). A class's
// track_width is its narrowest minimum on any copper layer, in millimetres as the KiCad project
// reader writes it. Where layers differ, each layer's own minimum is also kept as
// `track_width@<layer>`. The set named DEFAULT is the class a net with no set of its own takes.
func netClassDefs(sets []conSet, layers []copperLayer) []*ir.Constraint {
	var out []*ir.Constraint
	for _, s := range sets {
		params := map[string]string{}
		if strings.EqualFold(s.name, "DEFAULT") {
			params["is_default"] = "true"
		}
		var least int64
		differ := false
		for i, w := range s.widths {
			if i > 0 && w != s.widths[0] {
				differ = true
			}
			if w > 0 && (least == 0 || w < least) {
				least = w
			}
		}
		if least > 0 {
			params["track_width"] = mm(least)
		}
		if differ && len(s.widths) == len(layers) {
			for i, w := range s.widths {
				if w > 0 {
					params["track_width@"+layers[i].name] = mm(w)
				}
			}
		}
		out = append(out, &ir.Constraint{Name: s.name, Kind: constraintKindNetClass, Params: params})
	}
	return out
}

func mm(nm int64) string { return strconv.FormatFloat(float64(nm)/1e6, 'g', -1, 64) }

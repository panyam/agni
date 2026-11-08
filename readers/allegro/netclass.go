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

// constraintKindBoardRules is the ir.Constraint kind that carries a board's own copper minimums, as
// the KiCad project reader writes it, so the copper rules hold the board to them.
const constraintKindBoardRules = "board_rules"

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

// boardRules returns the board's minimum track width as a board_rules constraint, or nil when no
// named set gives a width. Allegro checks each net against its own set, so the narrowest width any
// set allows is the narrowest track the board's own rules accept anywhere. That is what KiCad's
// min_track_width means. Clearance, drill and annular ring are not read, so their rules keep their
// fixed floors (agni issue 1432).
func boardRules(sets []conSet) *ir.Constraint {
	var least int64
	for _, s := range sets {
		for _, w := range s.widths {
			if w > 0 && (least == 0 || w < least) {
				least = w
			}
		}
	}
	if least == 0 {
		return nil
	}
	return &ir.Constraint{Name: "board", Kind: constraintKindBoardRules, Params: map[string]string{"min_track_width": mm(least)}}
}

func mm(nm int64) string { return strconv.FormatFloat(float64(nm)/1e6, 'g', -1, 64) }

// The kinds a match-group table (0x2C) declares, read from its third byte. On the BeagleBone AI the
// two-net tables named for a pair (USB1_D, DDR1_DQS0) say 0x03 and the DDR byte lanes say 0x06. A
// net's own constraint table says 0x05 or 0x0C, and declares no group.
const (
	tableKindDiffPair   = 0x03
	tableKindMatchGroup = 0x06
)

// netGroups reads the differential pairs and match groups the board declares (agni issue 1360). The
// nets that reach the same pair or group table are its members, and the table's name is a string key
// in its header. A net can sit in a pair and in a match group at once, as a DDR strobe pair sits in
// its byte lane.
func (p *board) netGroups(src string) []*ir.NetGroup {
	byTable := map[int]*ir.NetGroup{}
	var order []int
	p.list(listNets, func(net int) {
		name := p.str(p.u32(net, 12))
		if name == "" {
			return
		}
		for _, tbl := range p.groupTables(net) {
			g := byTable[tbl]
			if g == nil {
				var kind ir.NetGroupKind
				switch p.b[tbl+2] {
				case tableKindDiffPair:
					kind = ir.NetGroupKind_NET_GROUP_KIND_DIFF_PAIR
				case tableKindMatchGroup:
					kind = ir.NetGroupKind_NET_GROUP_KIND_MATCH_GROUP
				default:
					continue
				}
				g = &ir.NetGroup{Name: p.str(p.u32(tbl, p.l.tableName)), Kind: kind, Prov: &ir.Provenance{SourceFile: src}}
				byTable[tbl] = g
				order = append(order, tbl)
			}
			g.Nets = append(g.Nets, name)
		}
	})
	var out []*ir.NetGroup
	for _, tbl := range order {
		g := byTable[tbl]
		// A pair names two nets. A table that reaches some other count is not read as one.
		if g.Kind == ir.NetGroupKind_NET_GROUP_KIND_DIFF_PAIR && len(g.Nets) != 2 {
			continue
		}
		sort.Strings(g.Nets)
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// groupTables lists every table (0x2C) a net's match-group pointer reaches. A net points at a chain
// of 0x26s, one per table it sits in, each naming its table and the net's next 0x26 at +8. A 17.2
// 0x26 names the net at +12. A 16.6 chain instead ends at a 0x26 whose +8 names the net, so a 16.6
// strobe pair in its byte lane reads in both (agni issue 1437, on the BeagleBoard-X15).
func (p *board) groupTables(net int) []int {
	var out []int
	seen := map[int]bool{}
	netKey := p.u32(net, 4)
	ptr := p.u32(net, p.l.netAssign+12)
	for range 8 {
		off, ok := p.at(ptr, tagMatchGroup, tagTable)
		if !ok || seen[off] {
			break
		}
		seen[off] = true
		if p.b[off] == tagTable {
			out = append(out, off)
			break
		}
		for _, w := range []int{8, 12, 16} {
			if t, ok := p.at(p.u32(off, w), tagTable); ok && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
		next, ok := p.at(p.u32(off, 8), tagMatchGroup)
		if !ok || (p.u32(next, 12) != netKey && !p.chainEndsAt(next, netKey)) {
			break
		}
		ptr = p.u32(off, 8)
	}
	return out
}

// chainEndsAt reports whether following +8 from a 0x26 through further 0x26s reaches one whose +8
// names the net, which is how a 16.6 chain ends.
func (p *board) chainEndsAt(off int, netKey uint32) bool {
	for range 8 {
		if p.u32(off, 8) == netKey {
			return true
		}
		next, ok := p.at(p.u32(off, 8), tagMatchGroup)
		if !ok {
			return false
		}
		off = next
	}
	return false
}

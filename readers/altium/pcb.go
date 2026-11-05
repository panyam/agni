package altium

import (
	"fmt"
	"strconv"
	"strings"
)

const none16 = 0xFFFF // a net, polygon or component index meaning none

// V6 layer ids for the layers the reader names.
const (
	layerTop        = 1
	layerBottom     = 32
	layerTopOverlay = 33
	layerBotOverlay = 34
	layerMulti      = 74
)

// pcb is a .PcbDoc decoded into the records the reader uses. Every coordinate is in internal units
// of 0.1 µinch (2.54 nm), absolute, Y up.
type pcb struct {
	board    props
	nets     []string
	comps    []comp
	pads     []pad
	vias     []via
	tracks   []track
	arcs     []arc
	texts    []text
	polygons []props
	stack    []int // copper layer ids, top to bottom
}

type comp struct {
	p          props
	ref        string // the board designator
	comment    string
	params     map[string]string // the base design's schematic parameters
	namedPads  int
	mechanical bool // a mounting hole, fiducial or other non-electrical part
	graphical  bool // a drawing such as a layer marker, whose copper Altium's export leaves out
}

type pad struct {
	name      string
	comp, net int
	layer     byte
	x, y      int64
	w, h      int64 // top copper size
	shape     byte  // 1 round, 2 rectangle, 3 octagon
	roundRect bool
	hole      int64
	rot       float64
}

type via struct {
	net       int
	x, y      int64
	dia, hole int64
	from, to  byte
}

type track struct {
	layer          byte
	net, comp      int
	keepout        bool
	poured         bool // drawn by a polygon pour, or its outline
	x1, y1, x2, y2 int64
	width          int64
}

type arc struct {
	layer      byte
	net, comp  int
	keepout    bool
	poured     bool
	cx, cy, r  int64
	start, end float64 // degrees, counter-clockwise from start to end
	width      int64
}

type text struct {
	layer      byte
	comp       int
	x, y       int64
	height     int64
	rot        float64
	mirror     bool
	comment    bool
	designator bool
	s          string
}

func index16(v uint16) int {
	if v == none16 {
		return -1
	}
	return int(v)
}

func decode(c *cfb) (*pcb, error) {
	p := &pcb{}
	read := func(name string, required bool) ([]byte, error) {
		b, ok, err := c.stream(name + "/Data")
		if err != nil {
			return nil, err
		}
		if !ok && required {
			return nil, fmt.Errorf("no %s stream, so this is not a binary Altium board", name)
		}
		return b, nil
	}
	b, err := read("Board6", true)
	if err != nil {
		return nil, err
	}
	boards, err := parseProps(b)
	if err != nil || len(boards) == 0 {
		return nil, fmt.Errorf("Board6: %v", err)
	}
	p.board = boards[0]
	p.stack = copperStack(p.board)

	if b, err = read("Nets6", true); err != nil {
		return nil, err
	}
	nets, err := parseProps(b)
	if err != nil {
		return nil, fmt.Errorf("Nets6: %w", err)
	}
	for _, n := range nets {
		p.nets = append(p.nets, n["NAME"])
	}

	if b, err = read("Components6", true); err != nil {
		return nil, err
	}
	cps, err := parseProps(b)
	if err != nil {
		return nil, fmt.Errorf("Components6: %w", err)
	}
	for _, cp := range cps {
		kind, _ := cp.int("COMPONENTKIND")
		p.comps = append(p.comps, comp{p: cp, mechanical: kind == 1 || kind == 2, graphical: kind == 2})
	}

	if b, err = read("Polygons6", false); err != nil {
		return nil, err
	}
	if p.polygons, err = parseProps(b); err != nil {
		return nil, fmt.Errorf("Polygons6: %w", err)
	}

	if err := p.readPads(read); err != nil {
		return nil, err
	}
	if err := p.readCopper(read); err != nil {
		return nil, err
	}
	if err := p.readTexts(read); err != nil {
		return nil, err
	}
	if err := p.readParameters(read); err != nil {
		return nil, err
	}
	for i := range p.comps {
		if p.comps[i].ref == "" {
			p.comps[i].ref = p.comps[i].p["SOURCEDESIGNATOR"]
		}
	}
	return p, nil
}

// copperStack follows Board6's LAYER<n>NEXT chain from the top copper layer. Layer ids do not give
// physical order: a board can run 1, 2, 4, 3, 5, 32, with MID3 above MID2.
func copperStack(b props) []int {
	stack := []int{layerTop}
	seen := map[int]bool{layerTop: true}
	for cur := layerTop; ; {
		next, ok := b.int("LAYER" + strconv.Itoa(cur) + "NEXT")
		if !ok || next == 0 || seen[next] || next > 82 {
			break
		}
		stack = append(stack, next)
		seen[next] = true
		cur = next
	}
	if stack[len(stack)-1] != layerBottom {
		stack = append(stack, layerBottom)
	}
	return stack
}

type reader func(name string, required bool) ([]byte, error)

func (p *pcb) readPads(read reader) error {
	b, err := read("Pads6", false)
	if err != nil {
		return err
	}
	recs, err := parseRecords(b, 0x02, 6)
	if err != nil {
		return fmt.Errorf("Pads6: %w", err)
	}
	for _, r := range recs {
		var name string
		if s := r.subs[0]; len(s) > 0 && int(s[0]) <= len(s)-1 {
			name = strings.TrimSpace(latin1(s[1 : 1+int(s[0])]))
		}
		g := field(r.subs[4])
		pd := pad{
			name:  name,
			layer: g.u8(0),
			net:   index16(g.u16(3)),
			comp:  index16(g.u16(7)),
			x:     int64(g.i32(13)), y: int64(g.i32(17)),
			w: int64(g.i32(21)), h: int64(g.i32(25)),
			hole:  int64(g.i32(45)),
			shape: g.u8(49),
			rot:   g.f64(52),
		}
		if st := field(r.subs[5]); len(st) >= 596 && st.u8(532) == 9 {
			pd.roundRect = true
		}
		if pd.comp >= 0 && pd.comp < len(p.comps) && pd.name != "" {
			p.comps[pd.comp].namedPads++
		}
		p.pads = append(p.pads, pd)
	}
	return nil
}

func (p *pcb) readCopper(read reader) error {
	b, err := read("Vias6", false)
	if err != nil {
		return err
	}
	recs, err := parseRecords(b, 0x03, 1)
	if err != nil {
		return fmt.Errorf("Vias6: %w", err)
	}
	for _, r := range recs {
		g := field(r.subs[0])
		p.vias = append(p.vias, via{
			net: index16(g.u16(3)), x: int64(g.i32(13)), y: int64(g.i32(17)),
			dia: int64(g.i32(21)), hole: int64(g.i32(25)), from: g.u8(29), to: g.u8(30),
		})
	}

	if b, err = read("Tracks6", false); err != nil {
		return err
	}
	if recs, err = parseRecords(b, 0x04, 1); err != nil {
		return fmt.Errorf("Tracks6: %w", err)
	}
	for _, r := range recs {
		g := field(r.subs[0])
		p.tracks = append(p.tracks, track{
			layer: g.u8(0), keepout: g.u8(2) == 2, poured: index16(g.u16(5)) >= 0, net: p.primitiveNet(g), comp: index16(g.u16(7)),
			x1: int64(g.i32(13)), y1: int64(g.i32(17)), x2: int64(g.i32(21)), y2: int64(g.i32(25)),
			width: int64(g.i32(29)),
		})
	}

	if b, err = read("Arcs6", false); err != nil {
		return err
	}
	if recs, err = parseRecords(b, 0x01, 1); err != nil {
		return fmt.Errorf("Arcs6: %w", err)
	}
	for _, r := range recs {
		g := field(r.subs[0])
		p.arcs = append(p.arcs, arc{
			layer: g.u8(0), keepout: g.u8(2) == 2, poured: index16(g.u16(5)) >= 0, net: p.primitiveNet(g), comp: index16(g.u16(7)),
			cx: int64(g.i32(13)), cy: int64(g.i32(17)), r: int64(g.i32(21)),
			start: g.f64(25), end: g.f64(33), width: int64(g.i32(41)),
		})
	}
	return nil
}

// primitiveNet is a primitive's net index. Copper a polygon pour made carries no net of its own and
// names its polygon, whose NET property holds the net.
func (p *pcb) primitiveNet(g field) int {
	if n := index16(g.u16(3)); n >= 0 {
		return n
	}
	poly := index16(g.u16(5))
	if poly < 0 || poly >= len(p.polygons) {
		return -1
	}
	if n, ok := p.polygons[poly].int("NET"); ok {
		return n
	}
	return -1
}

func (p *pcb) readTexts(read reader) error {
	ws := map[uint32]string{}
	if b, err := read("WideStrings6", false); err != nil {
		return err
	} else if len(b) > 0 {
		if ws, err = parseWideStrings(b); err != nil {
			return fmt.Errorf("WideStrings6: %w", err)
		}
	}
	b, err := read("Texts6", false)
	if err != nil {
		return err
	}
	recs, err := parseRecords(b, 0x05, 2)
	if err != nil {
		return fmt.Errorf("Texts6: %w", err)
	}
	for _, r := range recs {
		g := field(r.subs[0])
		t := text{
			layer: g.u8(0), comp: index16(g.u16(7)),
			x: int64(g.i32(13)), y: int64(g.i32(17)), height: int64(g.i32(21)),
			rot: g.f64(27), mirror: g.u8(35) != 0,
		}
		if len(g) >= 123 {
			t.comment, t.designator = g.u8(40) != 0, g.u8(41) != 0
		}
		if s, ok := ws[g.u32(115)]; ok && len(g) >= 119 {
			t.s = s
		} else if s := r.subs[1]; len(s) > 0 && int(s[0]) <= len(s)-1 {
			t.s = latin1(s[1 : 1+int(s[0])])
		}
		t.s = strings.ReplaceAll(t.s, "\r\n", "\n")
		if t.comp >= 0 && t.comp < len(p.comps) {
			c := &p.comps[t.comp]
			switch {
			case t.designator && c.ref == "":
				c.ref = strings.TrimSpace(t.s)
			case t.comment && c.comment == "":
				c.comment = strings.TrimSpace(t.s)
			}
		}
		p.texts = append(p.texts, t)
	}
	return nil
}

// readParameters reads PrimitiveParameters: groups of a header naming a component's UNIQUEID and a
// variant, then COUNT parameter lists. Only the base design's group (no variant) is kept.
func (p *pcb) readParameters(read reader) error {
	b, err := read("PrimitiveParameters", false)
	if err != nil || len(b) == 0 {
		return err
	}
	lists, err := parseProps(b)
	if err != nil {
		return fmt.Errorf("PrimitiveParameters: %w", err)
	}
	byID := map[string]int{}
	for i, c := range p.comps {
		if id := c.p["UNIQUEID"]; id != "" {
			byID[id] = i
		}
	}
	for i := 0; i < len(lists); {
		h := lists[i]
		n, ok := h.int("COUNT")
		if !ok || n < 0 {
			i++
			continue
		}
		ci, found := byID[h["PRIMITIVEID"]]
		base := h["VARIANTGUID"] == ""
		for j := i + 1; j <= i+n && j < len(lists); j++ {
			if found && base && lists[j]["VALUE"] != "" {
				if p.comps[ci].params == nil {
					p.comps[ci].params = map[string]string{}
				}
				p.comps[ci].params[lists[j]["NAME"]] = lists[j]["VALUE"]
			}
		}
		i += n + 1
	}
	return nil
}

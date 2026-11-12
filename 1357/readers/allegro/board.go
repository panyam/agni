package allegro

import (
	"fmt"
	"math"
	"slices"

	"sort"

	"github.com/panyam/agni/internal/boardlayer"
	"github.com/panyam/agni/internal/geomath"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// Layer classes. A block's layer is a (class, subclass) byte pair. For ETCH and the classes that
// share its layer list, the subclass is the copper index, 0 the top.
const (
	classBoardGeom = 0x01
	classCompValue = 0x02
	classDrawing   = 0x04
	classEtch      = 0x06
	classMfg       = 0x07
	classPkgGeom   = 0x09
	classRefDes    = 0x0D
	classBoundary  = 0x15
)

// Fixed subclasses, the codes at or above 0xEA that name the same layer in every board.
const (
	subOutline     = 0xEA // board geometry outline
	subFmtOutline  = 0xFD // drawing format outline
	subAutosilkBot = 0xF3
	subAutosilkTop = 0xF4
	subSilkBot     = 0xF6 // package geometry
	subSilkTop     = 0xF7
	subTextSilkBot = 0xFA // ref-des, value and other text classes
	subTextSilkTop = 0xFB
	subAssemblyBot = 0xFC
	subAssemblyTop = 0xFD
)

// copperLayer is one ETCH layer, named in the contract's KiCad-style vocabulary.
type copperLayer struct {
	name   string // F.Cu, In1.Cu, ..., B.Cu
	source string // the board's own name, e.g. TOP, LAY2_GND
	plane  bool
}

// copperLayers reads the ETCH class's layer list from the header's layer map, in stackup order.
func (p *board) copperLayers() []copperLayer {
	key := le32(p.b, hdrLayerMap+8*classEtch+4)
	off, ok := p.at(key, tagLayerList)
	if !ok {
		return nil
	}
	n := int(le16(p.b, off+2))
	out := make([]copperLayer, 0, n)
	for i := 0; i < n; i++ {
		e := off + 4 + 12*i
		out = append(out, copperLayer{
			name:   boardlayer.CopperName(i, n),
			source: p.str(le32(p.b, e)),
			plane:  le32(p.b, e+4)&0x100 != 0,
		})
	}
	return out
}

// artLayer names the silkscreen or fabrication layer a fixed subclass draws on, or "" for one the
// board geometry does not keep. Ref-des and value text use the text subclasses, package outlines the
// package geometry ones.
func artLayer(class, sub byte) string {
	switch class {
	case classPkgGeom:
		switch sub {
		case subSilkTop:
			return "F.SilkS"
		case subSilkBot:
			return "B.SilkS"
		case subAssemblyTop:
			return "F.Fab"
		case subAssemblyBot:
			return "B.Fab"
		}
	case classMfg:
		switch sub {
		case subAutosilkTop:
			return "F.SilkS"
		case subAutosilkBot:
			return "B.SilkS"
		}
	case classCompValue:
		// Value text sits on a silkscreen subclass that a board's silk film usually leaves out,
		// and films are not read, so value text is kept with the assembly drawing.
		switch sub {
		case subTextSilkTop, subAssemblyTop:
			return "F.Fab"
		case subTextSilkBot, subAssemblyBot:
			return "B.Fab"
		}
	case classRefDes:
		switch sub {
		case subTextSilkTop:
			return "F.SilkS"
		case subTextSilkBot:
			return "B.SilkS"
		case subAssemblyTop:
			return "F.Fab"
		case subAssemblyBot:
			return "B.Fab"
		}
	}
	return ""
}

func (p *board) toBoard(src string) (*geom.BoardGeometry, error) {
	layers := p.copperLayers()
	if len(layers) == 0 {
		return nil, fmt.Errorf("allegro: the board names no copper layers")
	}
	g := &geom.BoardGeometry{DesignRef: designName(src), UnitNm: 1, Prov: &geom.Provenance{SourceFile: src}}
	for i, l := range layers {
		kind := "signal"
		if l.plane {
			kind = "power"
		}
		g.Layers = append(g.Layers, &geom.BoardLayer{Number: int32(i), Name: l.name, Kind: kind}) // #nosec G115 -- a layer index fits a subclass byte
	}
	b := &boardBuilder{p: p, g: g, layers: layers, src: src, stacks: map[stackKey]string{}}
	b.outline()
	b.placements()
	b.copper()
	b.zones()
	b.boardArt()
	return g, nil
}

type boardBuilder struct {
	p      *board
	g      *geom.BoardGeometry
	layers []copperLayer
	src    string
	// stacks holds the id of each padstack entry written, by its 0x1C and side, so a padstack
	// shared by many pads is written once per side.
	stacks map[stackKey]string
}

type stackKey struct {
	off    int
	bottom bool
}

func (b *boardBuilder) copperLayer(sub byte) string {
	if int(sub) >= len(b.layers) {
		return ""
	}
	return b.layers[sub].name
}

// pt converts a file coordinate pair to a board point in nanometres.
func (b *boardBuilder) pt(x, y int32) *geom.Point {
	return &geom.Point{X: b.p.nm(float64(x)), Y: b.p.nm(float64(y))}
}

// outline is the board edge: a shape of class board geometry with the outline subclass, or a
// drawing-format outline when the board draws none.
func (b *boardBuilder) outline() {
	p := b.p
	found := false
	visit := func(off int) {
		tag, class, sub := p.b[off], p.b[off+2], p.b[off+3]
		if !(class == classBoardGeom && sub == subOutline) && !(class == classDrawing && sub == subFmtOutline) && !(class == classBoardGeom && sub == subFmtOutline) {
			return
		}
		var head uint32
		switch tag {
		case tagShape:
			head = p.u32(off, p.l.shapeSegs)
		case tagGraphic:
			head = p.u32(off, p.l.graphSegs)
		default:
			return
		}
		if path := b.path(head, p.u32(off, 4)); len(path) > 1 {
			b.g.Outline = appendOutline(b.g.Outline, path)
			found = true
		}
	}
	p.list(listRects, visit)
	if !found {
		p.list(listGraphics, visit)
	}
	if !found {
		b.symbolOutline()
	}
}

// symbolOutline is the board edge drawn as a rectangle inside a placed mechanical symbol, a
// footprint with no designator. Clay R6 draws its edge that way, as the design outline of its
// CLAY_R6_BOARD_2X2 symbol. A placed symbol's areas are board-absolute. The rectangles are taken in
// file order, so the outline reads the same every time.
func (b *boardBuilder) symbolOutline() {
	p := b.p
	var rects []int
	for _, off := range p.keys {
		if p.b[off] != tagRect || p.b[off+2] != classBoardGeom || (p.b[off+3] != subOutline && p.b[off+3] != subFmtOutline) {
			continue
		}
		if owner, ok := p.at(p.u32(off, 12), tagFootInst); ok {
			if _, designated := p.at(p.u32(owner, p.l.fiInstRef), tagCompInst); !designated {
				rects = append(rects, off)
			}
		}
	}
	sort.Ints(rects)
	for _, off := range rects {
		c := p.l.rectCoords
		x0, y0, x1, y1 := p.i32(off, c), p.i32(off, c+4), p.i32(off, c+8), p.i32(off, c+12)
		b.g.Outline = appendOutline(b.g.Outline, []*geom.Point{b.pt(x0, y0), b.pt(x1, y0), b.pt(x1, y1), b.pt(x0, y1), b.pt(x0, y0)})
	}
}

func appendOutline(o *geom.BoardOutline, path []*geom.Point) *geom.BoardOutline {
	if o == nil {
		o = &geom.BoardOutline{}
	}
	o.Paths = append(o.Paths, &geom.Polyline{Points: path})
	return o
}

// path follows a segment chain (lines and arcs) from head to the owner's key and returns it as one
// polyline, arcs approximated by chords. Consecutive segments share their end points, so each
// segment after the first adds the points past its start.
func (b *boardBuilder) path(head, owner uint32) []*geom.Point {
	var pts []*geom.Point
	b.segments(head, owner, func(seg []*geom.Point, _ int64) {
		if len(pts) > 0 && len(seg) > 0 && samePoint(pts[len(pts)-1], seg[0]) {
			seg = seg[1:]
		}
		pts = append(pts, seg...)
	})
	return pts
}

// segments visits each line and arc of a segment chain as a polyline in nanometres, with its width.
func (b *boardBuilder) segments(head, owner uint32, visit func(pts []*geom.Point, width int64)) {
	p := b.p
	p.chain(head, owner, next8, func(off int) {
		w := p.l.segWidth
		width := p.nm(float64(p.u32(off, w)))
		switch p.b[off] {
		case tagLineH, tagLineD, tagLineV:
			visit([]*geom.Point{b.pt(p.i32(off, w+4), p.i32(off, w+8)), b.pt(p.i32(off, w+12), p.i32(off, w+16))}, width)
		case tagArc:
			visit(b.arc(off), width)
		}
	})
}

// arc approximates an arc block by chords no longer than a tenth of a radian. A start equal to the
// end is a full circle. Bit 6 of the subtype byte marks a clockwise sweep.
func (b *boardBuilder) arc(off int) []*geom.Point {
	p := b.p
	s, e, c := p.l.arcStart(), p.l.arcEnd(), p.l.arcCentre()
	sx, sy := float64(p.i32(off, s)), float64(p.i32(off, s+4))
	ex, ey := float64(p.i32(off, e)), float64(p.i32(off, e+4))
	cx, cy, r := p.float(off, c), p.float(off, c+8), p.float(off, c+16)
	start := math.Atan2(sy-cy, sx-cx)
	end := math.Atan2(ey-cy, ex-cx)
	cw := p.b[off+3]&0x40 != 0
	sweep := end - start
	switch {
	case sx == ex && sy == ey:
		sweep = 2 * math.Pi
		if cw {
			sweep = -sweep
		}
	case cw && sweep > 0:
		sweep -= 2 * math.Pi
	case !cw && sweep < 0:
		sweep += 2 * math.Pi
	}
	if math.IsNaN(sweep) || math.IsNaN(r) || r <= 0 {
		return []*geom.Point{b.pt(int32(sx), int32(sy)), b.pt(int32(ex), int32(ey))}
	}
	n := int(math.Ceil(math.Abs(sweep) / 0.1))
	if n < 1 {
		n = 1
	}
	pts := make([]*geom.Point, 0, n+1)
	pts = append(pts, &geom.Point{X: p.nm(sx), Y: p.nm(sy)})
	for i := 1; i < n; i++ {
		a := start + sweep*float64(i)/float64(n)
		pts = append(pts, &geom.Point{X: p.nm(cx + r*math.Cos(a)), Y: p.nm(cy + r*math.Sin(a))})
	}
	return append(pts, &geom.Point{X: p.nm(ex), Y: p.nm(ey)})
}

func samePoint(a, b *geom.Point) bool { return a.GetX() == b.GetX() && a.GetY() == b.GetY() }

// placements emits each placed footprint with a designator and its pads. Pads are footprint-local in
// the file and stay so here. Allegro mirrors a bottom footprint's X and then rotates it
// counter-clockwise; the contract rotates and then mirrors, so a bottom footprint's angle is negated.
func (b *boardBuilder) placements() {
	p := b.p
	for _, pl := range p.placements() {
		rot := float64(p.u32(pl.inst, p.l.fiRot)) / 1000
		bottom := p.b[pl.inst+2] == 1
		side := "F.Cu"
		if bottom {
			side = "B.Cu"
			rot = -rot
		}
		g := &geom.ComponentPlacement{
			RefDes:      pl.ref,
			At:          b.pt(p.i32(pl.inst, p.l.fiX), p.i32(pl.inst, p.l.fiX+4)),
			RotationDeg: geomath.NormDeg(rot),
			Layer:       side,
			Mirror:      bottom,
			Prov:        &geom.Provenance{SourceFile: b.src, SourceId: pl.ref},
		}
		p.chain(p.u32(pl.inst, p.l.fiPads), p.u32(pl.inst, 4), func(byte) int { return p.l.ppNextInFp }, func(pp int) {
			if p.b[pp] != tagPlacedPad {
				return
			}
			if pad := b.pad(pp, bottom); pad != nil {
				g.Pads = append(g.Pads, pad)
			}
		})
		b.g.Placements = append(b.g.Placements, g)
		b.footprintArt(pl)
	}
}

// pad builds one placed pad from the footprint pad it copies and that pad's padstack.
func (b *boardBuilder) pad(placedPad int, bottom bool) *geom.Pad {
	p := b.p
	fp, ok := p.at(p.u32(placedPad, p.l.ppPad), tagPad)
	if !ok {
		return nil
	}
	pad := &geom.Pad{
		Number:      p.str(p.u32(fp, 8)),
		At:          b.pt(p.i32(fp, 16), p.i32(fp, 20)),
		RotationDeg: geomath.NormDeg(float64(p.u32(fp, p.l.padRot)) / 1000),
		Net:         p.padNet(placedPad),
	}
	ps, ok := p.at(p.u32(fp, 24), tagPadstack)
	if !ok {
		return pad
	}
	st := b.padstack(ps)
	pad.Shape, pad.Size, pad.Drill = st.shape, st.size, st.drill
	pad.Padstack = b.stackEntry(ps, bottom)
	switch {
	case st.count > 1:
		pad.Layers = []string{"*.Cu"}
	default:
		idx := st.start
		if bottom {
			idx = len(b.layers) - 1 - st.start
		}
		if name := b.copperLayer(byte(idx)); name != "" { // #nosec G115 -- bounded by the layer count
			pad.Layers = []string{name}
		}
	}
	return pad
}

// stack is what a pad or via takes from its padstack: the land on its first copper layer, the hole,
// and the copper span.
type stack struct {
	shape        string
	size         *geom.Point
	drill        int64
	start, count int
}

// padSlot is the pad's slot within a copper layer's components, after the antipad and the thermal
// relief.
const padSlot = 2

// padstack reads a 0x1C. Its components follow a header: fixed technical-layer slots (21 in 17.2,
// 11 in 16.6), then the slots of each copper layer (four in 17.2, three in 16.6). The pad of the
// first spanned layer is read.
func (b *boardBuilder) padstack(off int) stack {
	p := b.p
	l := p.l
	st := stack{start: int(p.b[off+3]), count: int(le16(p.b, off+l.psCount))}
	if st.count > 0 {
		c := off + l.psHeader + l.psComp*(l.psFixed+padSlot)
		st.shape = padShape(p.b[c])
		st.size = &geom.Point{X: p.nm(float64(p.i32(c, l.psW))), Y: p.nm(float64(p.i32(c, l.psW+4)))}
	}
	drill := p.u32(off, l.psDrill)
	if sx, sy := p.u32(off, l.psSlotX), p.u32(off, l.psSlotY); sy > 0 && sy < sx {
		drill = sy // a slot is drawn by its narrower dimension, the drill bit's
	}
	st.drill = p.nm(float64(drill))
	return st
}

// Slots within one copper layer's components, in the order a 0x1C holds them.
const (
	antipadSlot = 0
	thermalSlot = 1
)

// stackEntry writes the padstack at off into the board's padstack table, once per side, and
// returns the entry's id. Each spanned copper layer contributes its pad, thermal relief and antipad
// (agni 1356). A part on the bottom turns the padstack over, so its entry takes the layers in
// reverse and its id gains "@bottom".
func (b *boardBuilder) stackEntry(off int, bottom bool) string {
	key := stackKey{off, bottom}
	if id, ok := b.stacks[key]; ok {
		return id
	}
	p := b.p
	l := p.l
	name := p.str(p.u32(off, 12))
	id := name
	if id == "" {
		id = fmt.Sprintf("padstack@%x", off)
	}
	if bottom {
		id += "@bottom"
	}
	ps := &geom.Padstack{Id: id, Name: name}
	start, count := int(p.b[off+3]), int(le16(p.b, off+l.psCount))
	slot := func(i, k int) *geom.PadShape {
		c := off + l.psHeader + l.psComp*(l.psFixed+i*l.psPerLayer+k)
		shape := padShape(p.b[c])
		if shape == "" {
			return nil
		}
		return &geom.PadShape{Shape: shape, Size: &geom.Point{X: p.nm(float64(p.i32(c, l.psW))), Y: p.nm(float64(p.i32(c, l.psW+4)))}}
	}
	for i := 0; i < count; i++ {
		idx := start + i
		if bottom {
			idx = len(b.layers) - 1 - idx
		}
		if idx < 0 || idx >= len(b.layers) {
			continue
		}
		name := b.layers[idx].name
		ly := &geom.PadstackLayer{Layer: name, Pad: slot(i, padSlot), Thermal: slot(i, thermalSlot), Antipad: slot(i, antipadSlot)}
		if ly.Pad != nil || ly.Thermal != nil || ly.Antipad != nil {
			ps.Layers = append(ps.Layers, ly)
		}
	}
	if bottom {
		slices.SortStableFunc(ps.Layers, func(a, c *geom.PadstackLayer) int { return b.layerIndex(a.Layer) - b.layerIndex(c.Layer) })
	}
	b.g.Padstacks = append(b.g.Padstacks, ps)
	b.stacks[key] = id
	return id
}

func (b *boardBuilder) layerIndex(name string) int {
	for i, l := range b.layers {
		if l.name == name {
			return i
		}
	}
	return len(b.layers)
}

// padShape maps an Allegro pad shape code to the contract's shape word. A shape the renderer has no
// word for is drawn as a rectangle of its extent.
func padShape(code byte) string {
	switch code {
	case 0x02:
		return "circle"
	case 0x0B, 0x0C:
		return "oval"
	case 0x1B:
		return "roundrect"
	case 0x03:
		return "octagon"
	case 0x05, 0x06:
		return "rect"
	case 0x16:
		return "custom"
	case 0x00:
		return ""
	}
	return "rect"
}

// copper emits each named net's tracks and vias, read from the net's chain of connected items.
func (b *boardBuilder) copper() {
	p := b.p
	p.list(listNets, func(net int) {
		name := p.str(p.u32(net, 12))
		nc := &geom.NetCopper{Net: name}
		p.chain(p.u32(net, p.l.netAssign), p.u32(net, 4), next8, func(asg int) {
			if p.b[asg] != tagNetAssign {
				return
			}
			p.chain(p.u32(asg, 16), p.u32(asg, 4), next8, func(item int) {
				switch p.b[item] {
				case tagTrack:
					b.track(item, nc)
				case tagVia:
					b.via(item, nc)
				}
			})
		})
		if len(nc.Segments) > 0 || len(nc.Vias) > 0 {
			b.g.Nets = append(b.g.Nets, nc)
		}
	})
}

// track adds a track's segments. Only ETCH tracks are copper. Anti-etch tracks draw thermal-relief
// clearances and are dropped.
func (b *boardBuilder) track(off int, nc *geom.NetCopper) {
	p := b.p
	if p.b[off+2] != classEtch {
		return
	}
	layer := b.copperLayer(p.b[off+3])
	if layer == "" {
		return
	}
	p.chain(p.u32(off, p.l.trackSegs), p.u32(off, 4), next8, func(seg int) {
		w := p.l.segWidth
		width := p.nm(float64(p.u32(seg, w)))
		switch p.b[seg] {
		case tagLineH, tagLineD, tagLineV:
			nc.Segments = append(nc.Segments, &geom.TrackSegment{
				A: b.pt(p.i32(seg, w+4), p.i32(seg, w+8)), B: b.pt(p.i32(seg, w+12), p.i32(seg, w+16)), Width: width, Layer: layer,
			})
		case tagArc:
			for _, a := range b.arcPieces(seg) {
				nc.Segments = append(nc.Segments, &geom.TrackSegment{A: a[0], Mid: a[1], B: a[2], Width: width, Layer: layer})
			}
		}
	})
}

// arcPieces reads an arc block as the start, mid and end of each arc a TrackSegment holds: one arc,
// or two half circles for a full circle (geomath.CenterArc). An arc with no usable radius comes
// back as its chord with no mid, as arc() draws it.
func (b *boardBuilder) arcPieces(off int) [][3]*geom.Point {
	p := b.p
	s, e, c := p.l.arcStart(), p.l.arcEnd(), p.l.arcCentre()
	sx, sy := float64(p.i32(off, s)), float64(p.i32(off, s+4))
	ex, ey := float64(p.i32(off, e)), float64(p.i32(off, e+4))
	cx, cy, r := p.float(off, c), p.float(off, c+8), p.float(off, c+16)
	if math.IsNaN(cx) || math.IsNaN(cy) || math.IsNaN(r) || r <= 0 {
		return [][3]*geom.Point{{b.pt(int32(sx), int32(sy)), nil, b.pt(int32(ex), int32(ey))}}
	}
	at := func(q [2]float64) *geom.Point { return &geom.Point{X: p.nm(q[0]), Y: p.nm(q[1])} }
	var out [][3]*geom.Point
	for _, a := range geomath.CenterArc(sx, sy, ex, ey, cx, cy, p.b[off+3]&0x40 != 0) {
		out = append(out, [3]*geom.Point{at(a.S), at(a.M), at(a.E)})
	}
	return out
}

// via adds a via, sized by its padstack's land on the first spanned layer. A padstack spanning one
// layer, or every layer, is a through via.
func (b *boardBuilder) via(off int, nc *geom.NetCopper) {
	p := b.p
	v := &geom.Via{At: b.pt(p.i32(off, p.l.viaX), p.i32(off, p.l.viaX+4)), LayerFrom: "F.Cu", LayerTo: "B.Cu"}
	if ps, ok := p.at(p.u32(off, p.l.viaPadstack), tagPadstack); ok {
		st := b.padstack(ps)
		v.Drill = st.drill
		v.Padstack = b.stackEntry(ps, false)
		if st.size != nil {
			v.Size = st.size.GetX()
		}
		if st.count > 1 && st.count < len(b.layers) {
			v.LayerFrom = b.copperLayer(byte(st.start))              // #nosec G115 -- bounded by the layer count
			v.LayerTo = b.copperLayer(byte(st.start + st.count - 1)) // #nosec G115 -- bounded by the layer count
		}
	}
	nc.Vias = append(nc.Vias, v)
}

// zones emits each zone boundary: a shape of the boundary class whose subclass is the copper layer.
// Its net is reached through a table and a pointer array. The fill Allegro computed is not kept, as
// for every other board format (agni issue 1357).
func (b *boardBuilder) zones() {
	p := b.p
	p.list(listShapes, func(off int) {
		if p.b[off] != tagShape || p.b[off+2] != classBoundary {
			return
		}
		layer := b.copperLayer(p.b[off+3])
		if layer == "" {
			return
		}
		outline := b.path(p.u32(off, p.l.shapeSegs), p.u32(off, 4))
		if len(outline) < 3 {
			return
		}
		b.g.Zones = append(b.g.Zones, &geom.Zone{Net: b.zoneNet(off), Layer: layer, Outline: &geom.Polyline{Points: outline}})
	})
}

func (b *boardBuilder) zoneNet(shape int) string {
	p := b.p
	tbl, ok := p.at(p.u32(shape, p.l.shapeTable), tagTable)
	if !ok {
		return ""
	}
	arr, ok := p.at(p.u32(tbl, p.l.tablePtr1), tagPtrArray)
	if !ok {
		return ""
	}
	net, ok := p.at(p.u32(arr, 28), tagNet)
	if !ok {
		return ""
	}
	return p.str(p.u32(net, 12))
}

// footprintArt emits a placed footprint's silkscreen and fabrication graphics and its text. Both are
// stored board-absolute in the file, so they are not transformed.
func (b *boardBuilder) footprintArt(pl placed) {
	p := b.p
	instKey := p.u32(pl.inst, 4)
	p.chain(p.u32(pl.inst, p.l.fiGraphics), instKey, next8, func(off int) {
		if p.b[off] == tagGraphic {
			b.graphic(off, pl.ref)
		}
	})
	p.chain(p.u32(pl.inst, p.l.fiText), instKey, next8, func(off int) {
		if p.b[off] == tagText {
			b.text(off, pl.ref)
		}
	})
}

// boardArt emits board-level silkscreen graphics and text.
func (b *boardBuilder) boardArt() {
	p := b.p
	p.list(listGraphics, func(off int) {
		if p.b[off] == tagGraphic {
			b.graphic(off, "")
		}
	})
	p.list(listText, func(off int) {
		if p.b[off] == tagText && p.u32(off, p.l.textGroup) == 0 {
			b.text(off, "")
		}
	})
}

func (b *boardBuilder) graphic(off int, ref string) {
	p := b.p
	layer := artLayer(p.b[off+2], p.b[off+3])
	if layer == "" {
		return
	}
	b.segments(p.u32(off, p.l.graphSegs), p.u32(off, 4), func(pts []*geom.Point, width int64) {
		b.g.Graphics = append(b.g.Graphics, &geom.BoardGraphic{
			Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: pts},
			Layer: layer, RefDes: ref, Width: width,
		})
	})
}

func (b *boardBuilder) text(off int, ref string) {
	p := b.p
	class := p.b[off+2]
	layer := artLayer(class, p.b[off+3])
	if layer == "" {
		return
	}
	sg, ok := p.at(p.u32(off, p.l.textStr), tagStrGraphic)
	if !ok {
		return
	}
	n := int(le16(p.b, sg+22))
	if sg+24+n > len(p.b) {
		return
	}
	s := cstring(p.b[sg+24 : sg+24+n])
	if s == "" {
		return
	}
	kind := "user"
	switch class {
	case classRefDes:
		kind = "reference"
	case classCompValue:
		kind = "value"
	}
	if ref == "" {
		kind = "gr"
	}
	justify := ""
	switch p.b[off+22] {
	case 1:
		justify = "left"
	case 2:
		justify = "right"
	}
	b.g.Texts = append(b.g.Texts, &geom.BoardText{
		Text:        s,
		At:          b.pt(p.i32(off, p.l.textX), p.i32(off, p.l.textX+4)),
		RotationDeg: geomath.NormDeg(float64(p.u32(off, p.l.textRot)) / 1000),
		Height:      b.fontHeight(p.b[off+20]),
		Layer:       layer,
		Mirror:      p.b[off+23] == 1 || p.b[off+23] == 3,
		Justify:     justify,
		RefDes:      ref,
		Kind:        kind,
	})
}

// fontHeight is the character height of a 1-based font index, from the definition table holding the
// fonts (code 0x08), or 0 when the board defines none.
func (b *boardBuilder) fontHeight(idx byte) int64 {
	p := b.p
	h := int64(0)
	if idx == 0 {
		return 0
	}
	p.list(listTables, func(off int) {
		if h != 0 || p.b[off] != tagDefTable || le16(p.b, off+2) != 0x08 {
			return
		}
		if int(idx) > int(p.u32(off, p.l.defCount)) {
			return
		}
		item := off + p.l.defItems + p.l.defItemSize[0x08]*(int(idx)-1)
		h = p.nm(float64(p.i32(item, 8)))
	})
	return h
}

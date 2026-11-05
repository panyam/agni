package altium

import (
	"github.com/panyam/agni/internal/boardlayer"
	"github.com/panyam/agni/internal/geomath"
	"math"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

func itoa(i int) string { return strconv.Itoa(i) }

type boardBuilder struct {
	p      *pcb
	g      *geom.BoardGeometry
	ox, oy int64          // the board origin, internal units
	copper map[int]string // V6 layer id -> contract name
	byName map[string]int // Altium layer name (TOP, MID3, PLANE1) -> V6 id
	src    string
}

func (p *pcb) toBoard(src string) (*geom.BoardGeometry, error) {
	b := &boardBuilder{
		p: p, src: src,
		ox: p.board.length("ORIGINX"), oy: p.board.length("ORIGINY"),
		copper: map[int]string{}, byName: map[string]int{"TOP": layerTop, "BOTTOM": layerBottom},
		g: &geom.BoardGeometry{DesignRef: designName(src), UnitNm: 1, Prov: &geom.Provenance{SourceFile: src}},
	}
	for i := 1; i <= 30; i++ {
		b.byName["MID"+itoa(i)] = i + 1
	}
	for i := 1; i <= 16; i++ {
		b.byName["PLANE"+itoa(i)] = 38 + i
	}
	for i, id := range p.stack {
		name := boardlayer.CopperName(i, len(p.stack))
		b.copper[id] = name
		kind := "signal"
		if id >= 39 && id <= 54 {
			kind = "power"
		}
		b.g.Layers = append(b.g.Layers, &geom.BoardLayer{Number: int32(i), Name: name, Kind: kind}) // #nosec G115 -- at most 82 layer ids
	}
	if path := b.outline(p.board); len(path) > 1 {
		b.g.Outline = &geom.BoardOutline{Paths: []*geom.Polyline{{Points: path}}}
	}
	b.placements()
	b.copperItems()
	b.zones()
	b.art()
	return b.g, nil
}

// nm converts an internal unit (0.1 µinch, exactly 2.54 nm) to nanometres.
func nm(v int64) int64 { return v * 254 / 100 }

// pt converts an absolute position to the contract's frame: nanometres from the board's origin.
func (b *boardBuilder) pt(x, y int64) *geom.Point {
	return &geom.Point{X: nm(x - b.ox), Y: nm(y - b.oy)}
}

// outline reads a vertex list (Board6's outline, or a polygon's) as a closed path, arcs approximated
// by chords. KIND<i>=1 makes vertex i an arc of radius R about (CX, CY) from SA to EA degrees; the
// arc is walked in whichever direction meets the path where it already ends.
func (b *boardBuilder) outline(p props) []*geom.Point {
	var pts []*geom.Point
	add := func(x, y float64) {
		q := b.pt(int64(math.Round(x)), int64(math.Round(y)))
		if n := len(pts); n > 0 && pts[n-1].GetX() == q.GetX() && pts[n-1].GetY() == q.GetY() {
			return
		}
		pts = append(pts, q)
	}
	for i := 0; ; i++ {
		k := itoa(i)
		if _, ok := p["VX"+k]; !ok {
			break
		}
		vx, vy := float64(p.length("VX"+k)), float64(p.length("VY"+k))
		if kind, _ := p.int("KIND" + k); kind == 1 {
			cx, cy, r := float64(p.length("CX"+k)), float64(p.length("CY"+k)), float64(p.length("R"+k))
			sa, ea := p.float("SA"+k), p.float("EA"+k)
			arc := arcPoints(cx, cy, r, sa, ea)
			if n := len(pts); n > 0 && len(arc) > 0 {
				last := pts[n-1]
				a, z := b.pt(int64(arc[0][0]), int64(arc[0][1])), b.pt(int64(arc[len(arc)-1][0]), int64(arc[len(arc)-1][1]))
				if dist(last, z) < dist(last, a) {
					for l, h := 0, len(arc)-1; l < h; l, h = l+1, h-1 {
						arc[l], arc[h] = arc[h], arc[l]
					}
				}
			}
			for _, q := range arc {
				add(q[0], q[1])
			}
		}
		add(vx, vy)
	}
	if len(pts) > 2 {
		pts = append(pts, &geom.Point{X: pts[0].GetX(), Y: pts[0].GetY()})
	}
	return pts
}

func dist(a, b *geom.Point) float64 {
	return math.Hypot(float64(a.GetX()-b.GetX()), float64(a.GetY()-b.GetY()))
}

// arcPoints walks an arc counter-clockwise from sa to ea degrees in chords of a tenth of a radian.
func arcPoints(cx, cy, r, sa, ea float64) [][2]float64 {
	sweep := ea - sa
	for sweep <= 0 {
		sweep += 360
	}
	n := max(1, int(math.Ceil(sweep*math.Pi/180/0.1)))
	out := make([][2]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		a := (sa + sweep*float64(i)/float64(n)) * math.Pi / 180
		out = append(out, [2]float64{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	return out
}

// placements emits every component that owns a pad. Altium stores pads at board positions, and the
// contract keeps them footprint-local, so each pad's local position is solved from the placement:
// world = at + M(R(rotation) * local), with M mirroring X on a bottom placement. A bottom part's
// angle is negated, as the other board readers carry one.
func (b *boardBuilder) placements() {
	byComp := map[int][]pad{}
	for _, pd := range b.p.pads {
		if pd.comp >= 0 {
			byComp[pd.comp] = append(byComp[pd.comp], pd)
		}
	}
	for i := range b.p.comps {
		c := &b.p.comps[i]
		pads := byComp[i]
		if c.ref == "" || len(pads) == 0 {
			continue
		}
		bottom := strings.EqualFold(c.p["LAYER"], "BOTTOM")
		rot := math.Mod(c.p.float("ROTATION"), 360)
		side := "F.Cu"
		if bottom {
			side, rot = "B.Cu", -rot
		}
		rot = geomath.NormDeg(rot)
		cx, cy := c.p.length("X"), c.p.length("Y")
		pl := &geom.ComponentPlacement{
			RefDes: c.ref, At: b.pt(cx, cy), RotationDeg: rot, Layer: side, Mirror: bottom,
			Prov: &geom.Provenance{SourceFile: b.src, SourceId: c.ref},
		}
		s, co := math.Sincos(-rot * math.Pi / 180)
		for _, pd := range pads {
			dx, dy := float64(nm(pd.x-cx)), float64(nm(pd.y-cy))
			angle := pd.rot - rot
			if bottom {
				dx = -dx
				angle = -pd.rot - rot
			}
			gp := &geom.Pad{
				Number:      pd.name,
				At:          &geom.Point{X: int64(math.Round(dx*co - dy*s)), Y: int64(math.Round(dx*s + dy*co))},
				RotationDeg: geomath.NormDeg(angle),
				Size:        &geom.Point{X: nm(pd.w), Y: nm(pd.h)},
				Shape:       padShape(pd),
				Drill:       nm(pd.hole),
				Net:         b.netName(pd.net),
			}
			switch {
			case pd.layer == layerMulti:
				gp.Layers = []string{"*.Cu"}
			case b.copper[int(pd.layer)] != "":
				gp.Layers = []string{b.copper[int(pd.layer)]}
			}
			pl.Pads = append(pl.Pads, gp)
		}
		b.g.Placements = append(b.g.Placements, pl)
	}
}

func padShape(pd pad) string {
	switch {
	case pd.roundRect:
		return "roundrect"
	case pd.shape == 1 && pd.w == pd.h:
		return "circle"
	case pd.shape == 1:
		return "oval"
	case pd.shape == 3:
		return "octagon"
	}
	return "rect"
}

// graphical reports whether a primitive belongs to a graphical component, such as a layer marker.
// Altium's export leaves such a component out, its copper included, and so does this reader.
func (b *boardBuilder) graphical(comp int) bool {
	return comp >= 0 && comp < len(b.p.comps) && b.p.comps[comp].graphical
}

func (b *boardBuilder) netName(i int) string {
	if i < 0 || i >= len(b.p.nets) {
		return ""
	}
	return b.p.nets[i]
}

// copperItems groups each net's tracks, arcs and vias. Keepouts are not copper. The tracks and arcs a
// polygon pour drew are left out with the pour's copper (agni issue 1357), and so is a graphical
// component's copper. Arcs are chorded.
func (b *boardBuilder) copperItems() {
	byNet := map[string]*geom.NetCopper{}
	var order []string
	get := func(n string) *geom.NetCopper {
		nc := byNet[n]
		if nc == nil {
			nc = &geom.NetCopper{Net: n}
			byNet[n] = nc
			order = append(order, n)
		}
		return nc
	}
	for _, t := range b.p.tracks {
		layer := b.copper[int(t.layer)]
		if t.keepout || t.poured || layer == "" || b.graphical(t.comp) {
			continue
		}
		nc := get(b.netName(t.net))
		nc.Segments = append(nc.Segments, &geom.TrackSegment{A: b.pt(t.x1, t.y1), B: b.pt(t.x2, t.y2), Width: nm(t.width), Layer: layer})
	}
	for _, a := range b.p.arcs {
		layer := b.copper[int(a.layer)]
		if a.keepout || a.poured || layer == "" || b.graphical(a.comp) {
			continue
		}
		nc := get(b.netName(a.net))
		pts := arcPoints(float64(a.cx), float64(a.cy), float64(a.r), a.start, a.end)
		for i := 1; i < len(pts); i++ {
			nc.Segments = append(nc.Segments, &geom.TrackSegment{
				A:     b.pt(int64(math.Round(pts[i-1][0])), int64(math.Round(pts[i-1][1]))),
				B:     b.pt(int64(math.Round(pts[i][0])), int64(math.Round(pts[i][1]))),
				Width: nm(a.width), Layer: layer,
			})
		}
	}
	for _, v := range b.p.vias {
		from, to := b.copper[int(v.from)], b.copper[int(v.to)]
		if from == "" || to == "" {
			from, to = "F.Cu", "B.Cu"
		}
		nc := get(b.netName(v.net))
		nc.Vias = append(nc.Vias, &geom.Via{At: b.pt(v.x, v.y), Size: nm(v.dia), Drill: nm(v.hole), LayerFrom: from, LayerTo: to})
	}
	// A drilled pad that belongs to no part, such as a stitching or mounting pad placed free, has
	// no placement to hang on, so it is kept as a via of its pad's size. ODB++ writes it the same way.
	for _, pd := range b.p.pads {
		if pd.comp >= 0 || pd.hole <= 0 {
			continue
		}
		nc := get(b.netName(pd.net))
		nc.Vias = append(nc.Vias, &geom.Via{At: b.pt(pd.x, pd.y), Size: nm(max(pd.w, pd.h)), Drill: nm(pd.hole), LayerFrom: "F.Cu", LayerTo: "B.Cu"})
	}
	for _, n := range order {
		b.g.Nets = append(b.g.Nets, byNet[n])
	}
}

// zones emits each polygon pour's outline as drawn, with its net. The poured copper is not kept, as
// for every other board format (agni issue 1357).
func (b *boardBuilder) zones() {
	for _, poly := range b.p.polygons {
		id, ok := b.byName[strings.ToUpper(poly["LAYER"])]
		layer := b.copper[id]
		if !ok || layer == "" {
			continue
		}
		pts := b.outline(poly)
		if len(pts) < 3 {
			continue
		}
		net := -1
		if n, ok := poly.int("NET"); ok {
			net = n
		}
		b.g.Zones = append(b.g.Zones, &geom.Zone{Net: b.netName(net), Layer: layer, Outline: &geom.Polyline{Points: pts}})
	}
}

// art emits silkscreen lines and arcs, and the designators and comments drawn on the silkscreen.
func (b *boardBuilder) art() {
	silk := map[byte]string{layerTopOverlay: "F.SilkS", layerBotOverlay: "B.SilkS"}
	ref := func(i int) string {
		if i < 0 || i >= len(b.p.comps) {
			return ""
		}
		return b.p.comps[i].ref
	}
	for _, t := range b.p.tracks {
		if layer := silk[t.layer]; layer != "" {
			b.g.Graphics = append(b.g.Graphics, &geom.BoardGraphic{
				Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{b.pt(t.x1, t.y1), b.pt(t.x2, t.y2)}},
				Layer: layer, RefDes: ref(t.comp), Width: nm(t.width),
			})
		}
	}
	for _, a := range b.p.arcs {
		if layer := silk[a.layer]; layer != "" {
			var pts []*geom.Point
			for _, q := range arcPoints(float64(a.cx), float64(a.cy), float64(a.r), a.start, a.end) {
				pts = append(pts, b.pt(int64(math.Round(q[0])), int64(math.Round(q[1]))))
			}
			b.g.Graphics = append(b.g.Graphics, &geom.BoardGraphic{
				Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: pts},
				Layer: layer, RefDes: ref(a.comp), Width: nm(a.width),
			})
		}
	}
	for _, t := range b.p.texts {
		layer := silk[t.layer]
		if layer == "" || t.s == "" || strings.HasPrefix(t.s, ".") {
			continue
		}
		kind := "user"
		switch {
		case t.comp < 0:
			kind = "gr"
		case t.designator:
			kind = "reference"
		case t.comment:
			kind = "value"
		}
		if b.hidden(t) {
			continue
		}
		b.g.Texts = append(b.g.Texts, &geom.BoardText{
			Text: t.s, At: b.pt(t.x, t.y), RotationDeg: geomath.NormDeg(t.rot), Height: nm(t.height),
			Layer: layer, Mirror: t.mirror, RefDes: ref(t.comp), Kind: kind,
		})
	}
}

// hidden reports whether a part's designator or comment text is switched off, as Altium hides it
// when the part's NAMEON or COMMENTON is false.
func (b *boardBuilder) hidden(t text) bool {
	if t.comp < 0 || t.comp >= len(b.p.comps) {
		return false
	}
	c := b.p.comps[t.comp].p
	switch {
	case t.designator:
		return strings.EqualFold(c["NAMEON"], "FALSE")
	case t.comment:
		return strings.EqualFold(c["COMMENTON"], "FALSE")
	}
	return false
}

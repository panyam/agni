package render

import (
	"math"
	"strings"

	"github.com/panyam/agni/core/svg"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// A pad or via that names a padstack draws its land on each copper layer the padstack has one,
// in that layer's group, so a reader who shows one layer sees that layer's lands at their own size
// (agni 1356). Through copper is then the plated barrel alone.

// stackIndex finds a board's padstack entries by id.
type stackIndex map[string]*geom.Padstack

func padstacksOf(b *geom.BoardGeometry) stackIndex {
	ix := stackIndex{}
	for _, ps := range b.GetPadstacks() {
		ix[ps.GetId()] = ps
	}
	return ix
}

// of is the entry a pad or via names, or nil when it names none the board holds. A nil entry
// draws the pad as it did before padstacks, from its own shape.
func (ix stackIndex) of(id string) *stackView {
	if ps, ok := ix[id]; ok && id != "" {
		return &stackView{ps}
	}
	return nil
}

type stackView struct{ ps *geom.Padstack }

// on is the padstack's entry for one copper layer, or nil where it has nothing there.
func (v *stackView) on(layer string) *geom.PadstackLayer {
	for _, l := range v.ps.GetLayers() {
		if l.GetLayer() == layer {
			return l
		}
	}
	return nil
}

// layers lists the copper layers the padstack has an entry on.
func (v *stackView) layers() []string {
	var out []string
	for _, l := range v.ps.GetLayers() {
		out = append(out, l.GetLayer())
	}
	return out
}

// planeIndex holds, for each plane layer, the nets its zones carry. A board marks a plane by its
// layer kind: Allegro's "power", IPC-2581's "PLANE".
type planeIndex map[string]map[string]bool

func planesOf(b *geom.BoardGeometry) planeIndex {
	ix := planeIndex{}
	for _, l := range b.GetLayers() {
		switch strings.ToLower(l.GetKind()) {
		case "power", "plane":
			ix[l.GetName()] = map[string]bool{}
		}
	}
	for _, z := range b.GetZones() {
		if nets, ok := ix[z.GetLayer()]; ok {
			nets[z.GetNet()] = true
		}
	}
	return ix
}

// cuts is whether a pad on net cuts an antipad on layer: the layer is a plane, and none of its
// zones carry the pad's net.
func (ix planeIndex) cuts(layer, net string) bool {
	nets, ok := ix[layer]
	return ok && !nets[net]
}

// drawStackLayer draws a padstack's land on one layer at the pad's position, and its antipad as an
// outline where cut says the layer clears around it. The antipad takes no clicks.
func drawStackLayer(c *svg.Canvas, pl *geom.ComponentPlacement, pad *geom.Pad, l *geom.PadstackLayer, fr sheetFrame, color string, cut bool, keys ...svg.Attr) {
	draw := func(sh *geom.PadShape, attrs ...svg.Attr) {
		cx, cy, ring := landAt(pl, pad, sh)
		if ring != nil {
			c.El("polygon", append([]svg.Attr{svg.A("points", points(ring, fr.tx, fr.ty))}, attrs...)...)
			return
		}
		drawShapeAt(c, cx, cy, sh.GetShape(), sh.GetSize(), geomath.PadAngle(pl, pad), fr, attrs...)
	}
	if a := l.GetAntipad(); cut && a != nil {
		draw(a, svg.A("fill", "none"), svg.A("stroke", color), svg.F("stroke-width", 2*strokePx),
			svg.A("stroke-dasharray", "3 2"), svg.A("pointer-events", "none"))
	}
	if p := l.GetPad(); p != nil {
		draw(p, append([]svg.Attr{svg.A("fill", color)}, keys...)...)
	}
}

// landAt places one shape of a padstack on the board (agni 1407). It returns the shape's centre,
// moved by its offset, and for a custom shape its outline as board points. Both go through
// geomath.PlaceLand, so the land turns and mirrors with its pad.
func landAt(pl *geom.ComponentPlacement, pad *geom.Pad, sh *geom.PadShape) (cx, cy int64, ring []*geom.Point) {
	ox, oy := float64(sh.GetOffset().GetX()), float64(sh.GetOffset().GetY())
	x, y := geomath.PlaceLand(pl, pad, ox, oy)
	if out := sh.GetOutline(); len(out) > 2 {
		ring = make([]*geom.Point, 0, len(out))
		for _, p := range out {
			px, py := geomath.PlaceLand(pl, pad, ox+float64(p.GetX()), oy+float64(p.GetY()))
			ring = append(ring, &geom.Point{X: int64(math.Round(px)), Y: int64(math.Round(py))})
		}
	}
	return int64(math.Round(x)), int64(math.Round(y)), ring
}

// drawBarrel draws a plated hole's barrel, a ring at the drill's edge around a page-coloured hole.
func drawBarrel(c *svg.Canvas, cx, cy, drillPx float64, style Style, keys ...svg.Attr) {
	c.El("circle", append([]svg.Attr{svg.F("cx", cx), svg.F("cy", cy), svg.F("r", math.Max(drillPx/2, strokePx/2)),
		svg.A("fill", style.Page), svg.A("stroke", style.Via), svg.F("stroke-width", strokePx)}, keys...)...)
}

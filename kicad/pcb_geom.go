package kicad

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// ReadBoardGeometry parses a KiCad .kicad_pcb board into the board-geometry sidecar
// (geom.BoardGeometry) — the physical layout the netlist reader (Read) deliberately
// drops: the layer table, the Edge.Cuts outline, per-footprint placement and pads, routed
// copper grouped by net, and zone outlines. It is joined to the netlist IR by ref_des
// (placements) and net name (copper); pad numbers match ir.Connection.pin_ref.
//
// Fidelity: lossy-bounded (render/DRC subset, CONSTRAINTS C6). Silkscreen/legend text and
// silk/fab body graphics (the footprint ref-des and value, free gr_text, and the fp_*/gr_*
// outlines) ARE carried (WS7); arc tracks, zone fill polygons, teardrops, and 3D references
// are not; outline and silk arcs are approximated as polylines. Coordinates are emitted in
// nanometers (unit_nm=1), Y-flipped to the geom contract's Y-up frame; rotations stay
// verbatim (see the proto contract). The caller owns file I/O (C1).
func ReadBoardGeometry(r io.Reader, sourceFile string) (*geom.BoardGeometry, error) {
	root, err := parse(r)
	if err != nil {
		return nil, err
	}
	if root.Head() != "kicad_pcb" {
		return nil, fmt.Errorf("kicad: not a .kicad_pcb file (root is %q)", root.Head())
	}
	return extractBoardGeometry(root, sourceFile), nil
}

func extractBoardGeometry(root *node, src string) *geom.BoardGeometry {
	g := &geom.BoardGeometry{UnitNm: 1, Prov: &geom.Provenance{SourceFile: src}}
	if tb := root.Child("title_block"); tb != nil {
		g.DesignRef = atomOf(tb.Child("title").Arg(1))
	}

	// The layer table doubles as the number->name map copper items resolve through.
	layerName := map[string]string{}
	if lt := root.Child("layers"); lt != nil {
		for _, l := range lt.Kids[1:] { // Kids[0] is the "layers" head atom
			name := atomOf(l.Arg(1))
			g.Layers = append(g.Layers, &geom.BoardLayer{
				Number: int32(atoi(l.Head())),
				Name:   name,
				Kind:   atomOf(l.Arg(2)),
			})
			layerName[l.Head()] = name
		}
	}

	// Nets referenced by copper items carry a number; the top-level net table maps it to
	// the name the IR keys on. (The netlist reader skips this table on purpose — pads
	// carry their names — but segments and vias reference by number only.)
	netName := map[string]string{}
	for _, n := range root.Children("net") {
		netName[atomOf(n.Arg(1))] = atomOf(n.Arg(2))
	}

	g.Outline = boardOutline(root)
	for _, fp := range root.Children("footprint") {
		if p := boardPlacement(fp, src); p != nil {
			g.Placements = append(g.Placements, p)
		}
	}
	g.Nets = netCopper(root, netName)
	for _, z := range root.Children("zone") {
		if zone := zoneOutline(z, netName); zone != nil {
			g.Zones = append(g.Zones, zone)
		}
	}
	g.Texts = boardTexts(root)
	g.Graphics = boardGraphics(root)
	return g
}

// graphicHeads are the KiCad graphic element base names, shared by the footprint (fp_*) and
// free (gr_*) forms.
var graphicHeads = []string{"line", "arc", "circle", "poly", "rect"}

// boardGraphics collects the board's non-copper artwork: each footprint's silkscreen and fab
// body graphics (fp_line/arc/circle/poly/rect, composed footprint-local -> board coordinates
// so they sit on the part) and free gr_* graphics that are NOT the board edge. The board
// edge (Edge.Cuts) is boardOutline's job and is excluded here; copper and zone fills are
// their own artifacts. Layer is kept verbatim so the renderer classifies silk vs fab.
func boardGraphics(root *node) []*geom.BoardGraphic {
	var out []*geom.BoardGraphic
	for _, fp := range root.Children("footprint") {
		ref := propValue(fp, "Reference")
		if ref == "" || placeholderRef(ref) {
			continue
		}
		origin := pcbPoint(fp.Child("at"))
		rot := -rotOf(fp.Child("at"))
		back := atomOf(fp.Child("layer").Arg(1)) == "B.Cu"
		place := func(p *geom.Point) *geom.Point { return geomath.ComposePlacement(origin, rot, back, p) }
		for _, base := range graphicHeads {
			for _, n := range fp.Children("fp_" + base) {
				if gr := boardGraphicOf(n, base, place, ref); gr != nil {
					out = append(out, gr)
				}
			}
		}
	}
	identity := func(p *geom.Point) *geom.Point { return p }
	for _, base := range graphicHeads {
		for _, n := range root.Children("gr_" + base) {
			if atomOf(n.Child("layer").Arg(1)) == "Edge.Cuts" {
				continue // the board edge is BoardOutline, not a graphic
			}
			if gr := boardGraphicOf(n, base, identity, ""); gr != nil {
				out = append(out, gr)
			}
		}
	}
	return out
}

// boardGraphicOf builds one BoardGraphic from a KiCad graphic node. place maps a Y-flipped
// source point into board coordinates: it composes the footprint transform for fp_* graphics,
// and is the identity for free gr_* graphics (already absolute). Returns nil if the node lacks
// the geometry its kind needs.
func boardGraphicOf(n *node, base string, place func(*geom.Point) *geom.Point, ref string) *geom.BoardGraphic {
	shape := graphicShape(n, base, place)
	if shape == nil {
		return nil
	}
	return &geom.BoardGraphic{
		Shape:  shape,
		Layer:  atomOf(n.Child("layer").Arg(1)),
		RefDes: ref,
		Width:  graphicWidth(n),
	}
}

// graphicShape converts a KiCad graphic node into a geom.Shape, mapping each source point
// through place. Arcs are approximated as polylines by the shared arcPolyline (the same C6
// bound the board outline uses), and circles keep center+radius (radius is rotation- and
// mirror-invariant, so it is measured in the source frame). Returns nil for a malformed node.
func graphicShape(n *node, base string, place func(*geom.Point) *geom.Point) *geom.Shape {
	switch base {
	case "line":
		s, e := n.Child("start"), n.Child("end")
		if s == nil || e == nil {
			return nil
		}
		return &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{place(pcbPoint(s)), place(pcbPoint(e))}}
	case "rect":
		s, e := n.Child("start"), n.Child("end")
		if s == nil || e == nil {
			return nil
		}
		return &geom.Shape{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{place(pcbPoint(s)), place(pcbPoint(e))}, Fill: graphicFill(n)}
	case "circle":
		c, e := n.Child("center"), n.Child("end")
		if c == nil || e == nil {
			return nil
		}
		cp, ep := pcbPoint(c), pcbPoint(e)
		r := int64(math.Round(math.Hypot(float64(ep.X-cp.X), float64(ep.Y-cp.Y))))
		return &geom.Shape{Kind: geom.Shape_KIND_CIRCLE, Points: []*geom.Point{place(cp)}, Radius: r, Fill: graphicFill(n)}
	case "arc":
		s, m, e := n.Child("start"), n.Child("mid"), n.Child("end")
		if s == nil || m == nil || e == nil {
			return nil
		}
		pl := arcPolyline(pcbPoint(s), pcbPoint(m), pcbPoint(e))
		pts := make([]*geom.Point, len(pl.Points))
		for i, p := range pl.Points {
			pts[i] = place(p)
		}
		return &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: pts}
	case "poly":
		pts := n.Child("pts")
		if pts == nil {
			return nil
		}
		var pp []*geom.Point
		for _, xy := range pts.Children("xy") {
			pp = append(pp, place(pcbPoint(xy)))
		}
		if len(pp) == 0 {
			return nil
		}
		return &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: pp, Fill: graphicFill(n)}
	}
	return nil
}

// graphicFill maps a KiCad (fill ...) node to the shape fill: a filled body fills with the
// silk/outline color (FILL_OUTLINE, since silk is a single color), otherwise outline only.
func graphicFill(n *node) geom.Shape_Fill {
	f := n.Child("fill")
	if f == nil {
		return geom.Shape_FILL_UNSPECIFIED
	}
	switch atomOf(f.Arg(1)) {
	case "yes", "solid":
		return geom.Shape_FILL_OUTLINE
	}
	return geom.Shape_FILL_UNSPECIFIED
}

// graphicWidth reads a graphic's stroke width (source units), across the (stroke (width W))
// form (v7+) and the older bare (width W) child; 0 when absent.
func graphicWidth(n *node) int64 {
	if s := n.Child("stroke"); s != nil {
		if w := s.Child("width"); w != nil {
			return mmToNm(atomOf(w.Arg(1)))
		}
	}
	if w := n.Child("width"); w != nil {
		return mmToNm(atomOf(w.Arg(1)))
	}
	return 0
}

// boardTexts collects the board's placed text: each footprint's silkscreen ref-des and
// value (composed from footprint-local to board coordinates so they pin to the pads), and
// free graphic text (gr_text, including the title block, authored absolute). Hidden source
// text is dropped, mirroring the no-ref placement skip. Footprint user text (fp_text) and
// silk/fab graphics are out of scope for this tier (C6 bound; see the proto contract).
func boardTexts(root *node) []*geom.BoardText {
	var out []*geom.BoardText
	for _, fp := range root.Children("footprint") {
		ref := propValue(fp, "Reference")
		if ref == "" || placeholderRef(ref) {
			continue
		}
		fpAt := fp.Child("at")
		back := atomOf(fp.Child("layer").Arg(1)) == "B.Cu"
		for _, p := range fp.Children("property") {
			var kind string
			switch atomOf(p.Arg(1)) {
			case "Reference":
				kind = "reference"
			case "Value":
				kind = "value"
			default:
				continue
			}
			if t := footprintText(p, atomOf(p.Arg(2)), kind, ref, fpAt, back); t != nil {
				out = append(out, t)
			}
		}
	}
	for _, g := range root.Children("gr_text") {
		if t := freeText(g); t != nil {
			out = append(out, t)
		}
	}
	return out
}

// footprintText composes one footprint-owned text (a property or fp_text) into a board-frame
// BoardText. Position: the footprint-local offset is rotated by the footprint orientation
// and mirrored on the back via geomath.ComposePlacement — the SAME composer padWorld uses,
// so text lands on its part. Glyph rotation is the footprint orientation plus the text's own
// local angle, carried in the Y-up frame (a Y-down source negates). Returns nil for empty or
// hidden text, or a text with no local (at).
func footprintText(n *node, text, kind, ref string, fpAt *node, back bool) *geom.BoardText {
	local := n.Child("at")
	if text == "" || local == nil || isHidden(n) {
		return nil
	}
	height, justify, mirror := effectsOf(n)
	return &geom.BoardText{
		Text:        text,
		At:          geomath.ComposePlacement(pcbPoint(fpAt), -rotOf(fpAt), back, pcbPoint(local)),
		RotationDeg: keepUpright(-(rotOf(fpAt) + rotOf(local))),
		Height:      height,
		Layer:       atomOf(n.Child("layer").Arg(1)),
		Mirror:      back || mirror,
		Justify:     justify,
		RefDes:      ref,
		Kind:        kind,
	}
}

// freeText converts a top-level gr_text (authored in absolute board coordinates) into a
// BoardText. Returns nil for empty or hidden text.
func freeText(n *node) *geom.BoardText {
	text := atomOf(n.Arg(1))
	if text == "" || isHidden(n) {
		return nil
	}
	at := n.Child("at")
	height, justify, mirror := effectsOf(n)
	return &geom.BoardText{
		Text:        text,
		At:          pcbPoint(at),
		RotationDeg: -rotOf(at),
		Height:      height,
		Layer:       atomOf(n.Child("layer").Arg(1)),
		Mirror:      mirror,
		Justify:     justify,
		Kind:        "gr",
	}
}

// effectsOf reads a text node's (effects (font (size w h)) (justify ...)) into glyph height
// (nanometers, from the font's y size), horizontal justify ("left"/"right"/"" center), and
// whether the justify marks the text mirrored (back-side legend).
func effectsOf(n *node) (height int64, justify string, mirror bool) {
	eff := n.Child("effects")
	if eff == nil {
		return 0, "", false
	}
	if f := eff.Child("font"); f != nil {
		if sz := f.Child("size"); sz != nil {
			height = mmToNm(atomOf(sz.Arg(2)))
		}
	}
	if j := eff.Child("justify"); j != nil {
		for i := 1; j.Arg(i) != nil; i++ {
			switch a := atomOf(j.Arg(i)); a {
			case "mirror":
				mirror = true
			case "left", "right":
				justify = a
			}
		}
	}
	return height, justify, mirror
}

// keepUpright folds a footprint text's glyph angle into (-90, 90] by half turns, so text on
// a rotated (e.g. 180°) footprint never renders upside down — matching KiCad's default
// keep_upright behavior for silkscreen fields. Free gr_text is exempt: it stays as authored
// (a mirrored back-side title is intentional).
func keepUpright(deg float64) float64 {
	for deg > 90 {
		deg -= 180
	}
	for deg <= -90 {
		deg += 180
	}
	return deg
}

// isHidden reports whether a text node is marked hidden — KiCad's (hide yes) child (v7+) or
// the bare (hide) form — so dropped text never renders.
func isHidden(n *node) bool {
	h := n.Child("hide")
	if h == nil {
		return false
	}
	return h.Arg(1) == nil || atomOf(h.Arg(1)) == "yes"
}

// pcbPoint reads an (xy ...) / (at ...) / (start ...) style node's x,y into the Y-up
// nanometer frame.
func pcbPoint(n *node) *geom.Point {
	return &geom.Point{X: mmToNm(atomOf(n.Arg(1))), Y: -mmToNm(atomOf(n.Arg(2)))}
}

// rotOf reads the optional third argument of an (at x y rot) node, verbatim degrees.
func rotOf(n *node) float64 {
	if n == nil || n.Arg(3) == nil {
		return 0
	}
	f, _ := strconv.ParseFloat(atomOf(n.Arg(3)), 64)
	return f
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

// boardOutline collects the Edge.Cuts graphics: gr_line as a two-point path, gr_rect as
// its four edges (one closed path), gr_arc approximated by arcPolyline. Paths stay as
// authored; stitching them into a closed polygon is the consumer's job.
func boardOutline(root *node) *geom.BoardOutline {
	out := &geom.BoardOutline{}
	onEdge := func(n *node) bool { return atomOf(n.Child("layer").Arg(1)) == "Edge.Cuts" }
	for _, l := range root.Children("gr_line") {
		if onEdge(l) {
			out.Paths = append(out.Paths, &geom.Polyline{Points: []*geom.Point{
				pcbPoint(l.Child("start")), pcbPoint(l.Child("end")),
			}})
		}
	}
	for _, r := range root.Children("gr_rect") {
		if onEdge(r) {
			a, b := pcbPoint(r.Child("start")), pcbPoint(r.Child("end"))
			out.Paths = append(out.Paths, &geom.Polyline{Points: []*geom.Point{
				a, {X: b.X, Y: a.Y}, b, {X: a.X, Y: b.Y}, a,
			}})
		}
	}
	for _, a := range root.Children("gr_arc") {
		if onEdge(a) {
			out.Paths = append(out.Paths, arcPolyline(
				pcbPoint(a.Child("start")), pcbPoint(a.Child("mid")), pcbPoint(a.Child("end"))))
		}
	}
	if len(out.Paths) == 0 {
		return nil
	}
	return out
}

// arcPolyline approximates the circular arc through three points with a fixed-step
// polyline (16 segments). A degenerate (collinear) triple falls back to the chord.
func arcPolyline(s, m, e *geom.Point) *geom.Polyline {
	cx, cy, ok := circumcenter(float64(s.X), float64(s.Y), float64(m.X), float64(m.Y), float64(e.X), float64(e.Y))
	if !ok {
		return &geom.Polyline{Points: []*geom.Point{s, m, e}}
	}
	a0 := math.Atan2(float64(s.Y)-cy, float64(s.X)-cx)
	a1 := math.Atan2(float64(m.Y)-cy, float64(m.X)-cx)
	a2 := math.Atan2(float64(e.Y)-cy, float64(e.X)-cx)
	// Sweep from a0 to a2 passing through a1: normalize both deltas into the same
	// direction; if the midpoint is not inside the CCW sweep, go CW.
	ccw := func(from, to float64) float64 {
		d := to - from
		for d < 0 {
			d += 2 * math.Pi
		}
		return d
	}
	sweep := ccw(a0, a2)
	if ccw(a0, a1) > sweep {
		sweep -= 2 * math.Pi // midpoint outside the CCW span: sweep clockwise instead
	}
	r := math.Hypot(float64(s.X)-cx, float64(s.Y)-cy)
	const steps = 16
	pl := &geom.Polyline{}
	for i := 0; i <= steps; i++ {
		a := a0 + sweep*float64(i)/steps
		pl.Points = append(pl.Points, &geom.Point{
			X: int64(math.Round(cx + r*math.Cos(a))),
			Y: int64(math.Round(cy + r*math.Sin(a))),
		})
	}
	return pl
}

// circumcenter returns the center of the circle through three points, ok=false when
// they are (near-)collinear.
func circumcenter(ax, ay, bx, by, cx, cy float64) (x, y float64, ok bool) {
	d := 2 * (ax*(by-cy) + bx*(cy-ay) + cx*(ay-by))
	if math.Abs(d) < 1e-9 {
		return 0, 0, false
	}
	a2, b2, c2 := ax*ax+ay*ay, bx*bx+by*by, cx*cx+cy*cy
	x = (a2*(by-cy) + b2*(cy-ay) + c2*(ay-by)) / d
	y = (a2*(cx-bx) + b2*(ax-cx) + c2*(bx-ax)) / d
	return x, y, true
}

// boardPlacement extracts one footprint's placement and pads; a footprint with no Reference
// (a graphic) yields nil, mirroring the netlist reader's skip so the two artifacts agree
// on which components exist.
func boardPlacement(fp *node, src string) *geom.ComponentPlacement {
	ref := propValue(fp, "Reference")
	if ref == "" || placeholderRef(ref) {
		return nil
	}
	at := fp.Child("at")
	layer := atomOf(fp.Child("layer").Arg(1))
	// KiCad is Y-down and pcbPoint Y-flips coordinates; a CCW rotation there is CW in the
	// canonical Y-up frame, so the reader negates it as part of that same frame conversion
	// (WS1-030). A footprint on B.Cu is mirrored across X; the renderer applies the flag.
	p := &geom.ComponentPlacement{
		RefDes:      ref,
		At:          pcbPoint(at),
		RotationDeg: -rotOf(at),
		Layer:       layer,
		Mirror:      layer == "B.Cu",
		Prov:        &geom.Provenance{SourceFile: src, SourceId: uuidOf(fp)},
	}
	for _, pad := range fp.Children("pad") {
		pat := pad.Child("at")
		g := &geom.Pad{
			Number:      atomOf(pad.Arg(1)),
			At:          pcbPoint(pat),
			RotationDeg: rotOf(pat),
			Shape:       atomOf(pad.Arg(3)),
		}
		if sz := pad.Child("size"); sz != nil {
			g.Size = &geom.Point{X: mmToNm(atomOf(sz.Arg(1))), Y: mmToNm(atomOf(sz.Arg(2)))}
		}
		if dr := pad.Child("drill"); dr != nil {
			g.Drill = mmToNm(atomOf(dr.Arg(1)))
		}
		if ls := pad.Child("layers"); ls != nil {
			for i := 1; ls.Arg(i) != nil; i++ {
				g.Layers = append(g.Layers, atomOf(ls.Arg(i)))
			}
		}
		if nr := pad.Child("net"); nr != nil {
			g.Net = atomOf(nr.Arg(len(nr.Kids) - 1)) // name is the last argument in both forms
		}
		p.Pads = append(p.Pads, g)
	}
	return p
}

// netCopper groups the board's segments and vias by resolved net name, sorted by name
// for a deterministic artifact. Items on net 0 / an unknown number ("no net") are
// dropped: copper the IR has no key for cannot join.
func netCopper(root *node, netName map[string]string) []*geom.NetCopper {
	byNet := map[string]*geom.NetCopper{}
	get := func(n *node) *geom.NetCopper {
		name := copperNet(n.Child("net"), netName)
		if name == "" {
			return nil
		}
		nc := byNet[name]
		if nc == nil {
			nc = &geom.NetCopper{Net: name}
			byNet[name] = nc
		}
		return nc
	}
	for _, s := range root.Children("segment") {
		nc := get(s)
		if nc == nil {
			continue
		}
		nc.Segments = append(nc.Segments, &geom.TrackSegment{
			A:     pcbPoint(s.Child("start")),
			B:     pcbPoint(s.Child("end")),
			Width: mmToNm(atomOf(s.Child("width").Arg(1))),
			Layer: atomOf(s.Child("layer").Arg(1)),
		})
	}
	for _, v := range root.Children("via") {
		nc := get(v)
		if nc == nil {
			continue
		}
		via := &geom.Via{
			At:    pcbPoint(v.Child("at")),
			Size:  mmToNm(atomOf(v.Child("size").Arg(1))),
			Drill: mmToNm(atomOf(v.Child("drill").Arg(1))),
		}
		if ls := v.Child("layers"); ls != nil {
			via.LayerFrom, via.LayerTo = atomOf(ls.Arg(1)), atomOf(ls.Arg(2))
		}
		nc.Vias = append(nc.Vias, via)
	}
	out := make([]*geom.NetCopper, 0, len(byNet))
	for _, nc := range byNet {
		out = append(out, nc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Net < out[j].Net })
	return out
}

// copperNet resolves a copper item's (net ...) reference to the net name the IR keys on,
// across the three source forms: (net N "name") carries the name directly; (net N) is the
// pre-KiCad-10 numbered form resolved through the board's net table; (net "name") is the
// KiCad 10 name-only form, recognizable because KiCad 10 also drops the table (and a name
// is not all digits). "" means no net / unresolvable — the caller drops the copper.
func copperNet(ref *node, netName map[string]string) string {
	if ref == nil || ref.Arg(1) == nil {
		return ""
	}
	if len(ref.Kids) > 2 { // (net N "name"): the name is the last argument
		return atomOf(ref.Arg(len(ref.Kids) - 1))
	}
	key := atomOf(ref.Arg(1))
	if name, ok := netName[key]; ok {
		return name
	}
	if _, err := strconv.Atoi(key); err != nil || len(netName) == 0 {
		return key // name-only form (KiCad 10)
	}
	return "" // a number the table does not know: no net
}

// zoneOutline extracts one zone's authored polygon. A zone on no net (a keepout) keeps
// net="" — it is real copper geometry even though it joins to nothing.
func zoneOutline(z *node, netName map[string]string) *geom.Zone {
	poly := z.Child("polygon")
	if poly == nil {
		return nil
	}
	pts := poly.Child("pts")
	if pts == nil {
		return nil
	}
	zone := &geom.Zone{Outline: &geom.Polyline{}}
	if nn := z.Child("net_name"); nn != nil {
		zone.Net = atomOf(nn.Arg(1))
	} else {
		zone.Net = copperNet(z.Child("net"), netName)
	}
	if l := z.Child("layer"); l != nil {
		zone.Layer = atomOf(l.Arg(1))
	} else if ls := z.Child("layers"); ls != nil {
		zone.Layer = atomOf(ls.Arg(1))
	}
	for _, xy := range pts.Children("xy") {
		zone.Outline.Points = append(zone.Outline.Points, pcbPoint(xy))
	}
	return zone
}

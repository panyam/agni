package ipc2581

import (
	"encoding/xml"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// arcSegments is the fixed polyline count an outline or zone arc step is approximated with,
// matching the KiCad outline-arc precedent. A track's arc step is kept as an arc (trackSegments).
const arcSegments = 16

// Copper XML subset. A LayerFeature groups per-layer feature Sets. Each Set names its net and
// holds routed Polylines (tracks), copper Pads, and drilled Holes (vias).
type layerFeatureEl struct {
	LayerRef string  `xml:"layerRef,attr"`
	Sets     []setEl `xml:"Set"`
}

type setEl struct {
	Net      string      `xml:"net,attr"`
	Tracks   []polyPath  `xml:"Features>Polyline"`
	Lines    []lineEl    `xml:"Features>Line"`
	FeatPads []cPadEl    `xml:"Features>Pad"`
	Pads     []cPadEl    `xml:"Pad"`
	Holes    []holeEl    `xml:"Hole"`
	Fills    []contourEl `xml:"Features>Contour"` // copper plane/pour: a filled region, authored outline (C6)
}

// lineEl is a single straight track segment. Allegro writes one for a route with no bend, in place
// of a two-point Polyline (agni 1383).
type lineEl struct {
	StartX  string     `xml:"startX,attr"`
	StartY  string     `xml:"startY,attr"`
	EndX    string     `xml:"endX,attr"`
	EndY    string     `xml:"endY,attr"`
	Desc    lineDescEl `xml:"LineDesc"`
	DescRef primRefEl  `xml:"LineDescRef"`
}

type cPadEl struct {
	Location ptEl      `xml:"Location"`
	PrimRef  primRefEl `xml:"StandardPrimitiveRef"`
	UserRef  primRefEl `xml:"UserPrimitiveRef"`
	// Padstack names the padstack the land belongs to, and PinRef the component pin, which a via's
	// land lacks (agni 1356).
	Padstack string   `xml:"padstackDefRef,attr"`
	PinRef   pinRefEl `xml:"PinRef"`
}

// primID returns the pad's primitive id. It prefers a standard-dictionary reference and falls
// back to a user-dictionary one. Both resolve through the same prims map.
func (p cPadEl) primID() string {
	if p.PrimRef.ID != "" {
		return p.PrimRef.ID
	}
	return p.UserRef.ID
}

type holeEl struct {
	Diameter      string `xml:"diameter,attr"`
	PlatingStatus string `xml:"platingStatus,attr"`
	X             string `xml:"x,attr"`
	Y             string `xml:"y,attr"`
}

// dictLineDesc is a line-descriptor dictionary of trace widths. A track's LineDescRef references
// an entry by id. The dictionary is the width-def half of the def/instance split, mirroring the
// padstack dict.
type dictLineDesc struct {
	Entries []entryLineDescEl `xml:"EntryLineDesc"`
}

type entryLineDescEl struct {
	ID   string     `xml:"id,attr"`
	Line lineDescEl `xml:"LineDesc"`
}

type lineDescEl struct {
	Width string `xml:"lineWidth,attr"`
}

// polyStep is one ordered step of a polyline, a destination point plus the arc center and
// direction when it is an arc. Interleaved segment/curve order is significant, so the steps are
// decoded in document order (see polyPath.UnmarshalXML) rather than into per-kind slices.
type polyStep struct {
	x, y   float64
	isArc  bool
	cx, cy float64
	cw     bool
}

// polyPath is a track/outline polyline decoded in document order. It holds a start point,
// ordered steps, and the id of the LineDesc that sets its width, or the width a LineDesc inside it
// gives.
type polyPath struct {
	x, y    float64
	steps   []polyStep
	lineRef string
	// lineWidth is an inline LineDesc's width in source units. Allegro writes one inside each
	// Polyline rather than naming a dictionary entry (agni 1383).
	lineWidth string
	// xform is the polygon's own Xform, which a pad primitive uses to place its shape off the pad's
	// centre and turn it with the pin (agni 1407).
	xOff, yOff, rot float64
}

// UnmarshalXML reads a Polyline's children in order, so straight (PolyStepSegment) and arc
// (PolyStepCurve) steps keep their interleaving. encoding/xml's default slice decoding would
// scramble them and corrupt the path.
func (p *polyPath) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "PolyBegin":
				p.x, p.y = attrF(t, "x"), attrF(t, "y")
			case "PolyStepSegment":
				p.steps = append(p.steps, polyStep{x: attrF(t, "x"), y: attrF(t, "y")})
			case "PolyStepCurve":
				p.steps = append(p.steps, polyStep{
					x: attrF(t, "x"), y: attrF(t, "y"), isArc: true,
					cx: attrF(t, "centerX"), cy: attrF(t, "centerY"),
					cw: strings.EqualFold(attr(t, "clockwise"), "true"), // Allegro emits "TRUE"/"FALSE"
				})
			case "LineDescRef":
				p.lineRef = attr(t, "id")
			case "LineDesc":
				p.lineWidth = attr(t, "lineWidth")
			case "Xform":
				p.xOff, p.yOff, p.rot = attrF(t, "xOffset"), attrF(t, "yOffset"), attrF(t, "rotation")
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

// copperNets groups routed copper by net into NetCopper, with each track step a TrackSegment (an
// arc step keeps its curve) and vias from drilled holes. Sorted by net name for
// deterministic output, mirroring the KiCad producer.
func (f *ipcGeomFile) copperNets(nmPerUnit float64, prims map[string]primShape, stacks *padstackIndex) []*geom.NetCopper {
	widths := f.lineWidths(nmPerUnit)
	padSize := f.copperPadIndex(nmPerUnit, prims) // (net,x,y) -> annular pad radius source
	spans := f.layerSpans()                       // drill-layer name -> (from,to) copper layers
	byNet := map[string]*geom.NetCopper{}
	order := []string{}
	get := func(net string) *geom.NetCopper {
		nc := byNet[net]
		if nc == nil {
			nc = &geom.NetCopper{Net: net}
			byNet[net] = nc
			order = append(order, net)
		}
		return nc
	}

	copper := f.copperLayers()
	names := f.layerNames()
	lands := f.landNets(nmPerUnit)
	for _, lf := range f.Features {
		layer := names.of(lf.LayerRef)
		for _, s := range lf.Sets {
			// A Set-direct Contour on a copper layer is a pour or plane, copper as etched, with its
			// cutouts as holes (agni 1357). On a fab, mask or silk layer it is drawing geometry.
			if copper[lf.LayerRef] {
				for _, c := range s.Fills {
					if sh := copperShape(c, layer, nmPerUnit); sh != nil {
						nc := get(s.Net)
						nc.Shapes = append(nc.Shapes, sh)
					}
				}
			}
			for _, h := range s.Holes {
				if !isVia(h.PlatingStatus) {
					continue
				}
				at := ptNm(ptEl{X: h.X, Y: h.Y}, nmPerUnit)
				// Allegro writes each drill hit in a Set of its own with no net, so a via takes the
				// net of the land drawn at its position (agni 1383).
				net := s.Net
				if net == "" {
					net = lands[[2]int64{at.X, at.Y}]
				}
				if net == "" {
					continue
				}
				drill := lenNm(h.Diameter, nmPerUnit)
				size := padSize[padKey(net, at)]
				if size == 0 {
					size = drill // no co-located pad: annular reads as zero ring, still a placed via
				}
				via := &geom.Via{At: at, Size: size, Drill: drill, Padstack: stacks.via[padKey(net, at)]}
				if sp, ok := spans[lf.LayerRef]; ok {
					via.LayerFrom, via.LayerTo = names.of(sp[0]), names.of(sp[1])
				}
				nc := get(net)
				nc.Vias = append(nc.Vias, via)
			}
			if s.Net == "" {
				continue
			}
			nc := get(s.Net)
			for _, ln := range s.Lines {
				w, ok := widths[ln.DescRef.ID]
				if !ok {
					w = lenNm(ln.Desc.Width, nmPerUnit)
				}
				nc.Segments = append(nc.Segments, &geom.TrackSegment{
					A: ptNm(ptEl{X: ln.StartX, Y: ln.StartY}, nmPerUnit), B: ptNm(ptEl{X: ln.EndX, Y: ln.EndY}, nmPerUnit),
					Width: w, Layer: layer,
				})
			}
			for _, tr := range s.Tracks {
				w, ok := widths[tr.lineRef]
				if !ok && tr.lineWidth != "" {
					w = lenNm(tr.lineWidth, nmPerUnit)
				}
				nc.Segments = append(nc.Segments, trackSegments(tr, nmPerUnit, w, layer)...)
			}
		}
	}

	out := make([]*geom.NetCopper, 0, len(order))
	for _, net := range order {
		out = append(out, byNet[net])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Net < out[j].Net })
	return out
}

// copperLayers is the set of layer names whose IPC-2581 layerFunction is a copper function
// (CONDUCTOR or PLANE). Only these hold pours/planes that become geom.Zones. DOCUMENT (fab,
// assembly), mask, silkscreen and drill Contours are drawing geometry, not copper.
func (f *ipcGeomFile) copperLayers() map[string]bool {
	m := map[string]bool{}
	for _, l := range f.Layers {
		switch strings.ToUpper(l.Function) {
		case "CONDUCTOR", "SIGNAL", "PLANE", "POWER_GROUND", "MIXED":
			m[l.Name] = true
		}
	}
	return m
}

// layerNamer maps a file's layer name onto the contract's vocabulary.
type layerNamer map[string]string

// of is ref's contract name. A copper layer is named by its place in the stackup, and any other
// reference goes through normalizeSide.
func (n layerNamer) of(ref string) string {
	if name, ok := n[ref]; ok {
		return name
	}
	return normalizeSide(ref)
}

// layerNames names the copper layers in stackup order, as the ODB++ reader does: the first is F.Cu,
// the last B.Cu, and the inner ones In1.Cu upward. Allegro's export keeps its own names (`L2-GND`),
// which no board rule or renderer reads as copper (agni 1383).
func (f *ipcGeomFile) layerNames() layerNamer {
	copper := f.copperLayers()
	var order []string
	for _, l := range f.Layers {
		if copper[l.Name] {
			order = append(order, l.Name)
		}
	}
	n := layerNamer{}
	for i, name := range order {
		switch {
		case i == 0:
			n[name] = "F.Cu"
		case i == len(order)-1:
			n[name] = "B.Cu"
		default:
			n[name] = "In" + strconv.Itoa(i) + ".Cu"
		}
	}
	return n
}

// landNets indexes the net of every via land on a copper layer by its position, so a drill hit in a
// Set with no net finds its via's net.
func (f *ipcGeomFile) landNets(nmPerUnit float64) map[[2]int64]string {
	copper := f.copperLayers()
	m := map[[2]int64]string{}
	for _, lf := range f.Features {
		if !copper[lf.LayerRef] {
			continue
		}
		for _, s := range lf.Sets {
			if s.Net == "" {
				continue
			}
			for _, pads := range [][]cPadEl{s.Pads, s.FeatPads} {
				for _, pd := range pads {
					at := ptNm(pd.Location, nmPerUnit)
					m[[2]int64{at.X, at.Y}] = s.Net
				}
			}
		}
	}
	return m
}

// pinNets indexes the net of each (ref-des, pin) a copper land names. A package pin in IPC-2581
// carries no net, so a pin's net is its land's Set's net (agni 1383).
func (f *ipcGeomFile) pinNets() map[[2]string]string {
	copper := f.copperLayers()
	m := map[[2]string]string{}
	for _, lf := range f.Features {
		if !copper[lf.LayerRef] {
			continue
		}
		for _, s := range lf.Sets {
			for _, pads := range [][]cPadEl{s.Pads, s.FeatPads} {
				for _, pd := range pads {
					k := [2]string{pd.PinRef.ComponentRef, pd.PinRef.Pin}
					if k[0] != "" && m[k] == "" {
						m[k] = s.Net
					}
				}
			}
		}
	}
	return m
}

// pinHoles indexes each plated or unplated hole's diameter by its position to the micrometre, so a
// pad finds its drill at the point it composes to. A mechanical pin has no copper land, so its
// position is the only key it shares with its hole.
func (f *ipcGeomFile) pinHoles(nmPerUnit float64) map[[2]int64]int64 {
	m := map[[2]int64]int64{}
	for _, lf := range f.Features {
		for _, s := range lf.Sets {
			for _, h := range s.Holes {
				if isVia(h.PlatingStatus) {
					continue
				}
				m[umKey(ptNm(ptEl{X: h.X, Y: h.Y}, nmPerUnit))] = lenNm(h.Diameter, nmPerUnit)
			}
		}
	}
	return m
}

func umKey(p *geom.Point) [2]int64 {
	return [2]int64{rnd(float64(p.X) / 1000), rnd(float64(p.Y) / 1000)}
}

// layerSpans indexes each drill layer's Span (fromLayer,toLayer) so a via records the copper
// layers it bridges. A through via spans the outer pair, blind/buried vias a narrower one.
//
// Allegro's export writes no Span, and names the drill layer by the copper layers it joins instead,
// counted from 1 in stackup order (`IPC2581DRILL_1-6`). A drill layer named that way spans those
// layers (agni 1383).
func (f *ipcGeomFile) layerSpans() map[string][2]string {
	copper := f.copperLayers()
	var order []string
	for _, l := range f.Layers {
		if copper[l.Name] {
			order = append(order, l.Name)
		}
	}
	m := map[string][2]string{}
	for _, l := range f.Layers {
		if l.Span != nil {
			m[l.Name] = [2]string{l.Span.From, l.Span.To}
			continue
		}
		if !strings.EqualFold(l.Function, "DRILL") {
			continue
		}
		if g := drillSpanName.FindStringSubmatch(l.Name); g != nil {
			from, _ := strconv.Atoi(g[1])
			to, _ := strconv.Atoi(g[2])
			if from >= 1 && to >= 1 && from <= len(order) && to <= len(order) {
				m[l.Name] = [2]string{order[from-1], order[to-1]}
			}
		}
	}
	return m
}

// drillSpanName matches the copper span Allegro writes at the end of a drill layer's name.
var drillSpanName = regexp.MustCompile(`_(\d+)-(\d+)$`)

// copperShape is a contour's polygon as a copper shape on layer, with each cutout a hole, or nil
// for a degenerate polygon.
func copperShape(c contourEl, layer string, nmPerUnit float64) *geom.CopperShape {
	outline := polyline(c.Polygon, nmPerUnit)
	if outline == nil {
		return nil
	}
	sh := &geom.CopperShape{Layer: layer, Outline: outline}
	for _, cut := range c.Cutouts {
		if h := polyline(cut, nmPerUnit); h != nil {
			sh.Holes = append(sh.Holes, h)
		}
	}
	return sh
}

// lineWidths resolves the LineDesc dictionaries into a width lookup keyed by id.
func (f *ipcGeomFile) lineWidths(nmPerUnit float64) map[string]int64 {
	m := map[string]int64{}
	for _, d := range f.LineDescs {
		for _, e := range d.Entries {
			m[e.ID] = lenNm(e.Line.Width, nmPerUnit)
		}
	}
	return m
}

// copperPadIndex indexes copper pad radii by (net, location), so a via's annular pad size comes
// from the co-located copper pad. The drilled Hole and its copper land are separate features,
// and they share a net and position.
func (f *ipcGeomFile) copperPadIndex(nmPerUnit float64, prims map[string]primShape) map[string]int64 {
	m := map[string]int64{}
	for _, lf := range f.Features {
		for _, s := range lf.Sets {
			for _, pads := range [][]cPadEl{s.Pads, s.FeatPads} {
				for _, pd := range pads {
					at := ptNm(pd.Location, nmPerUnit)
					if p, ok := prims[pd.primID()]; ok {
						m[padKey(s.Net, at)] = maxI64(m[padKey(s.Net, at)], p.sizeX)
					}
				}
			}
		}
	}
	return m
}

func padKey(net string, at *geom.Point) string {
	return net + "\x00" + itoa(at.X) + "\x00" + itoa(at.Y)
}

// flattenPath turns a polyPath into an integer-nm point list, approximating arc steps as
// arcSegments straight chords.
func flattenPath(p polyPath, nmPerUnit float64) []*geom.Point {
	pts := []*geom.Point{{X: rnd(p.x * nmPerUnit), Y: rnd(p.y * nmPerUnit)}}
	px, py := p.x, p.y
	for _, s := range p.steps {
		if !s.isArc {
			pts = append(pts, &geom.Point{X: rnd(s.x * nmPerUnit), Y: rnd(s.y * nmPerUnit)})
			px, py = s.x, s.y
			continue
		}
		pts = append(pts, arcPoints(px, py, s, nmPerUnit)...)
		px, py = s.x, s.y
	}
	return pts
}

// trackSegments reads a track's polyline as one TrackSegment per step. A PolyStepCurve becomes an
// arc, with its mid point computed from the center and direction (agni issue 1358).
func trackSegments(p polyPath, nmPerUnit float64, width int64, layer string) []*geom.TrackSegment {
	at := func(x, y float64) *geom.Point { return &geom.Point{X: rnd(x * nmPerUnit), Y: rnd(y * nmPerUnit)} }
	var out []*geom.TrackSegment
	px, py := p.x, p.y
	for _, s := range p.steps {
		if !s.isArc {
			out = append(out, &geom.TrackSegment{A: at(px, py), B: at(s.x, s.y), Width: width, Layer: layer})
		} else {
			for _, a := range geomath.CenterArc(px, py, s.x, s.y, s.cx, s.cy, s.cw) {
				out = append(out, &geom.TrackSegment{
					A: at(a.S[0], a.S[1]), Mid: at(a.M[0], a.M[1]), B: at(a.E[0], a.E[1]), Width: width, Layer: layer,
				})
			}
		}
		px, py = s.x, s.y
	}
	return out
}

// arcPoints approximates one arc step as arcSegments chords. The step runs from the current
// point to s.x,s.y about s.cx,s.cy, and walks the shorter or longer way per the clockwise flag.
func arcPoints(fromX, fromY float64, s polyStep, nmPerUnit float64) []*geom.Point {
	a0 := math.Atan2(fromY-s.cy, fromX-s.cx)
	a1 := math.Atan2(s.y-s.cy, s.x-s.cx)
	r := math.Hypot(fromX-s.cx, fromY-s.cy)
	// Normalize the sweep to the arc direction.
	if s.cw {
		for a1 >= a0 {
			a1 -= 2 * math.Pi
		}
	} else {
		for a1 <= a0 {
			a1 += 2 * math.Pi
		}
	}
	pts := make([]*geom.Point, 0, arcSegments)
	for i := 1; i <= arcSegments; i++ {
		a := a0 + (a1-a0)*float64(i)/arcSegments
		pts = append(pts, &geom.Point{
			X: rnd((s.cx + r*math.Cos(a)) * nmPerUnit),
			Y: rnd((s.cy + r*math.Sin(a)) * nmPerUnit),
		})
	}
	return pts
}

func isVia(platingStatus string) bool {
	return platingStatus == "VIA"
}

func rnd(v float64) int64 { return int64(math.Round(v)) }

func maxI64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minI64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

// attr returns the value of a start element's attribute by local name, or "".
func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func attrF(e xml.StartElement, name string) float64 {
	return parseFloat(attr(e, name))
}

// padstackIndex is the board's padstacks rebuilt from its per-layer lands, and which entry each pin
// and via uses (agni 1356). IPC-2581 can define a padstack in a PadStackDef, but Allegro's export
// names one on each land and defines none. So each copper layer's lands are gathered by the
// padstack they name. A bottom part's lands go into an entry of their own, since its padstack is
// turned over.
type padstackIndex struct {
	entries []*geom.Padstack
	pin     map[[2]string]string // (ref-des, pin) -> entry id
	via     map[string]string    // padKey(net, at) -> entry id
}

func (f *ipcGeomFile) padstacks(nmPerUnit float64, prims map[string]primShape) *padstackIndex {
	ix := &padstackIndex{pin: map[[2]string]string{}, via: map[string]string{}}
	bottom := map[string]bool{}
	pkgOf := map[string]string{}
	for _, c := range f.Comps {
		bottom[c.RefDes] = normalizeSide(c.LayerRef) == "B.Cu"
		pkgOf[c.RefDes] = c.PackageRef
	}
	// A layer's land names its primitive with the offset already applied to its location, while the
	// package's pin names the primitive that carries the offset in its Xform (agni 1407). A pin's
	// land takes its offset and outline from the package pin's primitive.
	pins := f.packagePins()
	pinPrim := func(ref, pin string) (primShape, bool) {
		for _, pp := range pins[pkgOf[ref]] {
			if pp.Number == pin {
				ps, ok := prims[pp.primID()]
				return ps, ok
			}
		}
		return primShape{}, false
	}
	copper := f.copperLayers()
	names := f.layerNames()
	lands := map[string]map[string]*geom.PadShape{} // entry id -> layer -> land
	byID := map[string]*geom.Padstack{}
	for _, lf := range f.Features {
		if !copper[lf.LayerRef] {
			continue
		}
		layer := names.of(lf.LayerRef)
		for _, s := range lf.Sets {
			for _, pads := range [][]cPadEl{s.Pads, s.FeatPads} {
				for _, pd := range pads {
					if pd.Padstack == "" {
						continue
					}
					id := pd.Padstack
					if bottom[pd.PinRef.ComponentRef] {
						id += "@bottom"
					}
					if pd.PinRef.ComponentRef != "" {
						ix.pin[[2]string{pd.PinRef.ComponentRef, pd.PinRef.Pin}] = id
					} else {
						ix.via[padKey(s.Net, ptNm(pd.Location, nmPerUnit))] = id
					}
					if byID[id] == nil {
						byID[id] = &geom.Padstack{Id: id, Name: pd.Padstack}
						lands[id] = map[string]*geom.PadShape{}
						ix.entries = append(ix.entries, byID[id])
					}
					p, ok := prims[pd.primID()]
					if _, have := lands[id][layer]; ok && !have {
						land := &geom.PadShape{Shape: p.shape, Size: &geom.Point{X: p.sizeX, Y: p.sizeY}, Offset: p.offset, Outline: p.outline}
						if pp, ok := pinPrim(pd.PinRef.ComponentRef, pd.PinRef.Pin); ok && pd.PinRef.ComponentRef != "" {
							land.Offset = pp.offset
							if len(pp.outline) > 0 {
								land.Outline = pp.outline
							}
						}
						lands[id][layer] = land
					}
				}
			}
		}
	}
	// Each entry lists its layers in the stackup's order.
	for _, ps := range ix.entries {
		for _, l := range f.Layers {
			if land, ok := lands[ps.Id][names.of(l.Name)]; ok {
				ps.Layers = append(ps.Layers, &geom.PadstackLayer{Layer: names.of(l.Name), Pad: land})
			}
		}
	}
	sort.Slice(ix.entries, func(i, j int) bool { return ix.entries[i].Id < ix.entries[j].Id })
	return ix
}

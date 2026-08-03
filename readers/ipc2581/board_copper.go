package ipc2581

import (
	"encoding/xml"
	"math"
	"sort"
	"strings"
	"strconv"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// arcSegments is the fixed polyline count an arc step is approximated with, matching the
// KiCad outline-arc precedent (C6: copper/outline arcs are bounded polylines, not a native
// arc primitive).
const arcSegments = 16

// Copper XML subset. A LayerFeature groups per-layer feature Sets; each Set names its net and
// carries routed Polylines (tracks), copper Pads, and drilled Holes (vias).
type layerFeatureEl struct {
	LayerRef string  `xml:"layerRef,attr"`
	Sets     []setEl `xml:"Set"`
}

type setEl struct {
	Net      string      `xml:"net,attr"`
	Tracks   []polyPath  `xml:"Features>Polyline"`
	FeatPads []cPadEl    `xml:"Features>Pad"`
	Pads     []cPadEl    `xml:"Pad"`
	Holes    []holeEl    `xml:"Hole"`
	Fills    []contourEl `xml:"Features>Contour"` // copper plane/pour: a filled region, authored outline (C6)
}

type cPadEl struct {
	Location ptEl      `xml:"Location"`
	PrimRef  primRefEl `xml:"StandardPrimitiveRef"`
	UserRef  primRefEl `xml:"UserPrimitiveRef"`
}

// primID returns the pad's primitive id, preferring a standard-dictionary reference and falling
// back to a user-dictionary one (both resolve through the same prims map).
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

// dictLineDesc is a line-descriptor dictionary: trace widths referenced by id from a track's
// LineDescRef (the width def half of the def/instance split, mirroring the padstack dict).
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

// polyStep is one ordered step of a polyline: a destination point, and when it is an arc, the
// arc center and direction. Interleaved segment/curve order is significant, so the steps are
// decoded in document order (see polyPath.UnmarshalXML) rather than into per-kind slices.
type polyStep struct {
	x, y   float64
	isArc  bool
	cx, cy float64
	cw     bool
}

// polyPath is a track/outline polyline decoded in document order: a start point, ordered
// steps, and the id of the LineDesc that sets its width.
type polyPath struct {
	x, y    float64
	steps   []polyStep
	lineRef string
}

// UnmarshalXML reads a Polyline's children in order so straight (PolyStepSegment) and arc
// (PolyStepCurve) steps keep their interleaving; encoding/xml's default slice decoding would
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
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

// copperNets groups routed copper by net into NetCopper: tracks flattened to straight
// TrackSegments (arcs approximated), plus vias from drilled holes. Sorted by net name for
// deterministic output, mirroring the KiCad producer.
func (f *ipcGeomFile) copperNets(nmPerUnit float64, prims map[string]primShape) []*geom.NetCopper {
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

	for _, lf := range f.Features {
		layer := normalizeSide(lf.LayerRef)
		for _, s := range lf.Sets {
			if s.Net == "" {
				continue
			}
			nc := get(s.Net)
			for _, tr := range s.Tracks {
				w := widths[tr.lineRef]
				pts := flattenPath(tr, nmPerUnit)
				for i := 0; i+1 < len(pts); i++ {
					nc.Segments = append(nc.Segments, &geom.TrackSegment{A: pts[i], B: pts[i+1], Width: w, Layer: layer})
				}
			}
			for _, h := range s.Holes {
				if !isVia(h.PlatingStatus) {
					continue
				}
				at := ptNm(ptEl{X: h.X, Y: h.Y}, nmPerUnit)
				drill := lenNm(h.Diameter, nmPerUnit)
				size := padSize[padKey(s.Net, at)]
				if size == 0 {
					size = drill // no co-located pad: annular reads as zero ring, still a placed via
				}
				via := &geom.Via{At: at, Size: size, Drill: drill}
				if sp, ok := spans[lf.LayerRef]; ok {
					via.LayerFrom, via.LayerTo = normalizeSide(sp[0]), normalizeSide(sp[1])
				}
				nc.Vias = append(nc.Vias, via)
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
// (CONDUCTOR or PLANE). Only these carry pours/planes that become geom.Zones; DOCUMENT (fab,
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

// layerSpans indexes each drill layer's Span (fromLayer,toLayer) so a via records the copper
// layers it bridges — a through via spans the outer pair, blind/buried vias a narrower one.
func (f *ipcGeomFile) layerSpans() map[string][2]string {
	m := map[string][2]string{}
	for _, l := range f.Layers {
		if l.Span != nil {
			m[l.Name] = [2]string{l.Span.From, l.Span.To}
		}
	}
	return m
}

// zones emits each Set's copper plane/pour fill (a Set-direct Contour) as a geom.Zone: net, layer,
// and the authored outline polygon. Cutouts (holes in the fill) are dropped per the C6 zone bound —
// consumers regenerate the fill or a later tier carries it. Sorted for a deterministic artifact.
func (f *ipcGeomFile) zones(nmPerUnit float64) []*geom.Zone {
	copper := f.copperLayers()
	var out []*geom.Zone
	for _, lf := range f.Features {
		if !copper[lf.LayerRef] {
			continue // a Zone is copper; DOCUMENT/mask/silk Contours are drawing geometry, not pours
		}
		layer := normalizeSide(lf.LayerRef)
		for _, s := range lf.Sets {
			for _, c := range s.Fills {
				pl := polyline(c.Polygon, nmPerUnit)
				if pl == nil {
					continue
				}
				out = append(out, &geom.Zone{Net: s.Net, Layer: layer, Outline: pl})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Net < out[j].Net })
	return out
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

// copperPadIndex indexes copper pad radii by (net, location) so a via's annular pad size is
// the co-located copper pad; the drilled Hole and its copper land are separate features that
// share a net and position.
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

// arcPoints approximates one arc step (from the current point to s.x,s.y about s.cx,s.cy) as
// arcSegments chords, walking the shorter or longer way per the clockwise flag.
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

package render

import (
	"encoding/binary"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// pickBoard is a small board with one part on each side, a track, a via, a zone, a numberless pad
// and a designator carrying a quote, so every keyed element appears at least once.
func pickBoard() *geom.BoardGeometry {
	pt := func(x, y int64) *geom.Point { return &geom.Point{X: x, Y: y} }
	pad := func(n, net string, x int64, layer string) *geom.Pad {
		return &geom.Pad{Number: n, Net: net, At: pt(x, 0), Size: pt(1_000_000, 1_000_000), Shape: "rect", Layers: []string{layer}}
	}
	return &geom.BoardGeometry{
		Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{pt(0, 0), pt(40_000_000, 0), pt(40_000_000, 20_000_000), pt(0, 20_000_000), pt(0, 0)}}}},
		Placements: []*geom.ComponentPlacement{
			{RefDes: `R"1`, At: pt(10_000_000, 10_000_000), Layer: "F.Cu", Pads: []*geom.Pad{pad("1", "VCC", -1_000_000, "F.Cu"), pad("2", "SDA", 1_000_000, "F.Cu")}},
			{RefDes: "C9", At: pt(30_000_000, 10_000_000), Layer: "B.Cu", Mirror: true, Pads: []*geom.Pad{pad("1", "GND", -1_000_000, "B.Cu"), pad("", "", 1_000_000, "B.Cu")}},
		},
		Nets: []*geom.NetCopper{{
			Net:      "SDA",
			Segments: []*geom.TrackSegment{{A: pt(11_000_000, 10_000_000), B: pt(20_000_000, 10_000_000), Width: 250_000, Layer: "F.Cu"}},
			Vias:     []*geom.Via{{At: pt(20_000_000, 10_000_000), Size: 600_000, Drill: 300_000}},
		}},
		Zones:    []*geom.Zone{{Net: "GND", Layer: "B.Cu", Outline: &geom.Polyline{Points: []*geom.Point{pt(25_000_000, 5_000_000), pt(35_000_000, 5_000_000), pt(35_000_000, 15_000_000)}}}},
		Graphics: []*geom.BoardGraphic{{RefDes: "C9", Layer: "B.SilkS", Shape: &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{pt(29_000_000, 9_000_000), pt(31_000_000, 9_000_000)}}}},
		Texts:    []*geom.BoardText{{Text: "C9", RefDes: "C9", At: pt(30_000_000, 12_000_000), Layer: "B.SilkS", Kind: "reference"}},
	}
}

var keyedEl = regexp.MustCompile(`<(\w+)[^>]*data-kind="(\w+)"[^>]*>`)

// keyedElements returns each keyed element as "tag kind" followed by its data-* attributes.
func keyedElements(doc string) []string {
	attr := regexp.MustCompile(`data-(ref|pin|net)="([^"]*)"`)
	var out []string
	for _, m := range keyedEl.FindAllStringSubmatch(doc, -1) {
		s := m[1] + " " + m[2]
		for _, a := range attr.FindAllStringSubmatch(m[0], -1) {
			s += " " + a[1] + "=" + a[2]
		}
		out = append(out, s)
	}
	return out
}

func TestBoardPickTargetsKeyEveryPickableElement(t *testing.T) {
	got := keyedElements(BoardSVG(pickBoard(), WithPickTargets()))
	if len(got) == 0 {
		t.Fatal("no keyed elements at all")
	}
	want := []string{
		"polygon net net=GND",                // the zone
		"rect component ref=R&#34;1",         // R"1's body, escaped
		"rect component ref=C9",              // C9's body
		"g net net=SDA",                      // the track's group
		"rect pin ref=R&#34;1 pin=1 net=VCC", // a front pad
		"rect pin ref=R&#34;1 pin=2 net=SDA", // a front pad
		"rect pin ref=C9 pin=1 net=GND",      // a back pad
		"rect component ref=C9",              // the numberless pad keys as its part
		"circle net net=SDA",                 // via ring
		"circle net net=SDA",                 // via drill, drawn over the ring
	}
	for _, w := range want {
		i := indexOf(got, w)
		if i < 0 {
			t.Errorf("missing keyed element %q in\n%s", w, strings.Join(got, "\n"))
			continue
		}
		got = append(got[:i], got[i+1:]...)
	}
	if len(got) > 0 {
		t.Errorf("unexpected keyed elements:\n%s", strings.Join(got, "\n"))
	}
}

func indexOf(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// TestBoardPickTargetsAreOptIn holds `agni render` and a report to an unkeyed board.
func TestBoardPickTargetsAreOptIn(t *testing.T) {
	doc := BoardSVG(pickBoard())
	if strings.Contains(doc, "data-kind") || strings.Contains(doc, "picks-") {
		t.Error("a board rendered without WithPickTargets must carry no pick keys")
	}
	if strings.Contains(BoardSVG(boardFixtureGeometry(t)), "data-kind") {
		t.Error("the fixture board must carry no pick keys without WithPickTargets")
	}
}

// TestBoardFootprintPicksSplitBySide puts each part's body rect in its side's group, so the layer
// CSS that hides a side also hides its parts from the picker.
func TestBoardFootprintPicksSplitBySide(t *testing.T) {
	doc := BoardSVG(pickBoard(), WithPickTargets())
	group := func(cls string) string {
		i := strings.Index(doc, `<g class="`+cls+`"`)
		if i < 0 {
			t.Fatalf("no %s group", cls)
		}
		return doc[i : i+strings.Index(doc[i:], "</g>")]
	}
	if g := group("picks-back"); !strings.Contains(g, `data-ref="C9"`) || strings.Contains(g, `data-ref="R&#34;1"`) {
		t.Errorf("picks-back must hold C9 alone:\n%s", g)
	}
	if g := group("picks-front"); !strings.Contains(g, `data-ref="R&#34;1"`) || strings.Contains(g, `data-ref="C9"`) {
		t.Errorf("picks-front must hold R\"1 alone:\n%s", g)
	}
	// Above the zones and under the copper, so a pad or track drawn over a body still wins.
	if !(strings.Index(doc, "zones-back") < strings.Index(doc, "picks-back") && strings.Index(doc, "picks-front") < strings.Index(doc, `class="copper-`)) {
		t.Error("footprint picks must draw after the zones and before the copper")
	}
}

// TestBoardPickRectCoversItsPads checks a body rect spans its pads, the area a click on the part
// lands in.
func TestBoardPickRectCoversItsPads(t *testing.T) {
	doc := BoardSVG(pickBoard(), WithPickTargets())
	re := regexp.MustCompile(`<rect x="([\d.]+)" y="([\d.]+)" width="([\d.]+)" height="([\d.]+)" fill="none" pointer-events="all" data-kind="component" data-ref="C9"`)
	m := re.FindStringSubmatch(doc)
	if m == nil {
		t.Fatal("no body rect for C9")
	}
	// C9's pads are 1mm squares 2mm apart, so the body spans 3mm by 1mm at the board's scale.
	fr := frameBoard(pickBoard())
	w, h := parseF(t, m[3]), parseF(t, m[4])
	if wantW, wantH := 3_000_000*fr.scale, 1_000_000*fr.scale; math.Abs(w-wantW) > 0.2 || math.Abs(h-wantH) > 0.2 {
		t.Errorf("C9 body is %.1f x %.1f px, want %.1f x %.1f", w, h, wantW, wantH)
	}
}

func parseF(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestBoardArtworkPassesClicksThrough holds the silkscreen and the designators unkeyed and
// transparent to the pointer, so a designator lying over a neighbour's pad leaves the pad to pick
// its own pin (agni issue 1347).
func TestBoardArtworkPassesClicksThrough(t *testing.T) {
	doc := BoardSVG(pickBoard(), WithPickTargets())
	for cls, holds := range map[string]string{"silk": "<polyline", "labels": ">C9</text>"} {
		open, body := svgGroup(t, doc, cls)
		if !strings.Contains(open, `pointer-events="none"`) {
			t.Errorf("%s group must pass clicks through: %s", cls, open)
		}
		if !strings.Contains(body, holds) {
			t.Fatalf("%s group holds no %s, so the test checks nothing", cls, holds)
		}
		if strings.Contains(body, "data-kind") {
			t.Errorf("%s group must carry no pick keys", cls)
		}
	}
	if strings.Contains(BoardSVG(pickBoard()), "pointer-events") {
		t.Error("an unkeyed board must not set pointer-events")
	}
}

// svgGroup returns the opening tag of the <g> with class cls and everything up to its matching
// </g>, counting nested groups.
func svgGroup(t *testing.T, doc, cls string) (open, body string) {
	t.Helper()
	start := strings.Index(doc, `<g class="`+cls+`"`)
	if start < 0 {
		t.Fatalf("no %s group", cls)
	}
	open = doc[start : start+strings.Index(doc[start:], ">")+1]
	depth := 0
	for i := start; i < len(doc); i++ {
		switch {
		case strings.HasPrefix(doc[i:], "<g ") || strings.HasPrefix(doc[i:], "<g>"):
			depth++
		case strings.HasPrefix(doc[i:], "</g>"):
			depth--
			if depth == 0 {
				return open, doc[start:i]
			}
		}
	}
	t.Fatalf("%s group never closes", cls)
	return "", ""
}

// PackBoard packs each footprint's pick area as BoardSVG draws it, in the pick group of its side,
// so a WebGL click between a part's pads picks the part (agni issue 1332).
func TestPackBoardPacksEachFootprintsPickArea(t *testing.T) {
	b := pickBoard()
	p := PackBoard(b)
	recs := decodeRecords(p.GetPrimitives())
	verts := p.GetVertices()
	vertex := func(i uint32) (int64, int64) {
		x := int32(binary.LittleEndian.Uint32(verts[i*8:]))
		y := int32(binary.LittleEndian.Uint32(verts[i*8+4:]))
		return int64(x) + p.GetOriginX(), int64(y) + p.GetOriginY()
	}
	want := map[string]uint8{`R"1`: groupBoardPickFront, "C9": groupBoardPickBack}
	found := map[string]bool{}
	for _, k := range p.GetKeys() {
		r := recs[k.GetPrimitive()]
		if r.group != groupBoardPickFront && r.group != groupBoardPickBack {
			continue
		}
		ref := k.GetRefDes()
		if want[ref] != r.group {
			t.Errorf("%s's pick area is in group %d, want %d", ref, r.group, want[ref])
		}
		if k.GetNet() != "" || k.GetPin() != "" {
			t.Errorf("%s's pick area names a net or a pin: %v", ref, k)
		}
		var pl *geom.ComponentPlacement
		for _, q := range b.GetPlacements() {
			if q.GetRefDes() == ref {
				pl = q
			}
		}
		minX, minY, maxX, maxY := footprintBody(pl)
		x0, y0 := vertex(r.firstVertex)
		x1, y1 := vertex(r.firstVertex + 2)
		if r.count != 4 || x0 != minX || y0 != minY || x1 != maxX || y1 != maxY {
			t.Errorf("%s's pick area is (%d,%d)-(%d,%d) over %d vertices, want BoardSVG's (%d,%d)-(%d,%d)", ref, x0, y0, x1, y1, r.count, minX, minY, maxX, maxY)
		}
		found[ref] = true
	}
	if len(found) != len(want) {
		t.Fatalf("pick areas for %v, want %v", found, want)
	}
	// A component highlight lights the part's copper and artwork, and never its undrawn pick area.
	hl := HighlightPacked(p, []*geom.HighlightSpec{{Components: []string{"C9"}}})
	if len(hl.GetGroups()) != 1 || len(hl.GetGroups()[0].GetPrimitives()) == 0 {
		t.Fatalf("C9 lit nothing: %v", hl)
	}
	for _, i := range hl.GetGroups()[0].GetPrimitives() {
		if g := recs[i].group; g == groupBoardPickFront || g == groupBoardPickBack {
			t.Errorf("a highlight lit C9's pick area, primitive %d", i)
		}
	}
	// Each side's copper row hides its side's pick areas.
	rows := BoardLayers(b)
	hides := func(layer string, g uint8) bool {
		for _, r := range rows {
			if r.GetName() == layer {
				for _, rg := range r.GetGroups() {
					if rg == uint32(g) {
						return true
					}
				}
			}
		}
		return false
	}
	if !hides("F.Cu", groupBoardPickFront) || !hides("B.Cu", groupBoardPickBack) {
		t.Errorf("the outer copper rows do not hide their sides' pick areas: %v", rows)
	}
}

// PackBoard packs in BoardSVG's paint order, back layer first, a layer's pours under its tracks and
// the edge over the copper. Net A's front track is listed before net B's back copper, and still
// packs after it, so WebGL draws it on top and the canvas picker takes it (agni issue 1332).
func TestPackBoardPacksInPaintOrder(t *testing.T) {
	pt := func(x, y int64) *geom.Point { return &geom.Point{X: x, Y: y} }
	b := &geom.BoardGeometry{
		Layers:  []*geom.BoardLayer{{Name: "F.Cu", Kind: "signal", Number: 0}, {Name: "B.Cu", Kind: "signal", Number: 31}},
		Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{pt(0, 0), pt(100, 0), pt(100, 100)}}}},
		Nets: []*geom.NetCopper{
			{Net: "A", Segments: []*geom.TrackSegment{{A: pt(10, 50), B: pt(90, 50), Width: 4, Layer: "F.Cu"}}},
			{Net: "B",
				Segments: []*geom.TrackSegment{{A: pt(50, 10), B: pt(50, 90), Width: 4, Layer: "B.Cu"}},
				Shapes: []*geom.CopperShape{{Layer: "B.Cu", Outline: &geom.Polyline{Points: []*geom.Point{pt(0, 0), pt(100, 0), pt(100, 100), pt(0, 100)}}}}},
		},
	}
	p := PackBoard(b)
	recs := decodeRecords(p.GetPrimitives())
	first := map[string]int{}
	for _, k := range p.GetKeys() {
		r := recs[k.GetPrimitive()]
		key := k.GetNet() + "/" + map[uint8]string{groupBoardCopperFront: "front", groupBoardCopperBack: "back"}[r.group]
		if r.group != groupBoardCopperFront && r.group != groupBoardCopperBack {
			key = k.GetNet() + "/pour"
		}
		if _, ok := first[key]; !ok {
			first[key] = int(k.GetPrimitive())
		}
	}
	edge := -1
	for i, r := range recs {
		if r.group == groupBoardEdge {
			edge = i
		}
	}
	pour, backTrack, frontTrack := first["B/pour"], first["B/back"], first["A/front"]
	if len(first) != 3 || edge < 0 {
		t.Fatalf("packed %v and the edge at %d, want B's pour, B's back track, A's front track and an edge", first, edge)
	}
	if !(pour < backTrack && backTrack < frontTrack && frontTrack < edge) {
		t.Errorf("paint order is pour %d, back track %d, front track %d, edge %d, want each after the last", pour, backTrack, frontTrack, edge)
	}
}

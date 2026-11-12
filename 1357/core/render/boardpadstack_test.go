package render

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// stackBoard is a four-layer board with one through-hole pin on net SIG and one on GND. Its
// padstack has a 2 mm land outside, a 1.2 mm land on In1.Cu, and on the In2.Cu plane a 1.6 mm
// antipad. The plane carries GND, so SIG's pin clears it and GND's pin joins it.
func stackBoard() *geom.BoardGeometry {
	circle := func(d int64) *geom.PadShape { return &geom.PadShape{Shape: "circle", Size: &geom.Point{X: d, Y: d}} }
	pad := func(n, net string, x int64) *geom.Pad {
		return &geom.Pad{Number: n, At: &geom.Point{X: x, Y: 0}, Shape: "circle", Size: &geom.Point{X: 2_000_000, Y: 2_000_000},
			Layers: []string{"*.Cu"}, Drill: 800_000, Net: net, Padstack: "TH20"}
	}
	return &geom.BoardGeometry{
		UnitNm: 1,
		Layers: []*geom.BoardLayer{{Number: 0, Name: "F.Cu", Kind: "signal"}, {Number: 1, Name: "In1.Cu", Kind: "signal"}, {Number: 2, Name: "In2.Cu", Kind: "power"}, {Number: 3, Name: "B.Cu", Kind: "signal"}},
		Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{{X: -5_000_000, Y: -5_000_000}, {X: 15_000_000, Y: -5_000_000}, {X: 15_000_000, Y: 5_000_000}, {X: -5_000_000, Y: 5_000_000}, {X: -5_000_000, Y: -5_000_000}}}}},
		Padstacks: []*geom.Padstack{{Id: "TH20", Name: "TH20", Layers: []*geom.PadstackLayer{
			{Layer: "F.Cu", Pad: circle(2_000_000)},
			{Layer: "In1.Cu", Pad: circle(1_200_000)},
			{Layer: "In2.Cu", Pad: circle(1_200_000), Antipad: circle(1_600_000)},
			{Layer: "B.Cu", Pad: circle(2_000_000)},
		}}},
		Placements: []*geom.ComponentPlacement{{RefDes: "J1", At: &geom.Point{}, Layer: "F.Cu", Pads: []*geom.Pad{pad("1", "SIG", 0), pad("2", "GND", 10_000_000)}}},
		Zones: []*geom.Zone{{Net: "GND", Layer: "In2.Cu", Outline: &geom.Polyline{Points: []*geom.Point{{X: -4_000_000, Y: -4_000_000}, {X: 14_000_000, Y: -4_000_000}, {X: 14_000_000, Y: 4_000_000}, {X: -4_000_000, Y: 4_000_000}}}}},
	}
}

// layerGroup returns the markup of one copper layer's group in a BoardSVG document.
func layerGroup(t *testing.T, doc, layer string) string {
	t.Helper()
	i := strings.Index(doc, `data-layer="`+layer+`">`)
	if i < 0 {
		t.Fatalf("no group for %s", layer)
	}
	j := strings.Index(doc[i:], "</g>\n<g class=")
	if j < 0 {
		j = len(doc) - i
	}
	return doc[i : i+j]
}

func TestAPadstackDrawsEachLayersOwnLand(t *testing.T) {
	b := stackBoard()
	doc := BoardSVG(b, WithPickTargets())
	radii := func(group string) []float64 {
		var out []float64
		for _, m := range regexp.MustCompile(`<circle [^>]*r="([0-9.]+)" fill="#`).FindAllStringSubmatch(group, -1) {
			r, _ := strconv.ParseFloat(m[1], 64)
			out = append(out, r)
		}
		return out
	}
	outer, inner := radii(layerGroup(t, doc, "F.Cu")), radii(layerGroup(t, doc, "In1.Cu"))
	if len(outer) != 2 || len(inner) != 2 {
		t.Fatalf("lands: F.Cu %v, In1.Cu %v, want both pins on each", outer, inner)
	}
	if ratio := inner[0] / outer[0]; ratio < 0.59 || ratio > 0.61 {
		t.Errorf("In1.Cu's land is %.2f of F.Cu's, want 0.6 (1.2 mm against 2 mm)", ratio)
	}
	if !strings.Contains(layerGroup(t, doc, "In1.Cu"), `data-kind="pin" data-ref="J1"`) {
		t.Error("an inner land takes no pick keys, so a pin shown on In1.Cu alone cannot be clicked")
	}
	// SIG's pin clears the GND plane and GND's pin joins it, so the plane draws one antipad.
	if n := strings.Count(layerGroup(t, doc, "In2.Cu"), "stroke-dasharray"); n != 1 {
		t.Errorf("In2.Cu draws %d antipads, want 1", n)
	}
	if strings.Count(layerGroup(t, doc, "F.Cu"), "stroke-dasharray") != 0 {
		t.Error("a signal layer draws an antipad")
	}
}

func TestAPadstackPacksEachLandIntoItsLayersGroup(t *testing.T) {
	b := stackBoard()
	rows := BoardLayers(b)
	if got := strings.Join(rowNames(rows), " "); !strings.HasPrefix(got, "F.Cu In1.Cu In2.Cu B.Cu") {
		t.Fatalf("rows %q: a layer only lands use gets no row", got)
	}
	groups := packedGroups(PackBoard(b))
	owned := map[uint32]bool{}
	for _, r := range rows {
		for _, g := range r.GetGroups() {
			owned[g] = true
		}
	}
	for g := range groups {
		if !owned[g] {
			t.Errorf("group %d packs %d primitives that no row hides", g, groups[g])
		}
	}
	in1 := boardRow(t, rows, "In1.Cu").GetGroups()[0]
	if groups[in1] != 2 {
		t.Errorf("In1.Cu's group packs %d primitives, want the two pins' lands", groups[in1])
	}
	// The plane packs both lands and SIG's antipad outline.
	if in2 := boardRow(t, rows, "In2.Cu").GetGroups()[0]; groups[in2] != 3 {
		t.Errorf("In2.Cu's group packs %d primitives, want two lands and one antipad", groups[in2])
	}
}

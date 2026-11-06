package render

import (
	"math"
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
		UnitNm:  1,
		Layers:  []*geom.BoardLayer{{Number: 0, Name: "F.Cu", Kind: "signal"}, {Number: 1, Name: "In1.Cu", Kind: "signal"}, {Number: 2, Name: "In2.Cu", Kind: "power"}, {Number: 3, Name: "B.Cu", Kind: "signal"}},
		Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{{X: -5_000_000, Y: -5_000_000}, {X: 15_000_000, Y: -5_000_000}, {X: 15_000_000, Y: 5_000_000}, {X: -5_000_000, Y: 5_000_000}, {X: -5_000_000, Y: -5_000_000}}}}},
		Padstacks: []*geom.Padstack{{Id: "TH20", Name: "TH20", Layers: []*geom.PadstackLayer{
			{Layer: "F.Cu", Pad: circle(2_000_000)},
			{Layer: "In1.Cu", Pad: circle(1_200_000)},
			{Layer: "In2.Cu", Pad: circle(1_200_000), Antipad: circle(1_600_000)},
			{Layer: "B.Cu", Pad: circle(2_000_000)},
		}}},
		Placements: []*geom.ComponentPlacement{{RefDes: "J1", At: &geom.Point{}, Layer: "F.Cu", Pads: []*geom.Pad{pad("1", "SIG", 0), pad("2", "GND", 10_000_000)}}},
		Zones:      []*geom.Zone{{Net: "GND", Layer: "In2.Cu", Outline: &geom.Polyline{Points: []*geom.Point{{X: -4_000_000, Y: -4_000_000}, {X: 14_000_000, Y: -4_000_000}, {X: 14_000_000, Y: 4_000_000}, {X: -4_000_000, Y: 4_000_000}}}}},
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

// offsetLand is one pad whose F.Cu land sits 0.5 mm off the pad along its own x and turns with it,
// and whose In1.Cu land is a custom triangle around a 0.4 mm offset (agni 1407).
func offsetLand(pl *geom.ComponentPlacement) (*geom.Pad, *geom.PadstackLayer, *geom.PadstackLayer) {
	pad := &geom.Pad{Number: "1", At: &geom.Point{}, Shape: "rect", Size: &geom.Point{X: 2_000_000, Y: 1_000_000}, Layers: []string{"*.Cu"}, Net: "SIG", Padstack: "OFF"}
	front := &geom.PadstackLayer{Layer: "F.Cu", Pad: &geom.PadShape{Shape: "rect", Size: &geom.Point{X: 2_000_000, Y: 1_000_000}, Offset: &geom.Point{X: 500_000}}}
	inner := &geom.PadstackLayer{Layer: "In1.Cu", Pad: &geom.PadShape{Shape: "custom", Size: &geom.Point{X: 600_000, Y: 600_000}, Offset: &geom.Point{X: 400_000},
		Outline: []*geom.Point{{X: -300_000, Y: -300_000}, {X: 300_000, Y: -300_000}, {X: 0, Y: 300_000}}}}
	pl.Pads = []*geom.Pad{pad}
	return pad, front, inner
}

// packed collects what packStackLayer adds.
func packed(pl *geom.ComponentPlacement, pad *geom.Pad, l *geom.PadstackLayer) [][2]int64 {
	var pts [][2]int64
	packStackLayer(func(_, _ uint8, _, _, _ string, p [][2]int64) { pts = append(pts, p...) }, 0, pl, pad, l, false, "J1", "SIG", "1")
	return pts
}

func centroid(pts [][2]int64) (float64, float64) {
	var x, y float64
	for _, p := range pts {
		x, y = x+float64(p[0]), y+float64(p[1])
	}
	return x / float64(len(pts)), y / float64(len(pts))
}

func TestAPackedLandSitsAtItsOffsetAndTurnsWithItsPart(t *testing.T) {
	sin, cos := math.Sincos(math.Pi / 6)
	for _, c := range []struct {
		name   string
		pl     *geom.ComponentPlacement
		cx, cy float64 // the F.Cu land's centre on the board
		long   float64 // the angle its 2 mm side lies at, CCW
		// The custom triangle's centroid, 100 µm below its own centre along the pad's y.
		innerCx, innerCy float64
	}{
		// A part turned 30° CCW turns the land's offset and its long side with it. 30° rather
		// than 90°, since a rectangle turned 90° either way covers the same ground.
		{"turned", &geom.ComponentPlacement{RefDes: "J1", At: &geom.Point{X: 10_000_000}, RotationDeg: 30},
			10_000_000 + 500_000*cos, 500_000 * sin, 30, 10_000_000 + 400_000*cos + 100_000*sin, 400_000*sin - 100_000*cos},
		// A part on the bottom mirrors the offset onto -x.
		{"mirrored", &geom.ComponentPlacement{RefDes: "J1", At: &geom.Point{}, Mirror: true}, -500_000, 0, 0, -400_000, -100_000},
	} {
		t.Run(c.name, func(t *testing.T) {
			pad, front, inner := offsetLand(c.pl)
			quad := packed(c.pl, pad, front)
			if len(quad) != 6 {
				t.Fatalf("F.Cu packs %d vertices, want a quad's 6", len(quad))
			}
			if x, y := centroid(quad); math.Abs(x-c.cx) > 2 || math.Abs(y-c.cy) > 2 {
				t.Errorf("F.Cu land centred at (%.0f, %.0f) nm, want (%.0f, %.0f)", x, y, c.cx, c.cy)
			}
			// The quad's first edge runs along the land's 2 mm side.
			dx, dy := float64(quad[1][0]-quad[0][0]), float64(quad[1][1]-quad[0][1])
			if a := math.Mod(math.Atan2(dy, dx)*180/math.Pi+360, 180); math.Abs(a-c.long) > 0.01 || math.Abs(math.Hypot(dx, dy)-2_000_000) > 2 {
				t.Errorf("F.Cu land's long side lies at %.2f°, %.0f nm long, want %.0f°, 2 mm", a, math.Hypot(dx, dy), c.long)
			}
			tri := packed(c.pl, pad, inner)
			if len(tri) != 3 {
				t.Fatalf("the custom land packs %d vertices, want its triangle's 3", len(tri))
			}
			if x, y := centroid(tri); math.Abs(x-c.innerCx) > 2 || math.Abs(y-c.innerCy) > 2 {
				t.Errorf("the custom land's centroid is (%.0f, %.0f) nm, want (%.0f, %.0f)", x, y, c.innerCx, c.innerCy)
			}
		})
	}
}

func TestASVGLandDrawsItsOutlineAndOffset(t *testing.T) {
	b := stackBoard()
	pl := &geom.ComponentPlacement{RefDes: "J1", At: &geom.Point{}, RotationDeg: 90, Layer: "F.Cu"}
	pad, front, inner := offsetLand(pl)
	pad.Drill = 400_000
	b.Placements = []*geom.ComponentPlacement{pl}
	b.Padstacks = []*geom.Padstack{{Id: "OFF", Layers: []*geom.PadstackLayer{front, inner}}}
	doc := BoardSVG(b, WithPickTargets())
	if n := strings.Count(layerGroup(t, doc, "In1.Cu"), "<polygon "); n != 1 {
		t.Errorf("In1.Cu draws %d polygons, want the custom land's one", n)
	}
	// The part turns 90°, so the land's offset points up the page from the hole.
	rect := regexp.MustCompile(`<rect x="([-0-9.]+)" y="([-0-9.]+)" width="([0-9.]+)" height="([0-9.]+)"[^>]*rotate\(-90`).FindStringSubmatch(layerGroup(t, doc, "F.Cu"))
	hole := regexp.MustCompile(`<circle cx="([-0-9.]+)" cy="([-0-9.]+)"`).FindStringSubmatch(doc[strings.Index(doc, `data-layer="F.Cu">`):])
	if rect == nil || hole == nil {
		t.Fatalf("F.Cu has no land turned by -90 (%v) or no hole (%v)", rect != nil, hole != nil)
	}
	f := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	cx, cy := f(rect[1])+f(rect[3])/2, f(rect[2])+f(rect[4])/2
	if dx, dy := cx-f(hole[1]), cy-f(hole[2]); math.Abs(dx) > 0.2 || dy > -1 {
		t.Errorf("the land's centre is (%.1f, %.1f) px from the hole, want straight up the page", dx, dy)
	}
}

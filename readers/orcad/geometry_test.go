package orcad

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// clayDir is Clay R6 in the cadence-exports samples artifact.
const clayDir = "../../tools/samples/cadence/clay-r6"

func clayDesigns(t *testing.T) []*dsn {
	t.Helper()
	var out []*dsn
	err := filepath.WalkDir(clayDir, func(p string, e os.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(strings.ToUpper(p), ".DSN") {
			return err
		}
		b, err := os.ReadFile(p) // #nosec G304 -- a fixed path in the samples corpus
		if err != nil {
			return err
		}
		d, err := load(b, p)
		if err != nil {
			return err
		}
		out = append(out, d)
		return nil
	})
	if err != nil {
		t.Fatalf("%v; run `make samples-oracle`", err)
	}
	if len(out) != 11 {
		t.Fatalf("read %d Clay designs, want the top and its ten blocks; run `make samples-oracle`", len(out))
	}
	return out
}

// TestEveryPlacedPinLandsWhereThePageStoresIt holds the placement transform to Clay R6 itself. A
// page stores each placed pin's position, and the symbol stores its connection point, so turning
// the symbol by the placement's transform must put each connection point on its stored position.
// The transform is applied by geomath.PlacePin, the function the renderer uses.
func TestEveryPlacedPinLandsWhereThePageStoresIt(t *testing.T) {
	checked, turned := 0, 0
	for _, d := range clayDesigns(t) {
		for _, pages := range d.folders {
			for _, pg := range pages {
				for _, part := range pg.parts {
					lp := d.cache.parts[part.cacheKey]
					if lp == nil {
						t.Errorf("%s: %s places %s, which the cache lacks", filepath.Base(d.name), part.ref, part.cacheKey)
						continue
					}
					tr := placeTransform(part.at, part.orient, lp.draw.width(), lp.draw.height())
					for _, pin := range part.pins {
						i := int(pin.index)
						if i < 0 {
							i = -i
						}
						var sp *symPin
						for k := range lp.pins {
							if lp.pins[k].slot == i-1 {
								sp = &lp.pins[k]
							}
						}
						if sp == nil {
							t.Errorf("%s: %s pin index %d has no symbol pin", filepath.Base(d.name), part.ref, pin.index)
							continue
						}
						got := geomath.PlacePin(tr, &geom.PinPoint{Loc: gp(sp.hot)})
						if want := gp(pin.at); got.GetX() != want.GetX() || got.GetY() != want.GetY() {
							t.Errorf("%s: %s pin %s at orientation %d lands at %v, and the page stores %v",
								filepath.Base(d.name), part.ref, sp.name, part.orient, got, want)
						}
						checked++
						if part.orient&3 != 0 {
							turned++
						}
					}
				}
			}
		}
	}
	if checked != 573 || turned < 100 {
		t.Errorf("checked %d pins, %d of them turned; want Clay's 573, with every orientation among them", checked, turned)
	}
}

// TestEveryPowerSymbolAndPortConnectsAtAWireEnd holds the placement of a power symbol or a port,
// which the page stores no pin for. Its symbol's connection point, placed from the corner of its
// stored box, must be the end of a wire or a placed pin on its page.
func TestEveryPowerSymbolAndPortConnectsAtAWireEnd(t *testing.T) {
	checked := 0
	for _, d := range clayDesigns(t) {
		for _, pages := range d.folders {
			for _, pg := range pages {
				ends := map[point]bool{}
				for _, w := range pg.wires {
					ends[w.a], ends[w.b] = true, true
				}
				for _, part := range pg.parts {
					for _, pin := range part.pins {
						ends[pin.at] = true
					}
				}
				for _, s := range pg.symbols {
					cs := d.cache.symbols[s.symbol]
					if cs == nil || len(cs.pins) == 0 {
						continue
					}
					tr := placeTransform(s.lo, s.orient, cs.draw.width(), cs.draw.height())
					p := geomath.PlacePin(tr, &geom.PinPoint{Loc: gp(cs.pins[0].hot)})
					if at := (point{int32(p.GetX()), int32(-p.GetY())}); !ends[at] {
						t.Errorf("%s: %s connects at %v, which no wire or pin ends at", filepath.Base(d.name), s.name, at)
					}
					checked++
				}
			}
		}
	}
	if checked != 97+141 {
		t.Errorf("checked %d symbols, want Clay's 97 power symbols and 141 ports", checked)
	}
}

// The turns of a placement agree with the four corner moves the transform is written from.
func TestATurnedPlacementMovesItsCorner(t *testing.T) {
	const w, h = 20, 40
	hot := point{30, 10}
	for orient, want := range map[byte]point{0: {130, 110}, 1: {110, 90}, 2: {90, 130}, 3: {130, 130}} {
		got := geomath.PlacePin(placeTransform(point{100, 100}, orient, w, h), &geom.PinPoint{Loc: gp(hot)})
		if got.GetX() != int64(want.x) || got.GetY() != -int64(want.y) {
			t.Errorf("orientation %d: %v, want (%d, %d) on the Y-up page", orient, got, want.x, -want.y)
		}
	}
}

func TestCapturesOverbarsBecomeKiCadMarkup(t *testing.T) {
	for in, want := range map[string]string{
		`C\S\`:      "~{CS}",
		`H\O\L\D\`:  "~{HOLD}",
		`EN_B`:      "EN_B",
		`R\E\SET_N`: "~{RE}SET_N",
		`\X`:        `\X`,
	} {
		if got := overbars(in); got != want {
			t.Errorf("overbars(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestABlockPinNameRunsIntoTheBlockOnEveryEdge renders one pin name on each edge of a block through
// the SVG renderer and reads where the text runs, by SVG's own rotate and text-anchor. A name on the
// top or bottom edge once ran out of the block, Clay's uC block among them (agni issue 1413).
func TestABlockPinNameRunsIntoTheBlockOnEveryEdge(t *testing.T) {
	lo, hi := point{100, 100}, point{300, 300} // Capture's y grows down the page
	pins := map[string]point{"LEFT": {100, 200}, "RIGHT": {300, 200}, "TOP": {200, 100}, "BOTTOM": {200, 300}}
	sheet := &geom.SheetGeometry{Size: &geom.BBox{Min: &geom.Point{X: 0, Y: -400}, Max: &geom.Point{X: 400, Y: 0}}}
	for name, at := range pins {
		sheet.Labels = append(sheet.Labels, blockPinLabel(name, at, lo, hi, 10))
	}
	out := render.SheetSVG(&geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{sheet}}, sheet)
	text := regexp.MustCompile(`<text ([^>]*)>([A-Z]+)<`)
	anchorOf := regexp.MustCompile(`text-anchor="(\w+)"`)
	rotateOf := regexp.MustCompile(`rotate\(([-0-9.]+) `)
	// inward is the direction from the pin into the block, in the SVG's own frame (y down).
	inward := map[string][2]float64{"LEFT": {1, 0}, "RIGHT": {-1, 0}, "TOP": {0, 1}, "BOTTOM": {0, -1}}
	seen := 0
	for _, m := range text.FindAllStringSubmatch(out, -1) {
		name, attrs := m[2], m[1]
		want, ok := inward[name]
		if !ok {
			continue
		}
		seen++
		deg := 0.0
		if r := rotateOf.FindStringSubmatch(attrs); r != nil {
			deg, _ = strconv.ParseFloat(r[1], 64)
		}
		anchor := "start"
		if a := anchorOf.FindStringSubmatch(attrs); a != nil {
			anchor = a[1]
		}
		rad := deg * math.Pi / 180
		dir := [2]float64{math.Cos(rad), math.Sin(rad)} // the run's reading direction
		switch anchor {
		case "end":
			dir = [2]float64{-dir[0], -dir[1]}
		case "middle":
			t.Errorf("%s is centred on its pin, so half of it runs out of the block", name)
			continue
		}
		if dot := dir[0]*want[0] + dir[1]*want[1]; dot < 0.99 {
			t.Errorf("%s runs along (%.0f, %.0f) from its pin, want into the block along (%.0f, %.0f)", name, dir[0], dir[1], want[0], want[1])
		}
	}
	if seen != len(pins) {
		t.Fatalf("found %d of %d pin names in the SVG:\n%s", seen, len(pins), out)
	}
}

// TestEveryNotConnectedPinOnClayGetsACross counts the pins Clay's pages flag not connected, which
// Capture draws a cross on (agni issue 1413). pstxnet.dat gathers seven pins onto its NC net. The
// WiFi block flags an eighth that names a page net of its own, so Capture nets it and still draws
// the cross.
func TestEveryNotConnectedPinOnClayGetsACross(t *testing.T) {
	flagged, netless := 0, 0
	for _, d := range clayDesigns(t) {
		for _, pages := range d.folders {
			for _, pg := range pages {
				flagged += len(noConnects(pg))
				for _, part := range pg.parts {
					for _, pin := range part.pins {
						if pin.index < 0 && pin.net == 0 {
							netless++
						}
					}
				}
			}
		}
	}
	if flagged != 8 || netless != 7 {
		t.Errorf("Clay's pages flag %d pins not connected, %d naming no net; want 8, with the 7 on pstxnet.dat's NC net", flagged, netless)
	}
}

func TestANotConnectedPinDrawsACrossOnItsPoint(t *testing.T) {
	pg := &page{
		netName: map[uint32][]string{},
		parts:   []placedPart{resistor(1, "R1", point{40, 60}, point{40, 110})},
	}
	pg.parts[0].pins[0].index = -1 // pin 1 flagged not connected
	marks := 0
	for _, p := range noConnects(pg) {
		for _, sh := range noConnectMark(p) {
			marks++
			a, b := sh.GetPoints()[0], sh.GetPoints()[1]
			if sh.GetKind() != geom.Shape_KIND_POLYLINE || a.GetX()+b.GetX() != 2*40 || a.GetY()+b.GetY() != 2*-60 {
				t.Errorf("stroke %v is not a polyline through pin 1 at (40, 60)", sh)
			}
		}
	}
	if marks != 2 {
		t.Errorf("%d strokes, want the two of one cross", marks)
	}
}

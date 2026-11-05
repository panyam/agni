package orcad

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

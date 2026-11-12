package kicad

import (
	"bufio"
	"bytes"
	"math"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// Q5's body is a background-filled circle, and its channel, arrow and diode are drawn inside it.
// KiCad paints body fills under the lines, so every background fill must come first in the
// symbol, or the circle's fill covers what is inside it (agni 1344).
func TestABodyFillDrawsUnderTheLinesInsideIt(t *testing.T) {
	g := readSheetGeometry(t, "orient-jetson-fet.kicad_sch")
	sym := geomath.IndexSymbols(g).SymbolFor(placement(g, "Q5"))
	if sym == nil {
		t.Fatal("Q5 has no symbol")
	}
	fills, after := 0, 0
	for _, s := range sym.Shapes {
		if s.Fill == geom.Shape_FILL_BACKGROUND {
			if after > 0 {
				t.Fatalf("a background fill draws after %d other shapes, over them", after)
			}
			fills++
			continue
		}
		after++
	}
	if fills == 0 || after < 13 {
		t.Fatalf("Q5 has %d background fills and %d shapes after them, want at least 1 and 13", fills, after)
	}
}

// The fet cut's U61 and Q5 are marked (dnp yes), and TP76 is fitted.
func TestDoNotPopulateReachesThePlacement(t *testing.T) {
	g := readSheetGeometry(t, "orient-jetson-fet.kicad_sch")
	for ref, want := range map[string]bool{"U61": true, "Q5": true, "TP76": false} {
		pl := placement(g, ref)
		if pl == nil {
			t.Fatalf("placement %s not found", ref)
		}
		if pl.Dnp != want {
			t.Errorf("%s dnp = %v, want %v", ref, pl.Dnp, want)
		}
	}
}

// KiCad draws an arrow at a hierarchical label's connect point. orient-labels.kicad_sch has an
// input label at each angle, and every corner of each arrow must be kicad-cli's.
func TestHierarchicalLabelArrowsMatchKiCad(t *testing.T) {
	arrowsMatchKiCad(t, "orient-labels.kicad_sch", "orient-labels.kicad-arrows.txt", 4, false)
}

// KiCad draws a sheet pin's arrow inside the sheet's box, as the hierarchical label it stands for
// seen from the parent. orient-sheetpins.kicad_sch has pins of each shape on each side of a sheet,
// and each arrow must be kicad-cli's outline (agni issue 1344). The outline is compared as a closed
// polygon, from any corner and in either direction. Every arrow is symmetric about its pin's line,
// and kicad-cli lists some sheet pins' corners in the reverse order from the matching label's, so
// the order says nothing about what is drawn.
func TestSheetPinArrowsMatchKiCad(t *testing.T) {
	arrowsMatchKiCad(t, "orient-sheetpins.kicad_sch", "orient-sheetpins.kicad-arrows.txt", 10, true)
}

// arrowsMatchKiCad holds every arrow in an oracle file, kicad-cli's corners per row, to an arrow
// the reader draws on the fixture's first sheet, corner by corner in kicad-cli's order. With
// anyOrder, the corners may start anywhere and run either way round the outline.
func arrowsMatchKiCad(t *testing.T, fixture, oracle string, rows int, anyOrder bool) {
	t.Helper()
	g := readSheetGeometry(t, fixture)
	type xy struct{ x, y float64 }
	var ours [][]xy
	for _, s := range g.Sheets[0].Shapes {
		if s.Kind != geom.Shape_KIND_POLYLINE {
			continue
		}
		var pts []xy
		for _, p := range s.Points {
			pts = append(pts, xy{float64(p.X) / 1e6, -float64(p.Y) / 1e6})
		}
		ours = append(ours, pts)
	}
	read := 0
	sc := bufio.NewScanner(bytes.NewReader(readFixture(t, oracle)))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		read++
		f := strings.Split(line, "\t")
		var want []xy
		for _, c := range f[1:] {
			a, b, _ := strings.Cut(c, ",")
			x, _ := strconv.ParseFloat(a, 64)
			y, _ := strconv.ParseFloat(b, 64)
			want = append(want, xy{x, y})
		}
		same := func(o []xy, start, step int) bool {
			for i, w := range want {
				p := o[((start+step*i)%len(want)+len(want))%len(want)]
				if math.Hypot(p.x-w.x, p.y-w.y) > 0.005 {
					return false
				}
			}
			return true
		}
		found := false
		for _, o := range ours {
			if len(o) < len(want) {
				continue
			}
			if !anyOrder {
				found = same(o, 0, 1)
			} else {
				// A closed outline repeats its first corner last, so compare its distinct corners.
				ring := o[:len(want)]
				for start := 0; start < len(want) && !found; start++ {
					found = same(ring, start, 1) || same(ring, start, -1)
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Errorf("%s: no arrow of ours has kicad-cli's corners %v", f[0], want)
		}
	}
	if read != rows {
		t.Fatalf("read %d kicad-cli arrows, want %d", read, rows)
	}
}

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
	g := readSheetGeometry(t, "orient-labels.kicad_sch")
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
	rows := 0
	sc := bufio.NewScanner(bytes.NewReader(readFixture(t, "orient-labels.kicad-arrows.txt")))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		rows++
		f := strings.Split(line, "\t")
		var want []xy
		for _, c := range f[1:] {
			a, b, _ := strings.Cut(c, ",")
			x, _ := strconv.ParseFloat(a, 64)
			y, _ := strconv.ParseFloat(b, 64)
			want = append(want, xy{x, y})
		}
		found := false
		for _, o := range ours {
			if len(o) < len(want) {
				continue
			}
			ok := true
			for i, w := range want {
				if math.Hypot(o[i].x-w.x, o[i].y-w.y) > 0.005 {
					ok = false
					break
				}
			}
			if ok {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: no arrow of ours has kicad-cli's corners %v", f[0], want)
		}
	}
	if rows != 4 {
		t.Fatalf("read %d kicad-cli arrows, want 4", rows)
	}
}

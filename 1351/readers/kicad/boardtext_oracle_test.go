package kicad

import (
	"bufio"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// oracleText is one text as kicad-cli strokes it, its layer and glyph box in mm, Y down.
type oracleText struct {
	layer, text    string
	x0, y0, x1, y1 float64
}

func readTextOracle(t *testing.T, name string) []oracleText {
	t.Helper()
	f, err := os.Open("testdata/" + name + ".kicad.text.oracle")
	if err != nil {
		t.Fatalf("%v (hack/kicad_text_oracle.sh writes it)", err)
	}
	defer f.Close()
	var out []oracleText
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "#") {
			continue
		}
		p := strings.Split(sc.Text(), "\t")
		if len(p) != 6 {
			t.Fatalf("bad oracle line %q", sc.Text())
		}
		v := make([]float64, 4)
		for i := range v {
			v[i], _ = strconv.ParseFloat(p[2+i], 64)
		}
		out = append(out, oracleText{p[0], p[1], v[0], v[1], v[2], v[3]})
	}
	return out
}

// capsOnly is a string whose glyphs all stand on the baseline and reach the cap height, so its
// stroked box is exactly the text's height tall. A lowercase letter or a descender would not be.
// Q is left out, since its tail goes below the baseline.
var capsOnly = regexp.MustCompile(`^[A-PR-Z0-9.+*?#%~-]+$`)

// Every footprint field agni draws sits on the layer kicad-cli draws it on, at the height kicad-cli
// strokes it (agni issue 1366). The oracle files come from hack/kicad_text_oracle.sh, so CI needs no
// KiCad. The fixture and gateway hold fields that give no size or layer, and Jetson holds every
// field KiCad writes with its size and layer stated.
func TestBoardTextAgainstKiCad(t *testing.T) {
	for _, c := range []struct {
		name, board string
		atLeast     int
	}{
		{"board", "testdata/board.kicad_pcb", 3},
		{"gateway", "../../examples/tutorial-project/designs/gateway/gateway.kicad_pcb", 30},
		{"jetson-agx-thor-baseboard", "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_pcb", 1000},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, err := os.Open(c.board)
			if err != nil {
				t.Fatalf("%v (make samples-oracle fetches the boards)", err)
			}
			defer f.Close()
			b, err := ReadBoardGeometry(f, c.name+".kicad_pcb")
			if err != nil {
				t.Fatal(err)
			}
			byText := map[string][]oracleText{}
			for _, o := range readTextOracle(t, c.name) {
				byText[o.text] = append(byText[o.text], o)
			}
			compared, wrongLayer, wrongHeight := 0, 0, 0
			for _, txt := range b.GetTexts() {
				if txt.GetKind() != "reference" && txt.GetKind() != "value" || !capsOnly.MatchString(txt.GetText()) {
					continue
				}
				o, ok := nearest(byText[txt.GetText()], txt)
				if !ok {
					continue
				}
				compared++
				if o.layer != txt.GetLayer() {
					wrongLayer++
					if wrongLayer <= 5 {
						t.Errorf("%s %q: agni draws it on %q, KiCad on %q", txt.GetRefDes(), txt.GetText(), txt.GetLayer(), o.layer)
					}
				}
				extent := o.y1 - o.y0
				if r := math.Mod(math.Abs(txt.GetRotationDeg()), 180); math.Abs(r-90) < 1 {
					extent = o.x1 - o.x0
				} else if r > 1 && r < 179 {
					compared--
					continue
				}
				if got := float64(txt.GetHeight()) / 1e6; math.Abs(got-extent) > 0.05*extent {
					wrongHeight++
					if wrongHeight <= 5 {
						t.Errorf("%s %q: agni's height %.3f mm, KiCad strokes it %.3f mm tall", txt.GetRefDes(), txt.GetText(), got, extent)
					}
				}
			}
			if compared < c.atLeast {
				t.Fatalf("compared %d texts with KiCad's, want at least %d, so the test checks too little", compared, c.atLeast)
			}
			if wrongLayer+wrongHeight > 0 {
				t.Errorf("%d of %d texts on a different layer, %d at a different height", wrongLayer, compared, wrongHeight)
			}
		})
	}
}

// nearest is the oracle row for a text of the same string whose box centre is nearest agni's anchor,
// within a few glyph heights, on agni's layer when one is there. A footprint can stroke its designator
// twice, the field on the silkscreen and a ${REFERENCE} text on the fab layer, and two parts can share
// a value, so a row on another layer counts only when agni's layer has none nearby. agni's anchor is
// Y-up in nanometres, and the oracle's box Y-down in mm.
func nearest(rows []oracleText, txt *geom.BoardText) (oracleText, bool) {
	ax, ay := float64(txt.GetAt().GetX())/1e6, -float64(txt.GetAt().GetY())/1e6
	limit := 4 * math.Max(float64(txt.GetHeight())/1e6, 1)
	pick := func(sameLayer bool) (oracleText, bool) {
		best, dist := oracleText{}, math.Inf(1)
		for _, o := range rows {
			if sameLayer && o.layer != txt.GetLayer() {
				continue
			}
			if d := math.Hypot((o.x0+o.x1)/2-ax, (o.y0+o.y1)/2-ay); d < dist {
				best, dist = o, d
			}
		}
		return best, dist <= limit
	}
	if o, ok := pick(true); ok {
		return o, true
	}
	return pick(false)
}

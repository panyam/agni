package render

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/kicad"
)

// kicad-cli 10.0.6 strokes "HiW" at a 10 mm height with a 1 mm pen, anchored at x=100, with its
// ink over [103.039, 125.896] left-justified, [89.048, 111.905] centred and [75.056, 97.913]
// right-justified. strokeInk places the ink there from the glyph table alone.
func TestStrokeInkMatchesKiCad(t *testing.T) {
	for _, c := range []struct {
		justify    string
		start, end float64
	}{
		{"left", 3.039, 25.896},
		{"", -10.952, 11.905},
		{"right", -24.944, -2.087},
	} {
		start, length, ok := strokeInk("HiW", c.justify, 10, 1)
		if !ok || math.Abs(start-c.start) > 0.01 || math.Abs(start+length-c.end) > 0.01 {
			t.Errorf("justify %q: ink [%.3f, %.3f], want [%.3f, %.3f]", c.justify, start, start+length, c.start, c.end)
		}
	}
	if _, _, ok := strokeInk("   ", "", 10, 1); ok {
		t.Error("a line of spaces has ink")
	}
}

// A stroke-font run moves along its own direction, backwards when it is mirrored, and draws
// left-justified at its ink's length. A text in any other font keeps its anchor and justify.
func TestBoardTextRunFollowsTheTextsFrame(t *testing.T) {
	base := func() *geom.BoardText {
		return &geom.BoardText{Text: "HiW", At: &geom.Point{X: 0, Y: 0}, Height: 10_000_000, Thickness: 1_000_000, Justify: "left", Font: "kicad_stroke"}
	}
	plain := base()
	at, justify, width := boardTextRun(plain)
	if at.X != 3_038_952 || at.Y != 0 || justify != "left" || width != 22_857_143 {
		t.Errorf("left run at (%d,%d) %q width %d, want (3038952,0) left 22857143", at.X, at.Y, justify, width)
	}
	mirrored := base()
	mirrored.Mirror = true
	if at, _, _ := boardTextRun(mirrored); at.X != -3_038_952 {
		t.Errorf("mirrored run starts at x=%d, want -3038952", at.X)
	}
	turned := base()
	turned.RotationDeg = 90
	if at, _, _ := boardTextRun(turned); at.X != 0 || at.Y != 3_038_952 {
		t.Errorf("turned run starts at (%d,%d), want (0,3038952)", at.X, at.Y)
	}
	other := base()
	other.Font = ""
	if at, justify, width := boardTextRun(other); at.X != 0 || justify != "left" || width != 0 {
		t.Errorf("a run in the renderer's own font moved to (%d,%d) %q width %d", at.X, at.Y, justify, width)
	}
}

// Every board text a KiCad board draws starts and ends its ink where kicad-cli strokes it, within
// 0.1 mm, against the text oracles hack/kicad_text_oracle.sh writes. Only texts at a multiple of 90
// degrees are held, since an oracle box is axis-aligned.
func TestBoardRunsAgainstKiCad(t *testing.T) {
	for _, c := range []struct {
		name, board string
		atLeast     int
	}{
		{"board", "../../readers/kicad/testdata/board.kicad_pcb", 6},
		{"gateway", "../../examples/tutorial-project/designs/gateway/gateway.kicad_pcb", 38},
		{"jetson-agx-thor-baseboard", "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_pcb", 5500},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, err := os.Open(c.board)
			if err != nil {
				t.Fatalf("%v (make samples-oracle fetches the boards)", err)
			}
			defer f.Close()
			b, err := kicad.ReadBoardGeometry(f, filepath.Base(c.board))
			if err != nil {
				t.Fatal(err)
			}
			oracle := readRunOracle(t, "../../readers/kicad/testdata/"+c.name+".kicad.text.oracle")
			compared, off := 0, 0
			for _, l := range boardLabels(b) {
				at, _, w := boardTextRun(l)
				turn := math.Mod(l.GetRotationDeg()+360, 90)
				if w == 0 || turn > 0.01 && turn < 89.99 {
					continue
				}
				r := l.GetRotationDeg() * math.Pi / 180
				dx, dy := math.Cos(r), math.Sin(r)
				if l.GetMirror() {
					dx, dy = -dx, -dy
				}
				sx, sy := float64(at.X)/1e6, -float64(at.Y)/1e6 // the oracle is in mm, Y down
				ex, ey := sx+dx*float64(w)/1e6, sy-dy*float64(w)/1e6
				a0, a1 := math.Min(sx, ex), math.Max(sx, ex)
				along := 0 // the box's x extent; 1 for y
				if math.Abs(dy) > math.Abs(dx) {
					a0, a1, along = math.Min(sy, ey), math.Max(sy, ey), 1
				}
				k0, k1, ok := closestRun(oracle[l.GetLayer()+"\t"+l.GetText()], (sx+ex)/2, (sy+ey)/2, a0, a1, along)
				if !ok {
					continue
				}
				compared++
				if math.Abs(a0-k0) > 0.1 || math.Abs(a1-k1) > 0.1 {
					off++
					if off <= 5 {
						t.Errorf("%s %q: agni's ink runs [%.3f, %.3f], KiCad's [%.3f, %.3f]", l.GetLayer(), l.GetText(), a0, a1, k0, k1)
					}
				}
			}
			if compared < c.atLeast {
				t.Fatalf("compared %d runs with KiCad's, want at least %d, so the test checks too little", compared, c.atLeast)
			}
			if off > 0 {
				t.Errorf("%d of %d runs end more than 0.1 mm from KiCad's", off, compared)
			}
		})
	}
}

// readRunOracle reads a text oracle's glyph boxes, keyed by layer and text.
func readRunOracle(t *testing.T, path string) map[string][][4]float64 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string][][4]float64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "#") {
			continue
		}
		p := strings.Split(sc.Text(), "\t")
		var v [4]float64
		for i := range v {
			v[i], _ = strconv.ParseFloat(p[2+i], 64)
		}
		out[p[0]+"\t"+p[1]] = append(out[p[0]+"\t"+p[1]], v)
	}
	return out
}

// closestRun is the extent, along the run's axis, of the oracle box of the same layer and text
// that agrees best with agni's run [a0, a1], among the boxes whose centre is within 2 mm of (x, y).
// A board can place one note many times close together, as Jetson's footprints each carry the same
// author and licence text, so the nearest centre can belong to a neighbour.
func closestRun(boxes [][4]float64, x, y, a0, a1 float64, along int) (float64, float64, bool) {
	k0, k1, best := 0.0, 0.0, math.Inf(1)
	for _, b := range boxes {
		if math.Hypot((b[0]+b[2])/2-x, (b[1]+b[3])/2-y) > 2 {
			continue
		}
		lo, hi := b[along], b[along+2]
		if e := math.Max(math.Abs(a0-lo), math.Abs(a1-hi)); e < best {
			k0, k1, best = lo, hi, e
		}
	}
	return k0, k1, !math.IsInf(best, 1)
}

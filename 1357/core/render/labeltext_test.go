package render

import (
	"bufio"
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func readKiCadOracle(t *testing.T, name string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "readers", "kicad", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		if line := sc.Text(); line != "" && !strings.HasPrefix(line, "#") {
			rows = append(rows, strings.Split(line, "\t"))
		}
	}
	return rows
}

// orient-labels.kicad_sch puts a local, a hierarchical and a global label at every angle, and a
// global label of every flag shape. Each label's text must sit where kicad-cli sets it, with its
// anchor and rotation (agni 1344).
func TestLabelTextSitsWhereKiCadDrawsIt(t *testing.T) {
	g := readKiCadSheet(t, "orient-labels.kicad_sch")
	ours := map[string]placedLabel{}
	for _, l := range collectLabels(g, g.Sheets[0], DefaultStyle) {
		if l.color == DefaultStyle.Label {
			ours[l.text] = l
		}
	}
	rows := readKiCadOracle(t, "orient-labels.kicad-text.txt")
	if len(rows) != 17 {
		t.Fatalf("read %d kicad-cli label runs, want 17 (4 local, 4 hierarchical, 4 global, 5 flag shapes)", len(rows))
	}
	anchor := map[string]string{"left": "start", "right": "end", "center": "middle", "": "middle"}
	for _, r := range rows {
		l, ok := ours[r[0]]
		if !ok {
			t.Errorf("%s is not drawn", r[0])
			continue
		}
		wx, _ := strconv.ParseFloat(r[1], 64)
		wy, _ := strconv.ParseFloat(r[2], 64)
		rot, _ := strconv.Atoi(r[4])
		x, y := labelBaselineMM(l)
		// kicad-cli centers a global label's text on the line through an E's middle, 0.07 heights
		// below where the rest of its text sits, so the tolerance covers that.
		if d := math.Hypot(x-wx, y-wy); d > 0.1 {
			t.Errorf("%s: kicad-cli sets it at (%.3f, %.3f), we at (%.3f, %.3f), %.3f mm away", r[0], wx, wy, x, y, d)
		}
		h, _ := splitJustify(l.justify)
		if anchor[h] != r[3] || ((l.rotationDeg-int32(rot))%360+360)%360 != 0 {
			t.Errorf("%s: we anchor %s at %d degrees, kicad-cli %s at %d", r[0], anchor[h], l.rotationDeg, r[3], rot)
		}
	}
}

// A global label's flag starts at its connect point. Its first two corners and its last one are
// the end at the wire, and they must be kicad-cli's. The far end follows the text's width, which
// is our font's and not KiCad's.
func TestGlobalLabelFlagsStartWhereKiCadDrawsThem(t *testing.T) {
	g := readKiCadSheet(t, "orient-labels.kicad_sch")
	sheet := g.Sheets[0]
	def := defaultTextHeight(g, sheet)
	rows := readKiCadOracle(t, "orient-labels.kicad-flags.txt")
	if len(rows) != 9 {
		t.Fatalf("read %d kicad-cli flags, want 9 (4 angles, 5 shapes)", len(rows))
	}
	for _, r := range rows {
		var pts [][2]float64
		for _, xy := range r[1:] {
			a, b, _ := strings.Cut(xy, ",")
			x, _ := strconv.ParseFloat(a, 64)
			y, _ := strconv.ParseFloat(b, 64)
			pts = append(pts, [2]float64{x, y})
		}
		var outline [][2]float64
		for _, l := range sheet.Labels {
			if l.Text == r[0] {
				for _, p := range flagOutline(l, labelHeight(l, def)) {
					outline = append(outline, [2]float64{float64(p.X) / 1e6, -float64(p.Y) / 1e6})
				}
			}
		}
		if len(outline) != 7 {
			t.Errorf("%s: we draw %d outline points, want 6 and the closing one", r[0], len(outline))
			continue
		}
		for _, i := range []int{0, 1, 5} {
			if d := math.Hypot(outline[i][0]-pts[i][0], outline[i][1]-pts[i][1]); d > 0.02 {
				t.Errorf("%s corner %d: kicad-cli (%.4f, %.4f), we (%.4f, %.4f)", r[0], i, pts[i][0], pts[i][1], outline[i][0], outline[i][1])
			}
		}
	}
}

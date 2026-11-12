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

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/kicad"
)

func readKiCadSheet(t *testing.T, name string) *geom.SchematicGeometry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "readers", "kicad", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	g, err := kicad.ReadSchematicGeometry(bytes.NewReader(raw), name)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// kicadRun is one text run as kicad-cli drew it, in page millimetres with y down.
type kicadRun struct {
	text     string
	x, y     float64
	anchor   string
	rotation int32
}

// The fet cut holds U61, whose pin names sit inside its body, and Q5, whose names sit above its
// pin lines (a name offset of 0) and whose gate pin is vertical. kicad-cli's export of it is the
// oracle: every pin name and number must sit where kicad-cli sets it, with its anchor and
// rotation.
func TestPinTextSitsWhereKiCadDrawsIt(t *testing.T) {
	g := readKiCadSheet(t, "orient-jetson-fet.kicad_sch")
	var ours []placedLabel
	for _, l := range collectLabels(g, g.Sheets[0], DefaultStyle) {
		if l.color == DefaultStyle.PinName || l.color == DefaultStyle.Ruler {
			ours = append(ours, l)
		}
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "readers", "kicad", "testdata", "orient-jetson-fet.kicad-text.txt"))
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]bool{"Q5": true, "U61": true, "TP76": true, "SISS05DN-T1-GE3": true, "LTC4412IS6#TRMPBF": true}
	var runs []kicadRun
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		f := strings.Split(sc.Text(), "\t")
		if strings.HasPrefix(f[0], "#") || fields[f[0]] {
			continue
		}
		x, _ := strconv.ParseFloat(f[1], 64)
		y, _ := strconv.ParseFloat(f[2], 64)
		r, _ := strconv.Atoi(f[4])
		runs = append(runs, kicadRun{f[0], x, y, f[3], int32(r)})
	}
	if len(runs) != 19 {
		t.Fatalf("read %d kicad-cli pin runs, want 19 (U61's 6 names and 6 numbers, Q5's 3 and 3, TP76's number)", len(runs))
	}
	anchor := map[string]string{"left": "start", "right": "end", "center": "middle", "": "middle"}
	for _, want := range runs {
		best, bestD := placedLabel{}, math.Inf(1)
		for _, l := range ours {
			if l.text != want.text {
				continue
			}
			x, y := labelBaselineMM(l)
			if d := math.Hypot(x-want.x, y-want.y); d < bestD {
				best, bestD = l, d
			}
		}
		if bestD > 0.1 {
			t.Errorf("%q: kicad-cli sets it at (%.3f, %.3f); our nearest is %.3f mm away", want.text, want.x, want.y, bestD)
			continue
		}
		h, _ := splitJustify(best.justify)
		if anchor[h] != want.anchor || best.rotationDeg != want.rotation {
			t.Errorf("%q at (%.3f, %.3f): we anchor %s at %d degrees, kicad-cli %s at %d",
				want.text, want.x, want.y, anchor[h], best.rotationDeg, want.anchor, want.rotation)
		}
	}
}

// labelBaselineMM is where kicad-cli would write our run, in its page millimetres. kicad-cli
// writes a run at its baseline, half a glyph height below the glyphs' center, and a
// bottom-justified run of ours is centered half its height above its anchor along its up.
func labelBaselineMM(l placedLabel) (float64, float64) {
	phi := float64(-l.rotationDeg) * math.Pi / 180
	ux, uy := -math.Sin(phi), math.Cos(phi)
	_, v := splitJustify(l.justify)
	off := -float64(l.height) / 2
	switch v {
	case "bottom":
		off += float64(l.height) / 2
	case "top":
		off -= float64(l.height) / 2
	}
	x, y := float64(l.x)+ux*off, float64(l.y)+uy*off
	return x / 1e6, -y / 1e6
}

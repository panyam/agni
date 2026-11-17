package render

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/ipc2581"
	"github.com/panyam/agni/readers/kicad"
)

// kicadDims is a kicaddim oracle (hack/kicad_dimension_oracle.sh): the board's outline bounds and
// each pad's centre as kicad-cli exports them, in millimetres in agni's board frame.
type kicadDims struct {
	extent [4]float64 // min x, min y, max x, max y
	pads   []kicadPad
}

type kicadPad struct {
	ref, pin, side string
	x, y           float64
}

// padTolMm is how far a drawn pad's centre may sit from KiCad's. IPC-D-356 rounds each centre to
// 0.0001 inch, which is 2.54 micrometres, so a centre reads up to 1.3 micrometres off on each axis.
const padTolMm = 0.002

// extentTolMm is how far the drawn outline's bounds may sit from KiCad's. GenCAD states the outline to
// 15 digits, so this allows only float noise.
const extentTolMm = 0.0005

func readKiCadDims(t *testing.T, path string) kicadDims {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out kicadDims
	extents := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		w := strings.Split(line, "\t")
		num := func(s string) float64 {
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				t.Fatalf("%s: row %q: %v", path, line, err)
			}
			return v
		}
		switch {
		case w[0] == "extent" && len(w) == 5:
			extents++
			for i := range out.extent {
				out.extent[i] = num(w[i+1])
			}
		case w[0] == "pad" && len(w) == 6:
			out.pads = append(out.pads, kicadPad{ref: w[1], pin: w[2], side: w[3], x: num(w[4]), y: num(w[5])})
		default:
			t.Fatalf("%s: row %q is neither an extent nor a pad", path, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if extents != 1 {
		t.Fatalf("%s: %d extent rows, want 1", path, extents)
	}
	return out
}

// drawnDims is what the packed board draws, in millimetres in the board frame: the bounds of its
// edge primitives, and the centre of every primitive keyed to a pad, by "ref pin".
type drawnDims struct {
	extent [4]float64
	pads   map[string][][2]float64
}

// measurePacked reads a packed board back into millimetres, as the viewer's measuring tool does. Each
// vertex is relative to the sheet's origin, and the sheet's units_per_mm makes it millimetres. A pad primitive's centre is the middle of its bounds, which
// holds for every land the packer draws, since each is a disk or a quad centred on the pad.
func measurePacked(t *testing.T, sheet *geom.PackedSheet) drawnDims {
	t.Helper()
	perMm := sheet.GetUnitsPerMm()
	if perMm <= 0 {
		t.Fatalf("the packed board states units_per_mm %v", perMm)
	}
	ox, oy := float64(sheet.GetOriginX()), float64(sheet.GetOriginY())
	verts := decodeVerts(sheet.GetVertices())
	recs := decodeRecords(sheet.GetPrimitives())
	bounds := func(r primRecord) (minX, minY, maxX, maxY float64) {
		minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
		for _, v := range verts[r.firstVertex : r.firstVertex+r.count] {
			x, y := (float64(v[0])+ox)/perMm, (float64(v[1])+oy)/perMm
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		}
		return
	}
	out := drawnDims{extent: [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}, pads: map[string][][2]float64{}}
	for _, r := range recs {
		if r.group != groupBoardEdge {
			continue
		}
		x0, y0, x1, y1 := bounds(r)
		out.extent = [4]float64{min(out.extent[0], x0), min(out.extent[1], y0), max(out.extent[2], x1), max(out.extent[3], y1)}
	}
	for _, k := range PackedKeys(sheet) {
		if int(k.GetPrimitive()) >= len(recs) {
			t.Fatalf("key names primitive %d of %d", k.GetPrimitive(), len(recs))
		}
		r := recs[k.GetPrimitive()]
		// A pad with no number, such as a fiducial, keys only its part, as a pick area and a silk
		// stroke do, and a via's land keys nothing. So a pin-less key counts as a pad only when it
		// names a part and is a land on outer or through copper.
		if k.GetPin() == "" && (k.GetRefDes() == "" || r.kind != primTriangles || !isPadGroup(r.group)) {
			continue
		}
		x0, y0, x1, y1 := bounds(r)
		key := k.GetRefDes() + " " + k.GetPin()
		out.pads[key] = append(out.pads[key], [2]float64{(x0 + x1) / 2, (y0 + y1) / 2})
	}
	return out
}

func isPadGroup(g uint8) bool {
	switch g {
	case groupBoardCopperFront, groupBoardCopperBack, groupBoardCopperInner, groupBoardThrough:
		return true
	}
	return false
}

// unlistedHoles lists, in millimetres, the centre of every pad with no number whose drill is as wide
// as its land. Such a pad is a mounting hole. Its land draws entirely under its hole, and KiCad's
// IPC-D-356 export leaves it out, so the oracle has no row to hold it to.
func unlistedHoles(b *geom.BoardGeometry) [][2]float64 {
	perMm := 1e6 / float64(b.GetUnitNm())
	var out [][2]float64
	for _, pl := range b.GetPlacements() {
		for _, pad := range pl.GetPads() {
			if pad.GetNumber() == "" && pad.GetDrill() > 0 && pad.GetDrill() >= max(pad.GetSize().GetX(), pad.GetSize().GetY()) {
				x, y := padWorld(pl, pad)
				out = append(out, [2]float64{float64(x) / perMm, float64(y) / perMm})
			}
		}
	}
	return out
}

// dimensionErrors holds a packed board to a kicaddim oracle and returns one line per disagreement.
// Every oracle pad needs a drawn primitive of its part and pin within padTolMm, and every drawn pad
// primitive needs an oracle pad of its part and pin within padTolMm. The second direction catches a
// pad drawn twice, once in the right place and once in the wrong one. It skips a pin-less primitive
// on one of holes, since the oracle cannot list it.
func dimensionErrors(oracle kicadDims, drawn drawnDims, holes [][2]float64) (errs []string, compared int) {
	for i, want := range oracle.extent {
		if d := math.Abs(drawn.extent[i] - want); d > extentTolMm {
			errs = append(errs, "outline bound "+strconv.Itoa(i)+": drawn "+mm(drawn.extent[i])+", KiCad "+mm(want))
		}
	}
	byKey := map[string][]kicadPad{}
	for _, p := range oracle.pads {
		key := p.ref + " " + p.pin
		byKey[key] = append(byKey[key], p)
		best := math.Inf(1)
		for _, c := range drawn.pads[key] {
			best = min(best, math.Hypot(c[0]-p.x, c[1]-p.y))
		}
		if best > padTolMm {
			errs = append(errs, key+": KiCad puts it at ("+mm(p.x)+", "+mm(p.y)+"), nearest drawn "+mm(best)+" away")
		}
		compared++
	}
	for key, centres := range drawn.pads {
		for _, c := range centres {
			if strings.HasSuffix(key, " ") && slices.ContainsFunc(holes, func(h [2]float64) bool { return math.Hypot(c[0]-h[0], c[1]-h[1]) <= padTolMm }) {
				continue
			}
			best := math.Inf(1)
			for _, p := range byKey[key] {
				best = min(best, math.Hypot(c[0]-p.x, c[1]-p.y))
			}
			if best > padTolMm {
				errs = append(errs, key+": drawn at ("+mm(c[0])+", "+mm(c[1])+"), nearest KiCad pad "+mm(best)+" away")
			}
		}
	}
	return errs, compared
}

func mm(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) + " mm" }

func dimensionsBoard(t *testing.T) *geom.BoardGeometry {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "dimensions.kicad_pcb"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, err := kicad.ReadBoardGeometry(f, "dimensions.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// holdToKiCad packs a board and holds it to an oracle. It fails on a comparison that covers fewer
// pads than want, and on any disagreement that no entry of known begins. Each entry of known must
// still begin a disagreement, so a fix fails the test until its entry is removed.
func holdToKiCad(t *testing.T, b *geom.BoardGeometry, oraclePath string, want int, known ...string) {
	t.Helper()
	oracle := readKiCadDims(t, oraclePath)
	if len(oracle.pads) < want {
		t.Fatalf("%s holds %d pads, want at least %d; regenerate it with hack/kicad_dimension_oracle.sh", oraclePath, len(oracle.pads), want)
	}
	errs, compared := dimensionErrors(oracle, measurePacked(t, PackBoard(b)), unlistedHoles(b))
	seen := map[string]bool{}
	for _, e := range errs {
		i := slices.IndexFunc(known, func(k string) bool { return strings.HasPrefix(e, k) })
		if i < 0 {
			t.Error(e)
			continue
		}
		seen[known[i]] = true
	}
	for _, k := range known {
		if !seen[k] {
			t.Errorf("%q no longer disagrees; remove it from known", k)
		}
	}
	if compared != len(oracle.pads) {
		t.Errorf("compared %d of %d pads", compared, len(oracle.pads))
	}
	t.Logf("%d pads and the outline held to KiCad, %d disagree", compared, len(errs))
}

// TestBoardPadsSitWhereKiCadPutsThem holds the packed board, the geometry the viewer draws, to
// kicad-cli's own pad centres and outline (agni issue 133). The fixture turns one three-pad part to
// 0, 90, 45 and 180 degrees, puts it on the back at 30 degrees, and turns a two-pin header to 270, so
// a wrong rotation, mirror or scale moves a pad off its row. U1 also has a paste window, which is not
// copper, and J1 a mounting hole, which KiCad does not list.
func TestBoardPadsSitWhereKiCadPutsThem(t *testing.T) {
	holdToKiCad(t, dimensionsBoard(t), filepath.Join("testdata", "dimensions.kicaddim.tsv"), 17)
}

// TestGlasgowPadsSitWhereKiCadPutsThem holds Glasgow revC3's 1,137 pads to kicad-cli, on a board
// whose auxiliary origin is not the file's.
func TestGlasgowPadsSitWhereKiCadPutsThem(t *testing.T) {
	holdToKiCad(t, glasgowBoard(t), filepath.Join("testdata", "glasgow.kicaddim.tsv"), 1100)
}

// TestIPC2581PadsSitWhereKiCadPutsThem reads KiCad's IPC-2581 export of the fixture and holds it to
// the same rows, so the check covers a second reader without a second oracle. KiCad exports U1's
// paste window and J1's mounting hole as package pins, PAD3 and NPTH2, which no copper land names
// (agni issue 1679).
func TestIPC2581PadsSitWhereKiCadPutsThem(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "dimensions.ipc2581.xml"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, err := ipc2581.ReadBoardGeometry(f, "dimensions.ipc2581.xml")
	if err != nil {
		t.Fatal(err)
	}
	holdToKiCad(t, b, filepath.Join("testdata", "dimensions.kicaddim.tsv"), 17)
}

// TestDimensionOracleCatchesATurnAMirrorAndAScale is the positive control for the three tests above.
// Each case breaks the fixture's geometry one way, and the oracle must name the part it broke.
func TestDimensionOracleCatchesATurnAMirrorAndAScale(t *testing.T) {
	oracle := readKiCadDims(t, filepath.Join("testdata", "dimensions.kicaddim.tsv"))
	cases := []struct {
		name, want string
		breakIt    func(b *geom.BoardGeometry)
	}{
		{"U2 turned a further quarter", "U2 ", func(b *geom.BoardGeometry) {
			for _, p := range b.GetPlacements() {
				if p.GetRefDes() == "U2" {
					p.RotationDeg += 90
				}
			}
		}},
		{"U5 drawn unmirrored", "U5 ", func(b *geom.BoardGeometry) {
			for _, p := range b.GetPlacements() {
				if p.GetRefDes() == "U5" {
					p.Mirror = !p.GetMirror()
				}
			}
		}},
		{"placed a thousandth too far out", "U4 ", func(b *geom.BoardGeometry) {
			for _, p := range b.GetPlacements() {
				p.At.X = p.GetAt().GetX() * 1001 / 1000
				p.At.Y = p.GetAt().GetY() * 1001 / 1000
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := dimensionsBoard(t)
			if errs, _ := dimensionErrors(oracle, measurePacked(t, PackBoard(b)), unlistedHoles(b)); len(errs) != 0 {
				t.Fatalf("the unbroken fixture disagrees: %v", errs)
			}
			c.breakIt(b)
			errs, _ := dimensionErrors(oracle, measurePacked(t, PackBoard(b)), unlistedHoles(b))
			for _, e := range errs {
				if strings.HasPrefix(e, c.want) {
					return
				}
			}
			t.Fatalf("broken that way, the oracle names no %q pad: %v", c.want, errs)
		})
	}
}

// TestBoardSVGStatesItsMillimetres maps BoardSVG's outline back to millimetres through the
// data-mm-* attributes on its root, as the viewer's measuring tool does in SVG, and holds it to
// KiCad's extent (agni issue 133). The drawing writes each coordinate to a tenth of a pixel, which
// is 6 micrometres on the fixture.
func TestBoardSVGStatesItsMillimetres(t *testing.T) {
	s := BoardSVG(dimensionsBoard(t))
	root := s[:strings.Index(s, ">")]
	attr := func(name string) float64 {
		m := regexp.MustCompile(name + `="([^"]+)"`).FindStringSubmatch(root)
		if m == nil {
			t.Fatalf("the root <svg> has no %s: %s", name, root)
		}
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	perPx, x0, y0 := attr("data-mm-per-px"), attr("data-mm-x0"), attr("data-mm-y0")
	edge := s[strings.Index(s, `<g class="edge"`):]
	edge = edge[:strings.Index(edge, "</g>")]
	got := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	pts := regexp.MustCompile(`(-?[\d.]+),(-?[\d.]+)`).FindAllStringSubmatch(edge, -1)
	if len(pts) < 4 {
		t.Fatalf("the edge group holds %d points: %s", len(pts), edge)
	}
	for _, p := range pts {
		px, _ := strconv.ParseFloat(p[1], 64)
		py, _ := strconv.ParseFloat(p[2], 64)
		x, y := x0+px*perPx, y0-py*perPx
		got = [4]float64{min(got[0], x), min(got[1], y), max(got[2], x), max(got[3], y)}
	}
	want := readKiCadDims(t, filepath.Join("testdata", "dimensions.kicaddim.tsv")).extent
	for i := range want {
		if d := math.Abs(got[i] - want[i]); d > 0.01 {
			t.Errorf("outline bound %d reads %s off the SVG, KiCad %s", i, mm(got[i]), mm(want[i]))
		}
	}
}

package render

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
	"github.com/panyam/agni/readers/formats"
)

// placedMeshes places every part's model meshes on the board, as a viewer composes them, and returns
// each part's outline seen from above as a CCW ring in board units, and its top in millimetres. move
// shifts every mesh point in the footprint's frame before placement, for the positive control.
func placedMeshes(t *testing.T, b *geom.BoardGeometry, pcb string, move [3]float64) (rings map[string][]float64, tops map[string]float64, unread map[string]string) {
	t.Helper()
	keys := map[string]bool{}
	for _, p := range b.GetPlacements() {
		for _, in := range p.GetModelInstances() {
			keys[in.GetMesh()] = true
		}
	}
	var list []string
	for k := range keys {
		list = append(list, k)
	}
	sort.Strings(list)
	meshes, unread := (&formats.Loader{ModelPaths: []string{kicadModelsDir}}).BoardMeshes(pcb, list)
	upm := 1e6 / float64(b.GetUnitNm())
	rings, tops = map[string][]float64{}, map[string]float64{}
	for _, p := range b.GetPlacements() {
		var pts [][2]float64
		top, any := 0.0, false
		for _, in := range p.GetModelInstances() {
			m := meshes[in.GetMesh()]
			tr := in.GetTransform()
			if m == nil || len(tr) != 12 {
				continue
			}
			for _, q := range m.Points() {
				x := tr[0]*q[0] + tr[1]*q[1] + tr[2]*q[2] + tr[3] + move[0]
				y := tr[4]*q[0] + tr[5]*q[1] + tr[6]*q[2] + tr[7] + move[1]
				z := tr[8]*q[0] + tr[9]*q[1] + tr[10]*q[2] + tr[11] + move[2]
				w := geomath.ComposePlacement(p.GetAt(), p.GetRotationDeg(), p.GetMirror(), &geom.Point{X: int64(math.Round(x * upm)), Y: int64(math.Round(y * upm))})
				pts = append(pts, [2]float64{float64(w.X), float64(w.Y)})
				if !any || z > top {
					top, any = z, true
				}
			}
		}
		if !any {
			continue
		}
		var ring []float64
		for _, q := range convexHull(pts) {
			ring = append(ring, q[0], q[1])
		}
		rings[p.GetRefDes()], tops[p.GetRefDes()] = ring, top
	}
	return rings, tops, unread
}

// meshVsKiCad compares one part's placed mesh with KiCad's model, both ways round, and returns how far
// each outline reaches past the other in millimetres and how far apart their tops are.
func meshVsKiCad(ring []float64, top, upm float64, m kicadModel) (out, in, dz float64) {
	out = outside(ring, upm, m)
	var mine kicadModel
	for k := 0; k+1 < len(ring); k += 2 {
		mine.ring = append(mine.ring, ring[k]/upm, ring[k+1]/upm)
	}
	var theirs []float64
	var pts [][2]float64
	for k := 0; k+1 < len(m.ring); k += 2 {
		pts = append(pts, [2]float64{m.ring[k] * upm, m.ring[k+1] * upm})
	}
	for _, q := range convexHull(pts) {
		theirs = append(theirs, q[0], q[1])
	}
	return out, outside(theirs, upm, mine), math.Abs(top - m.heightMm)
}

// kicadMeshTolMm is how far a placed mesh's outline or top may sit from KiCad's model. The .wrl is a
// tessellation of the model, so a curved edge cuts inside it by up to a few hundredths.
const kicadMeshTolMm = 0.1

// TestMeshesAgreeWithKiCad3D places each Glasgow part's mesh by its ModelInstance and its placement,
// as the stack view does, and holds it to kicad-cli's own 3D export of the board (agni issue 1652).
// Outline and top must agree both ways round, so a mesh placed off, turned, mirrored or scaled wrong
// fails.
func TestMeshesAgreeWithKiCad3D(t *testing.T) {
	pcb := filepath.Join("..", "..", "tools", "samples", "boards", "glasgow-revC3", "glasgow.kicad_pcb")
	b := glasgowBoard(t)
	rings, tops, unread := placedMeshes(t, b, pcb, [3]float64{})
	for k, why := range unread {
		t.Errorf("%s: %s", k, why)
	}
	upm := 1e6 / float64(b.GetUnitNm())
	rows := readKiCad3D(t, filepath.Join("testdata", "glasgow.kicad3d.tsv"))
	compared := 0
	drift := map[string]string{}
	for _, m := range rows {
		ring := rings[m.ref]
		if ring == nil {
			t.Errorf("%s: KiCad places a model and agni draws no mesh", m.ref)
			continue
		}
		out, in, dz := meshVsKiCad(ring, tops[m.ref], upm, m)
		if w := max(out, in, dz); w > kicadMeshTolMm {
			drift[m.ref] = fmt.Sprintf("outline %.3f past, %.3f short, top %.3f off", out, in, dz)
		}
		compared++
	}
	for ref, d := range drift {
		t.Errorf("%s: mesh vs KiCad: %s", ref, d)
	}
	if compared != len(rows) || len(rows) < 200 {
		t.Errorf("compared %d of %d parts, want all of Glasgow's 213", compared, len(rows))
	}
	t.Logf("%d parts' meshes held to KiCad's models", compared)
}

// TestMeshesFollowModelTransforms holds meshes to kicad-cli's export of the model-transforms fixture,
// whose footprints scale, turn and offset their models on both sides (agni issue 1652).
func TestMeshesFollowModelTransforms(t *testing.T) {
	pcb := filepath.Join("testdata", "model-transforms", "model-transforms.kicad_pcb")
	b := readKiCadBoard(t, pcb, kicadModelsDir)
	rings, tops, unread := placedMeshes(t, b, pcb, [3]float64{})
	for k, why := range unread {
		t.Errorf("%s: %s", k, why)
	}
	upm := 1e6 / float64(b.GetUnitNm())
	rows := readKiCad3D(t, filepath.Join("testdata", "model-transforms.kicad3d.tsv"))
	for _, m := range rows {
		ring := rings[m.ref]
		if ring == nil {
			t.Errorf("%s: KiCad places a model and agni draws no mesh", m.ref)
			continue
		}
		if out, in, dz := meshVsKiCad(ring, tops[m.ref], upm, m); max(out, in, dz) > kicadMeshTolMm {
			t.Errorf("%s: outline %.3f past, %.3f short, top %.3f off", m.ref, out, in, dz)
		}
	}
	if len(rows) != 15 {
		t.Errorf("the oracle holds %d parts, want 15", len(rows))
	}
}

// TestMeshOracleCatchesAMovedMesh is the positive control for TestMeshesAgreeWithKiCad3D. Every mesh
// moved 0.3 mm along the footprint's X, or lifted 0.3 mm, must leave most parts out of agreement.
func TestMeshOracleCatchesAMovedMesh(t *testing.T) {
	pcb := filepath.Join("..", "..", "tools", "samples", "boards", "glasgow-revC3", "glasgow.kicad_pcb")
	b := glasgowBoard(t)
	upm := 1e6 / float64(b.GetUnitNm())
	rows := readKiCad3D(t, filepath.Join("testdata", "glasgow.kicad3d.tsv"))
	for name, move := range map[string][3]float64{"moved 0.3 mm": {0.3, 0, 0}, "lifted 0.3 mm": {0, 0, 0.3}} {
		rings, tops, _ := placedMeshes(t, b, pcb, move)
		caught := 0
		for _, m := range rows {
			if ring := rings[m.ref]; ring != nil {
				if out, in, dz := meshVsKiCad(ring, tops[m.ref], upm, m); max(out, in, dz) > kicadMeshTolMm {
					caught++
				}
			}
		}
		if caught < len(rows)*9/10 {
			t.Errorf("%s: only %d of %d parts disagree with KiCad", name, caught, len(rows))
		}
	}
}

// stackMeshes places every part's meshes through the BodyModel transforms BoardStack carries, the path
// the stack view draws by. It reads partBodies, since the transform fixture has no stackup and so no
// BoardStack, and returns each part's outline from above as a CCW ring in board units and its top
// in millimetres off its face. shift moves every placed point, in millimetres, for the control.
func stackMeshes(t *testing.T, b *geom.BoardGeometry, pcb string, shift [3]float64) (rings map[string][]float64, tops map[string]float64) {
	t.Helper()
	bodies := partBodies(b, DefaultStyle)
	keys := map[string]bool{}
	for _, body := range bodies {
		for _, m := range body.GetModels() {
			keys[m.GetMesh()] = true
		}
	}
	var list []string
	for k := range keys {
		list = append(list, k)
	}
	sort.Strings(list)
	meshes, unread := (&formats.Loader{ModelPaths: []string{kicadModelsDir}}).BoardMeshes(pcb, list)
	for k, why := range unread {
		t.Errorf("%s: %s", k, why)
	}
	upm := 1e6 / float64(unitNm(b))
	rings, tops = map[string][]float64{}, map[string]float64{}
	for _, body := range bodies {
		var pts [][2]float64
		top, any := 0.0, false
		for _, bm := range body.GetModels() {
			m, tr := meshes[bm.GetMesh()], bm.GetTransform()
			if m == nil || len(tr) != 12 {
				continue
			}
			for _, q := range m.Points() {
				x := tr[0]*q[0] + tr[1]*q[1] + tr[2]*q[2] + tr[3] + shift[0]*upm
				y := tr[4]*q[0] + tr[5]*q[1] + tr[6]*q[2] + tr[7] + shift[1]*upm
				z := tr[8]*q[0] + tr[9]*q[1] + tr[10]*q[2] + tr[11] + shift[2]
				pts = append(pts, [2]float64{x, y})
				if !any || z > top {
					top, any = z, true
				}
			}
		}
		if !any {
			continue
		}
		var ring []float64
		for _, q := range convexHull(pts) {
			ring = append(ring, q[0], q[1])
		}
		rings[body.GetRefDes()], tops[body.GetRefDes()] = ring, top
	}
	return rings, tops
}

// TestStackMeshesAgreeWithKiCad3D holds the meshes the stack view draws, placed by their BodyModel
// transforms, to kicad-cli's export of Glasgow and of the transform fixture (agni issue
// 1652). A shift of 0.3 mm, sideways or up, must leave nine parts in ten out of agreement.
func TestStackMeshesAgreeWithKiCad3D(t *testing.T) {
	for _, c := range []struct{ pcb, oracle string }{
		{filepath.Join("..", "..", "tools", "samples", "boards", "glasgow-revC3", "glasgow.kicad_pcb"), "glasgow.kicad3d.tsv"},
		{filepath.Join("testdata", "model-transforms", "model-transforms.kicad_pcb"), "model-transforms.kicad3d.tsv"},
	} {
		b := readKiCadBoard(t, c.pcb, kicadModelsDir)
		upm := 1e6 / float64(b.GetUnitNm())
		rows := readKiCad3D(t, filepath.Join("testdata", c.oracle))
		rings, tops := stackMeshes(t, b, c.pcb, [3]float64{})
		for _, m := range rows {
			ring := rings[m.ref]
			if ring == nil {
				t.Errorf("%s %s: KiCad places a model and the stack view draws no mesh", c.oracle, m.ref)
				continue
			}
			if out, in, dz := meshVsKiCad(ring, tops[m.ref], upm, m); max(out, in, dz) > kicadMeshTolMm {
				t.Errorf("%s %s: outline %.3f past, %.3f short, top %.3f off", c.oracle, m.ref, out, in, dz)
			}
		}
		for name, shift := range map[string][3]float64{"moved 0.3 mm": {0.3, 0, 0}, "lifted 0.3 mm": {0, 0, 0.3}} {
			rings, tops := stackMeshes(t, b, c.pcb, shift)
			caught := 0
			for _, m := range rows {
				if ring := rings[m.ref]; ring != nil {
					if out, in, dz := meshVsKiCad(ring, tops[m.ref], upm, m); max(out, in, dz) > kicadMeshTolMm {
						caught++
					}
				}
			}
			if caught < len(rows)*9/10 {
				t.Errorf("%s %s: only %d of %d parts disagree", c.oracle, name, caught, len(rows))
			}
		}
	}
}

// kicadColour is one row of a kicad3dcolour oracle: the part of a model's surface in one material
// colour, as kicad-cli exports it. rgb is in linear light, and x,y is the centre of that area seen
// from above, in millimetres in the board frame.
type kicadColour struct {
	rgb        [3]float64
	area, x, y float64
}

func readKiCadColours(t *testing.T, path string) map[string][]kicadColour {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- a test fixture path
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]kicadColour{}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		w := strings.Split(line, "\t")
		if len(w) != 4 {
			t.Fatalf("%s: row %q has %d fields, want 4", path, line, len(w))
		}
		var c kicadColour
		var nums []float64
		for _, s := range append(strings.Split(w[1], ","), append([]string{w[2]}, strings.Split(w[3], ",")...)...) {
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				t.Fatalf("%s: row %q: %v", path, line, err)
			}
			nums = append(nums, v)
		}
		c.rgb, c.area, c.x, c.y = [3]float64{nums[0], nums[1], nums[2]}, nums[3], nums[4], nums[5]
		out[w[0]] = append(out[w[0]], c)
	}
	return out
}

// colourDrift places every part's meshes by their BodyModel transforms and, for each KiCad material
// colour of the part, returns how far the centre of our surface in that colour sits from KiCad's, in
// millimetres. A mesh part's colour c matches the KiCad material nearest c^2.2, since the GLB writes
// colours in linear light. halfTurn turns every model half a turn about its footprint's origin first,
// for the control.
func colourDrift(t *testing.T, pcb string, kicad map[string][]kicadColour, halfTurn bool) map[string]float64 {
	t.Helper()
	b := readKiCadBoard(t, pcb, kicadModelsDir)
	if halfTurn {
		for _, p := range b.GetPlacements() {
			for _, in := range p.GetModelInstances() {
				for i := 0; i < 8; i++ {
					in.Transform[i] = -in.Transform[i]
				}
			}
		}
	}
	bodies := partBodies(b, DefaultStyle)
	var keys []string
	seen := map[string]bool{}
	for _, body := range bodies {
		for _, m := range body.GetModels() {
			if !seen[m.GetMesh()] {
				seen[m.GetMesh()] = true
				keys = append(keys, m.GetMesh())
			}
		}
	}
	meshes, _ := (&formats.Loader{ModelPaths: []string{kicadModelsDir}}).BoardMeshes(pcb, keys)
	upm := 1e6 / float64(unitNm(b))
	drift := map[string]float64{}
	for _, body := range bodies {
		theirs := kicad[body.GetRefDes()]
		if len(theirs) == 0 {
			continue
		}
		sums := make([][3]float64, len(theirs)) // area, area*x, area*y
		for _, bm := range body.GetModels() {
			m, tr := meshes[bm.GetMesh()], bm.GetTransform()
			if m == nil {
				continue
			}
			place := func(q [3]float64) [3]float64 {
				return [3]float64{(tr[0]*q[0] + tr[1]*q[1] + tr[2]*q[2] + tr[3]) / upm, (tr[4]*q[0] + tr[5]*q[1] + tr[6]*q[2] + tr[7]) / upm, tr[8]*q[0] + tr[9]*q[1] + tr[10]*q[2] + tr[11]}
			}
			for _, part := range m.Parts {
				lin := [3]float64{math.Pow(part.Color[0], 2.2), math.Pow(part.Color[1], 2.2), math.Pow(part.Color[2], 2.2)}
				best, bestD := -1, 0.1
				for i, c := range theirs {
					if d := math.Abs(c.rgb[0]-lin[0]) + math.Abs(c.rgb[1]-lin[1]) + math.Abs(c.rgb[2]-lin[2]); d < bestD {
						best, bestD = i, d
					}
				}
				if best < 0 {
					continue
				}
				for i := 0; i+2 < len(part.Indices); i += 3 {
					a, bb, c := place(part.Positions[part.Indices[i]]), place(part.Positions[part.Indices[i+1]]), place(part.Positions[part.Indices[i+2]])
					u := [3]float64{bb[0] - a[0], bb[1] - a[1], bb[2] - a[2]}
					v := [3]float64{c[0] - a[0], c[1] - a[1], c[2] - a[2]}
					area := math.Sqrt(math.Pow(u[1]*v[2]-u[2]*v[1], 2)+math.Pow(u[2]*v[0]-u[0]*v[2], 2)+math.Pow(u[0]*v[1]-u[1]*v[0], 2)) / 2
					sums[best][0] += area
					sums[best][1] += area * (a[0] + bb[0] + c[0]) / 3
					sums[best][2] += area * (a[1] + bb[1] + c[1]) / 3
				}
			}
		}
		for i, c := range theirs {
			if s := sums[i]; s[0] > 0 {
				drift[body.GetRefDes()] = math.Max(drift[body.GetRefDes()], math.Hypot(s[1]/s[0]-c.x, s[2]/s[0]-c.y))
			}
		}
	}
	return drift
}

// kicadColourTolMm is how far the centre of a colour on a placed mesh may sit from KiCad's. Upright,
// 99 in 100 of Glasgow's colours sit within 0.004 mm.
const kicadColourTolMm = 0.05

// knownColourDriftMm lists the parts whose .wrl, from KiCad 9.0.0, colours differently from the
// KiCad 10.0.6 STEP that kicad-cli exports, with the most each drifts. U30's BGA-121 draws its pin-1
// mark at 0.14 mm² in the .wrl and 0.64 mm² in the STEP, in the same corner. The IDC header's STEP
// shapes its pins differently from its .wrl, 445 mm² of gold against 565, and that gold centres
// 0.29 mm higher. Glasgow stands its
// headers upright, where height does not show from above. The transform fixture turns each onto its
// side, which carries the difference into x and y.
var knownColourDriftMm = map[string]float64{
	"glasgow U30":         0.7,
	"model-transforms J1": 0.3,
	"model-transforms J2": 0.3,
	"model-transforms J3": 0.3,
	"model-transforms J4": 0.3,
	"model-transforms J5": 0.3,
}

// TestMeshColoursSitWhereKiCadsDo holds each colour on the meshes the stack view draws to where
// kicad-cli's export puts it (agni issue 1652). An outline from above cannot see a half turn of a
// part that looks the same either way, such as an electrolytic cap, whose polarity is only its
// stripe's colour. The centre of each colour's surface moves with the stripe.
func TestMeshColoursSitWhereKiCadsDo(t *testing.T) {
	for _, c := range []struct{ name, pcb string }{
		{"glasgow", filepath.Join("..", "..", "tools", "samples", "boards", "glasgow-revC3", "glasgow.kicad_pcb")},
		{"model-transforms", filepath.Join("testdata", "model-transforms", "model-transforms.kicad_pcb")},
	} {
		kicad := readKiCadColours(t, filepath.Join("testdata", c.name+".kicad3dcolour.tsv"))
		drift := colourDrift(t, c.pcb, kicad, false)
		if len(drift) < len(kicad) {
			t.Errorf("%s: compared %d parts' colours of the oracle's %d", c.name, len(drift), len(kicad))
		}
		for ref, d := range drift {
			tol := kicadColourTolMm
			if k, ok := knownColourDriftMm[c.name+" "+ref]; ok {
				tol = k
			}
			if d > tol {
				t.Errorf("%s %s: a colour's centre sits %.3f mm from KiCad's", c.name, ref, d)
			}
		}
	}
}

// TestColourCheckCatchesAHalfTurn is the positive control for TestMeshColoursSitWhereKiCadsDo. Every
// model turned half a turn about its footprint's origin must fail Glasgow's polarized parts: the
// electrolytic cap C87, the IDC headers J2 and J3, and the ICs. A part that looks the same either
// way, colours included, such as an 0402 resistor, passes, since a half turn does not change it.
func TestColourCheckCatchesAHalfTurn(t *testing.T) {
	pcb := filepath.Join("..", "..", "tools", "samples", "boards", "glasgow-revC3", "glasgow.kicad_pcb")
	drift := colourDrift(t, pcb, readKiCadColours(t, filepath.Join("testdata", "glasgow.kicad3dcolour.tsv")), true)
	for _, ref := range []string{"C87", "J2", "J3", "U30", "U19", "U5"} {
		if drift[ref] <= kicadColourTolMm {
			t.Errorf("%s turned half a turn drifts %.3f mm, inside the tolerance", ref, drift[ref])
		}
	}
	caught := 0
	for _, d := range drift {
		if d > kicadColourTolMm {
			caught++
		}
	}
	if caught < 30 {
		t.Errorf("a half turn moves %d parts' colours, want at least 30 of Glasgow's polarized parts", caught)
	}
	t.Logf("a half turn moves %d of %d parts' colours", caught, len(drift))
}

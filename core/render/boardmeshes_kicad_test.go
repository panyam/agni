package render

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
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

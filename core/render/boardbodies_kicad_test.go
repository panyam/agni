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

// kicadModel is one row of a kicad3d oracle (hack/kicad_3d_oracle.sh): a part's 3D model as
// kicad-cli places it. ring is the model's outline seen from above, as x,y pairs in millimetres in the
// board frame, and heightMm is the model's own height.
type kicadModel struct {
	ref, side string
	heightMm  float64
	ring      []float64
}

func readKiCad3D(t *testing.T, path string) []kicadModel {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []kicadModel
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		w := strings.Split(line, "\t")
		if len(w) != 4 {
			t.Fatalf("%s: row %q has %d fields, want 4", path, line, len(w))
		}
		m := kicadModel{ref: w[0], side: w[1]}
		if m.heightMm, err = strconv.ParseFloat(w[2], 64); err != nil {
			t.Fatalf("%s: row %q: %v", path, line, err)
		}
		for _, pair := range strings.Fields(w[3]) {
			xy := strings.Split(pair, ",")
			if len(xy) != 2 {
				t.Fatalf("%s: row %q: point %q", path, line, pair)
			}
			for _, s := range xy {
				v, err := strconv.ParseFloat(s, 64)
				if err != nil {
					t.Fatalf("%s: row %q: %v", path, line, err)
				}
				m.ring = append(m.ring, v)
			}
		}
		out = append(out, m)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// outside is how far, in millimetres, the farthest point of a model's outline sits outside a
// block's counter-clockwise convex ring, given in board units. It is 0 when the outline lies inside.
func outside(ring []float64, unitsPerMm float64, m kicadModel) float64 {
	worst := 0.0
	n := len(ring) / 2
	for k := 0; k+1 < len(m.ring); k += 2 {
		x, y := m.ring[k]*unitsPerMm, m.ring[k+1]*unitsPerMm
		for i := 0; i < n; i++ {
			ax, ay := ring[2*i], ring[2*i+1]
			bx, by := ring[2*((i+1)%n)], ring[2*((i+1)%n)+1]
			// The signed distance to the edge's line, negative on the outside of a CCW ring.
			d := ((bx-ax)*(y-ay) - (by-ay)*(x-ax)) / math.Hypot(bx-ax, by-ay)
			worst = max(worst, -d/unitsPerMm)
		}
	}
	return worst
}

func glasgowBoard(t *testing.T) *geom.BoardGeometry {
	t.Helper()
	path := filepath.Join("..", "..", "tools", "samples", "boards", "glasgow-revC3", "glasgow.kicad_pcb")
	f, err := os.Open(path)
	if err != nil {
		// Deliberately fatal rather than skipped, so a missing corpus does not read as a pass.
		t.Fatalf("sample corpus missing, run `make samples`: %v", err)
	}
	defer f.Close()
	b, err := kicad.ReadBoardGeometry(f, "glasgow.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// kicadOutlineTolMm is how far a model's outline may sit outside its block. The oracle rounds each
// point to a micrometre, and no point on Glasgow sits outside at all.
const kicadOutlineTolMm = 0.005

// kicadHeightTolMm is how far a block's stated height may differ from KiCad's model.
const kicadHeightTolMm = 0.05

// TestBodiesAgreeWithKiCad3D holds the stack view's part blocks to KiCad's own 3D export of Glasgow
// revC3 (agni 1643). For every part KiCad places a model for, the block must exist, sit on the same
// side, and enclose the model's outline seen from above. A block that states a height must match the
// model's. KiCad boards state none yet, so that check waits on the KiCad half of #1643.
func TestBodiesAgreeWithKiCad3D(t *testing.T) {
	v := BoardStack(glasgowBoard(t))
	byRef := map[string]*geom.BoardPartBody{}
	for _, body := range v.GetBodies() {
		byRef[body.GetRefDes()] = body
	}
	rows := readKiCad3D(t, filepath.Join("testdata", "glasgow.kicad3d.tsv"))
	if len(rows) < 200 {
		t.Fatalf("the oracle holds %d parts, want Glasgow's 213; regenerate it with hack/kicad_3d_oracle.sh", len(rows))
	}
	compared, heights := 0, 0
	for _, m := range rows {
		body := byRef[m.ref]
		if body == nil {
			t.Errorf("%s: KiCad places a model and agni has no block", m.ref)
			continue
		}
		if body.GetSide() != m.side {
			t.Errorf("%s: block on the %s, KiCad's model on the %s", m.ref, body.GetSide(), m.side)
		}
		if o := outside(body.GetOutline(), v.GetUnitsPerMm(), m); o > kicadOutlineTolMm {
			t.Errorf("%s: KiCad's model reaches %.3f mm outside the block", m.ref, o)
		}
		if body.HeightMm != nil {
			heights++
			if d := math.Abs(body.GetHeightMm() - m.heightMm); d > kicadHeightTolMm {
				t.Errorf("%s: block %.3f mm tall, KiCad's model %.3f mm", m.ref, body.GetHeightMm(), m.heightMm)
			}
		}
		compared++
	}
	if compared != len(rows) {
		t.Errorf("compared %d of %d parts", compared, len(rows))
	}
	t.Logf("%d parts held to KiCad's models, %d of them by height", compared, heights)
}

// TestKiCad3DOracleCatchesAMisplacedPart is the positive control for TestBodiesAgreeWithKiCad3D. A
// block is a courtyard, which runs some 0.7 mm past a small part's model, so containment catches a
// part placed further off than that, or turned the wrong way. C1 is an 0603 capacitor, 1.6 by 0.8 mm,
// in a 2.96 by 1.46 mm courtyard. Moved 3 mm, or turned a quarter, its model leaves the block.
func TestKiCad3DOracleCatchesAMisplacedPart(t *testing.T) {
	v := BoardStack(glasgowBoard(t))
	var model kicadModel
	for _, m := range readKiCad3D(t, filepath.Join("testdata", "glasgow.kicad3d.tsv")) {
		if m.ref == "C1" {
			model = m
		}
	}
	var body *geom.BoardPartBody
	for _, b := range v.GetBodies() {
		if b.GetRefDes() == "C1" {
			body = b
		}
	}
	if body == nil || model.ref == "" {
		t.Fatalf("C1 is missing: block %v, model %q", body != nil, model.ref)
	}
	if o := outside(body.GetOutline(), v.GetUnitsPerMm(), model); o > kicadOutlineTolMm {
		t.Fatalf("C1 as KiCad places it reads %.3f mm outside, want inside", o)
	}
	cx, cy := 0.0, 0.0
	n := float64(len(model.ring) / 2)
	for k := 0; k < len(model.ring); k += 2 {
		cx, cy = cx+model.ring[k]/n, cy+model.ring[k+1]/n
	}
	moved, turned := model, model
	moved.ring, turned.ring = nil, nil
	for k := 0; k < len(model.ring); k += 2 {
		x, y := model.ring[k], model.ring[k+1]
		moved.ring = append(moved.ring, x+3, y)
		turned.ring = append(turned.ring, cx-(y-cy), cy+(x-cx))
	}
	for name, m := range map[string]kicadModel{"moved 3 mm": moved, "turned a quarter": turned} {
		if o := outside(body.GetOutline(), v.GetUnitsPerMm(), m); o <= kicadOutlineTolMm {
			t.Errorf("C1 %s reads %.3f mm outside its block, want more than %.3f", name, o, kicadOutlineTolMm)
		}
	}
}

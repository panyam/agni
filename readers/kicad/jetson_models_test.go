package kicad

import (
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// antmicroModels is where `make samples-oracle` puts Antmicro's glTF models, which Jetson names
// through ${ANT3DMDL} (agni-samples' remote/antmicro-models.pin).
var antmicroModels = filepath.Join("..", "..", "tools", "samples", "remote", "antmicro-models", "hardware-components")

// jetsonOverhangs are the two Jetson models that reach past their courtyards as built: the SFP28
// cage, by 5.4 mm, and the MS621FE coin cell, by 2.3 mm.
var jetsonOverhangs = map[string]bool{
	"Conn_Amphenol_SFP28_Cage_U773GC16382071.step": true,
	"MS621FE-FL11E.step":                           true,
}

// courtyardLocal returns a footprint's courtyard points in its own frame, in millimetres with Y up,
// the frame a glTF stand-in is read in.
func courtyardLocal(fp *node, back bool) [][2]float64 {
	var out [][2]float64
	add := func(n *node) {
		if n == nil {
			return
		}
		p := footprintLocal(pcbPoint(n), back)
		out = append(out, [2]float64{float64(p.X) / 1e6, float64(p.Y) / 1e6})
	}
	for _, k := range fp.Kids {
		h := k.Head()
		if !strings.HasPrefix(h, "fp_") || !strings.HasSuffix(atomOf(k.Child("layer").Arg(1)), "CrtYd") {
			continue
		}
		switch h {
		case "fp_line", "fp_rect":
			s, e := k.Child("start"), k.Child("end")
			add(s)
			add(e)
			if h == "fp_rect" && s != nil && e != nil {
				for _, c := range [][2]*node{{s, e}, {e, s}} {
					a, b := pcbPoint(c[0]), pcbPoint(c[1])
					p := footprintLocal(&geom.Point{X: a.X, Y: b.Y}, back)
					out = append(out, [2]float64{float64(p.X) / 1e6, float64(p.Y) / 1e6})
				}
			}
		case "fp_poly":
			if pts := k.Child("pts"); pts != nil {
				for _, xy := range pts.Children("xy") {
					add(xy)
				}
			}
		case "fp_arc":
			add(k.Child("start"))
			add(k.Child("mid"))
			add(k.Child("end"))
		}
	}
	return out
}

// hull2 is the counter-clockwise convex hull of 2D points, by Andrew's monotone chain.
func hull2(pts [][2]float64) [][2]float64 {
	if len(pts) < 3 {
		return nil
	}
	pts = append([][2]float64(nil), pts...)
	sort.Slice(pts, func(i, j int) bool { return pts[i][0] < pts[j][0] || pts[i][0] == pts[j][0] && pts[i][1] < pts[j][1] })
	cross := func(o, a, b [2]float64) float64 { return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0]) }
	var lo, hi [][2]float64
	for _, p := range pts {
		for len(lo) >= 2 && cross(lo[len(lo)-2], lo[len(lo)-1], p) <= 0 {
			lo = lo[:len(lo)-1]
		}
		lo = append(lo, p)
	}
	for i := len(pts) - 1; i >= 0; i-- {
		for len(hi) >= 2 && cross(hi[len(hi)-2], hi[len(hi)-1], pts[i]) <= 0 {
			hi = hi[:len(hi)-1]
		}
		hi = append(hi, pts[i])
	}
	return append(lo[:len(lo)-1], hi[:len(hi)-1]...)
}

// outsideMm is how far p sits outside a counter-clockwise convex ring, or 0 inside it.
func outsideMm(ring [][2]float64, p [2]float64) float64 {
	worst := 0.0
	for i := range ring {
		a, b := ring[i], ring[(i+1)%len(ring)]
		d := ((b[0]-a[0])*(p[1]-a[1]) - (b[1]-a[1])*(p[0]-a[0])) / math.Hypot(b[0]-a[0], b[1]-a[1])
		worst = math.Max(worst, -d)
	}
	return worst
}

// jetsonModelFit reads Jetson's board and every glTF its ${ANT3DMDL} models stand for. It returns how
// many models it placed, and the names of those that reach more than 0.1 mm past their footprint's
// courtyard. A back part's model turns half a turn about Z, as its ModelInstance does (agni 1652).
// mirror flips each model's y, the error a wrong axis change makes.
func jetsonModelFit(t *testing.T, mirror bool) (placed int, outside map[string]float64) {
	t.Helper()
	if _, err := os.Stat(antmicroModels); err != nil {
		t.Fatalf("Antmicro's models are missing, run `make samples-oracle`: %v", err)
	}
	f, err := os.Open(filepath.Join("..", "..", "tools", "samples", "boards", "jetson-agx-thor-baseboard", "jetson-agx-thor-baseboard.kicad_pcb"))
	if err != nil {
		t.Fatalf("sample corpus missing, run `make samples-oracle`: %v", err)
	}
	defer f.Close()
	root, err := parseBoard(f)
	if err != nil {
		t.Fatal(err)
	}
	open := func(raw string) ([]byte, error) {
		ref := SplitModelPath(raw)
		if ref.Anchor != AnchorVar || ref.Var != "ANT3DMDL" {
			return nil, ErrModelNotFound
		}
		b, err := os.ReadFile(filepath.Join(antmicroModels, filepath.FromSlash(ref.Rel)))
		if err != nil {
			return nil, ErrModelNotFound
		}
		return b, nil
	}
	outside = map[string]float64{}
	for _, fp := range root.Children("footprint") {
		ring := hull2(courtyardLocal(fp, atomOf(fp.Child("layer").Arg(1)) == "B.Cu"))
		if len(ring) < 3 {
			continue
		}
		for _, m := range fp.Children("model") {
			raw := atomOf(m.Arg(1))
			if SplitModelPath(raw).Anchor != AnchorVar {
				continue
			}
			var pts [][3]float64
			for _, c := range modelCandidates(raw) {
				if p, inFootprint, err := readModel(open, c, path.Ext(raw)); err == nil && inFootprint {
					pts = p
					break
				}
			}
			if pts == nil {
				continue
			}
			placed++
			worst := 0.0
			for _, q := range pts {
				x, y := q[0], q[1]
				if mirror {
					y = -y
				}
				if atomOf(fp.Child("layer").Arg(1)) == "B.Cu" {
					x, y = -x, -y
				}
				worst = math.Max(worst, outsideMm(ring, [2]float64{x, y}))
			}
			if worst > 0.1 {
				name := filepath.Base(raw)
				outside[name] = math.Max(outside[name], worst)
			}
		}
	}
	return placed, outside
}

// TestJetsonsGLTFModelsSitInTheirCourtyards holds Antmicro's glTF stand-ins to Jetson's own
// footprints (agni 1687). No tool here can render Jetson's STEP library, so a footprint's courtyard
// is the oracle. A model in the wrong units, on the wrong axis or mirrored lands outside it. Every
// model but the two known overhangs sits within 0.1 mm.
func TestJetsonsGLTFModelsSitInTheirCourtyards(t *testing.T) {
	placed, outside := jetsonModelFit(t, false)
	if placed < 930 {
		t.Fatalf("placed %d of Jetson's glTF models, want at least 930 of 938", placed)
	}
	for name, d := range outside {
		if !jetsonOverhangs[name] {
			t.Errorf("%s reaches %.3f mm past its courtyard", name, d)
		}
	}
	t.Logf("%d models placed, %d known overhangs", placed, len(outside))
}

// TestJetsonsCourtyardCheckCatchesAMirroredModel is the positive control for the test above. With
// each model's y flipped, more models leave their courtyards than the two known overhangs.
func TestJetsonsCourtyardCheckCatchesAMirroredModel(t *testing.T) {
	_, outside := jetsonModelFit(t, true)
	extra := 0
	for name := range outside {
		if !jetsonOverhangs[name] {
			extra++
		}
	}
	if extra == 0 {
		t.Error("mirrored models all fit their courtyards, so the check cannot see a wrong axis")
	}
	t.Logf("mirrored, %d more models leave their courtyards", extra)
}

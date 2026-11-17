package kicad

import (
	"math"
	"strings"
	"testing"
)

func TestSplitModelPath(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want ModelRef
	}{
		{"${KICAD6_3DMODEL_DIR}/Resistor_SMD.3dshapes/R_0402_1005Metric.step", ModelRef{Anchor: AnchorLibrary, Rel: "Resistor_SMD.3dshapes/R_0402_1005Metric.step"}},
		{"${KICAD10_3DMODEL_DIR}/a.3dshapes/b.step", ModelRef{Anchor: AnchorLibrary, Rel: "a.3dshapes/b.step"}},
		{"$(KISYS3DMOD)/a.3dshapes/b.wrl", ModelRef{Anchor: AnchorLibrary, Rel: "a.3dshapes/b.wrl"}},
		{"${KIPRJMOD}/3d/x.step", ModelRef{Anchor: AnchorBoard, Rel: "3d/x.step"}},
		{"$(KIPRJMOD)/../../packages3D/USB.step", ModelRef{Anchor: AnchorBoard, Rel: "../../packages3D/USB.step"}},
		{"../../packages3D/SOT-143.step", ModelRef{Anchor: AnchorBoard, Rel: "../../packages3D/SOT-143.step"}},
		{`${KIPRJMOD}\libs\x.step`, ModelRef{Anchor: AnchorBoard, Rel: "libs/x.step"}},
		{"/home/someone/models/x.step", ModelRef{Anchor: AnchorAbsolute, Rel: "/home/someone/models/x.step"}},
		{`C:\models\x.step`, ModelRef{Anchor: AnchorAbsolute, Rel: "C:/models/x.step"}},
		{"${ANT3DMDL}/R_0402_1005Metric.step", ModelRef{Anchor: AnchorVar, Var: "ANT3DMDL", Rel: "R_0402_1005Metric.step"}},
		{"/${ANT3DMDL}/R_0402_1005Metric.step", ModelRef{Anchor: AnchorVar, Var: "ANT3DMDL", Rel: "R_0402_1005Metric.step"}},
		{"$(MY_MODELS)/sub/x.step", ModelRef{Anchor: AnchorVar, Var: "MY_MODELS", Rel: "sub/x.step"}},
	} {
		if got := SplitModelPath(c.raw); got != c.want {
			t.Errorf("SplitModelPath(%q) = %+v, want %+v", c.raw, got, c.want)
		}
	}
}

// boxModel is a 1 x 2 x 3 mm block standing on the origin, as a STEP file.
const boxModel = `ISO-10303-21;
HEADER;
ENDSEC;
DATA;
#1=CARTESIAN_POINT('',(0.,0.,0.));
#2=CARTESIAN_POINT('',(1.,2.,3.));
#3=VERTEX_POINT('',#1);
#4=VERTEX_POINT('',#2);
ENDSEC;
END-ISO-10303-21;
`

const tallModel = `ISO-10303-21;
HEADER;
ENDSEC;
DATA;
#1=CARTESIAN_POINT('',(0.,0.,0.));
#2=CARTESIAN_POINT('',(1.,1.,9.));
#3=VERTEX_POINT('',#1);
#4=VERTEX_POINT('',#2);
ENDSEC;
END-ISO-10303-21;
`

func readModelBoard(t *testing.T, footprints string, open ModelOpener) map[string]float64 {
	t.Helper()
	src := `(kicad_pcb (version 20221018) (generator pcbnew)
  (layers (0 "F.Cu" signal) (31 "B.Cu" signal))
` + footprints + "\n)\n"
	g, err := ReadBoardGeometryWithModels(strings.NewReader(src), "t.kicad_pcb", open)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, p := range g.GetPlacements() {
		if p.Height != nil {
			out[p.GetRefDes()] = float64(p.GetHeight()*g.GetUnitNm()) / 1e6
		}
	}
	return out
}

func modelFiles(files map[string]string) ModelOpener {
	return func(path string) ([]byte, error) {
		if s, ok := files[path]; ok {
			return []byte(s), nil
		}
		return nil, ErrModelNotFound
	}
}

func fp(ref, models string) string {
	return `  (footprint "x:y" (layer "F.Cu") (at 0 0)
    (property "Reference" "` + ref + `")
    (pad "1" smd rect (at 0 0) (size 1 1) (layers "F.Cu"))
` + models + "\n  )\n"
}

func TestModelHeightsFollowTheFootprintsTransform(t *testing.T) {
	open := modelFiles(map[string]string{"box.step": boxModel, "tall.step": tallModel})
	got := readModelBoard(t, fp("A", `(model "box.step")`)+
		fp("B", `(model "box.step" (scale (xyz 1 1 2)))`)+
		fp("C", `(model "box.step" (offset (xyz 0 0 1.5)))`)+
		// KiCad 5 wrote the offset as (at) in inches.
		fp("D", `(model "box.step" (at (xyz 0 0 0.1)))`)+
		// A turn of -90 about X puts the box's +y side up, 2 mm.
		fp("E", `(model "box.step" (rotate (xyz -90 0 0)))`)+
		// Hidden models do not count, whether KiCad 8 or KiCad 7 wrote the flag.
		fp("F", `(model "tall.step" (hide yes)) (model "box.step")`)+
		fp("G", `(model "tall.step" hide) (model "box.step")`)+
		// Two drawn models stand as tall as the taller.
		fp("H", `(model "box.step") (model "tall.step")`)+
		fp("I", `(model "missing.step")`), open)
	want := map[string]float64{"A": 3, "B": 6, "C": 4.5, "D": 3 + 2.54, "E": 2, "F": 3, "G": 3, "H": 9}
	for ref, w := range want {
		if h, ok := got[ref]; !ok || h < w-1e-6 || h > w+1e-6 {
			t.Errorf("%s: height %v (set %v), want %v", ref, h, ok, w)
		}
	}
	if _, ok := got["I"]; ok {
		t.Errorf("I: a missing model gave height %v", got["I"])
	}
}

func TestUnreadModelsNameThePathAndReason(t *testing.T) {
	src := `(kicad_pcb (version 20221018) (generator pcbnew)
  (layers (0 "F.Cu" signal))
` + fp("A", `(model "missing.step")`) + fp("B", `(model "missing.step")`) + fp("C", `(model "bad.step")`) + "\n)\n"
	g, err := ReadBoardGeometryWithModels(strings.NewReader(src), "t.kicad_pcb", modelFiles(map[string]string{"bad.step": "solid x"}))
	if err != nil {
		t.Fatal(err)
	}
	u := g.GetUnreadModels()
	if len(u) != 2 || u[0].GetPath() != "bad.step" || u[1].GetPath() != "missing.step" || u[1].GetParts() != 2 || u[1].GetReason() != "not found" {
		t.Errorf("unread models = %v", u)
	}
}

func TestANilOpenerReadsNoModel(t *testing.T) {
	if got := readModelBoard(t, fp("A", `(model "box.step")`), nil); len(got) != 0 {
		t.Errorf("heights %v with no opener", got)
	}
}

// gltfBlock is a glTF block 2 mm wide along x, 1 mm deep and 0.9 tall along glTF's Y, standing on
// the origin and offset 0.5 mm toward glTF's -Z, with its buffer in block.bin. In the footprint's
// frame, glTF's -Z is +y, so the block sits from y = 0 to y = 1.
const gltfBlock = `{"asset":{"version":"2.0"},"scene":0,"scenes":[{"nodes":[0]}],
"nodes":[{"mesh":0,"translation":[0,0,-0.5]}],
"meshes":[{"primitives":[{"attributes":{"POSITION":0},"indices":1}]}],
"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
             {"bufferView":1,"componentType":5123,"count":3,"type":"SCALAR"}],
"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":6}],
"buffers":[{"uri":"block.bin","byteLength":44}]}`

// blockBin holds three positions, (-1, 0, 0.5), (1, 0.9, 0.5) and (1, 0, -0.5), then indices 0 1 2.
func blockBin() string {
	var b []byte
	put := func(f float32) {
		u := math.Float32bits(f)
		b = append(b, byte(u), byte(u>>8), byte(u>>16), byte(u>>24))
	}
	for _, v := range []float32{-1, 0, 0.5, 1, 0.9, 0.5, 1, 0, -0.5} {
		put(v)
	}
	b = append(b, 0, 0, 1, 0, 2, 0, 0, 0)
	return string(b)
}

// TestAGLTFStandsInForANamedSTEP covers Antmicro's library (agni 1687). Jetson names X.step, and
// Antmicro ships X.gltf. The glTF is read in millimetres, in the footprint's own frame, so the
// footprint's model offset and turn do not move it. G names a glTF itself, which is read in metres:
// its block is scaled to 0.0009 m, which reads as 0.9 mm.
func TestAGLTFStandsInForANamedSTEP(t *testing.T) {
	open := modelFiles(map[string]string{
		"${ANT3DMDL}/block.gltf": gltfBlock,
		"${ANT3DMDL}/block.bin":  blockBin(),
		// The reader asks for a stand-in under the path as the footprint spells it. A Loader resolves
		// both spellings the same.
		"/${ANT3DMDL}/block.gltf": gltfBlock,
		"/${ANT3DMDL}/block.bin":  blockBin(),
		"${KIPRJMOD}/m.gltf":      strings.Replace(gltfBlock, `"translation":[0,0,-0.5]`, `"scale":[0.001,0.001,0.001]`, 1),
		"${KIPRJMOD}/block.bin":   blockBin(),
	})
	got := readModelBoard(t, fp("A", `(model "${ANT3DMDL}/block.step")`)+
		fp("B", `(model "/${ANT3DMDL}/block.step" (offset (xyz 0 0 4)) (rotate (xyz -90 0 0)))`)+
		fp("G", `(model "${KIPRJMOD}/m.gltf")`), open)
	for ref, want := range map[string]float64{"A": 0.9, "B": 0.9, "G": 0.9} {
		if h, ok := got[ref]; !ok || math.Abs(h-want) > 1e-5 {
			t.Errorf("%s: height %v (set %v), want %v", ref, h, ok, want)
		}
	}
}

// TestAGLTFLiesInTheFootprintFrame holds the axis change, glTF's (x, y, z) to the footprint's
// (x, -z, y), with the block offset toward glTF's -Z.
func TestAGLTFLiesInTheFootprintFrame(t *testing.T) {
	open := modelFiles(map[string]string{"L/block.gltf": gltfBlock, "L/block.bin": blockBin()})
	pts, inFootprint, err := readModel(open, "L/block.gltf", ".step")
	if err != nil {
		t.Fatal(err)
	}
	if !inFootprint {
		t.Error("a glTF read as needing the footprint's model transform")
	}
	want := [][3]float64{{-1, 0, 0}, {1, 0, 0.9}, {1, 1, 0}}
	for i, w := range want {
		for k := range w {
			if math.Abs(pts[i][k]-w[k]) > 1e-6 {
				t.Errorf("point %d = %v, want %v", i, pts[i], w)
				break
			}
		}
	}
}

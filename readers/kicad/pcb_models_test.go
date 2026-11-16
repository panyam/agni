package kicad

import (
	"errors"
	"strings"
	"testing"
)

func TestSplitModelPath(t *testing.T) {
	for _, c := range []struct {
		raw    string
		anchor ModelAnchor
		rel    string
	}{
		{"${KICAD6_3DMODEL_DIR}/Resistor_SMD.3dshapes/R_0402_1005Metric.step", AnchorLibrary, "Resistor_SMD.3dshapes/R_0402_1005Metric.step"},
		{"${KICAD10_3DMODEL_DIR}/a.3dshapes/b.step", AnchorLibrary, "a.3dshapes/b.step"},
		{"$(KISYS3DMOD)/a.3dshapes/b.wrl", AnchorLibrary, "a.3dshapes/b.wrl"},
		{"${KIPRJMOD}/3d/x.step", AnchorBoard, "3d/x.step"},
		{"$(KIPRJMOD)/../../packages3D/USB.step", AnchorBoard, "../../packages3D/USB.step"},
		{"../../packages3D/SOT-143.step", AnchorBoard, "../../packages3D/SOT-143.step"},
		{`${KIPRJMOD}\libs\x.step`, AnchorBoard, "libs/x.step"},
		{"/home/someone/models/x.step", AnchorAbsolute, "/home/someone/models/x.step"},
		{`C:\models\x.step`, AnchorAbsolute, "C:/models/x.step"},
		{"${MY_MODELS}/x.step", AnchorUnknown, "${MY_MODELS}/x.step"},
	} {
		a, rel := SplitModelPath(c.raw)
		if a != c.anchor || rel != c.rel {
			t.Errorf("SplitModelPath(%q) = %d, %q; want %d, %q", c.raw, a, rel, c.anchor, c.rel)
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
		return nil, errors.New("not found")
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
